package queue

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/hibiken/asynq"
)

func TestNotificationCapacityAndRetryPolicy(t *testing.T) {
	cfg := &config.QueueConfig{Concurrency: 2, Queues: map[string]int{DefaultQueue: 1, NotificationQueue: 7}}
	_, business := BuildServerConfig(cfg)
	_, mail := BuildNotificationServerConfig(cfg)
	if business.Queues[NotificationQueue] != 0 || business.Concurrency != 2 || mail.Concurrency != 4 || len(mail.Queues) != 1 || mail.Queues[NotificationQueue] != 1 {
		t.Fatal("notifications share business worker capacity")
	}
	if cfg.Queues[NotificationQueue] != 7 {
		t.Fatal("mutated caller configuration")
	}
	for _, kind := range []string{TaskNotificationDispatch, TaskOrderStatusEmail} {
		for n, want := range []time.Duration{3 * time.Second, 5 * time.Second, 10 * time.Second, 20 * time.Second, 30 * time.Second, 30 * time.Second} {
			if got := mail.RetryDelayFunc(n, errors.New("temporary"), asynq.NewTask(kind, nil)); got != want {
				t.Fatalf("retry %d: %v", n, got)
			}
		}
	}
	if mail.DelayedTaskCheckInterval > time.Second || mail.TaskCheckInterval > time.Second {
		t.Fatal("mail retry promotion is too slow")
	}
}

// Run only against an explicitly launched disposable local Redis, never the
// configured application queue. This proves routing and reserved capacity.
func TestNotificationDeliveryWhileBusinessWorkerIsBlocked(t *testing.T) {
	port, err := strconv.Atoi(os.Getenv("MAIL_TEST_REDIS_PORT"))
	if err != nil {
		t.Skip("disposable Redis not provided")
	}
	cfg := &config.QueueConfig{Enabled: true, Host: "127.0.0.1", Port: port, Concurrency: 1}
	opt, businessCfg := BuildServerConfig(cfg)
	_, mailCfg := BuildNotificationServerConfig(cfg)
	businessCfg.ShutdownTimeout = time.Second
	mailCfg.ShutdownTimeout = time.Second
	business := asynq.NewServer(opt, businessCfg)
	mail := asynq.NewServer(opt, mailCfg)
	gate := make(chan struct{})
	started := make(chan struct{})
	delivered := make(chan time.Time, 2)
	bm := asynq.NewServeMux()
	bm.HandleFunc("test:block", func(ctx context.Context, _ *asynq.Task) error {
		close(started)
		select {
		case <-gate:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	})
	var attempts atomic.Int32
	mm := asynq.NewServeMux()
	mm.HandleFunc(TaskNotificationDispatch, func(_ context.Context, task *asynq.Task) error {
		var payload NotificationDispatchPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}
		if payload.EnqueuedAt.IsZero() {
			return errors.New("enqueue timestamp missing")
		}
		if attempts.Add(1) == 1 {
			return errors.New("temporary SMTP failure")
		}
		delivered <- payload.EnqueuedAt
		return nil
	})
	mm.HandleFunc(TaskOrderStatusEmail, func(_ context.Context, task *asynq.Task) error {
		var payload OrderStatusEmailPayload
		if err := json.Unmarshal(task.Payload(), &payload); err != nil {
			return err
		}
		delivered <- payload.EnqueuedAt
		return nil
	})
	if err := business.Start(bm); err != nil {
		t.Fatal(err)
	}
	defer business.Shutdown()
	if err := mail.Start(mm); err != nil {
		t.Fatal(err)
	}
	defer mail.Shutdown()
	defer close(gate)
	raw := asynq.NewClient(opt)
	defer raw.Close()
	if _, err := raw.Enqueue(asynq.NewTask("test:block", nil)); err != nil {
		t.Fatal(err)
	}
	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("business task did not start")
	}
	client, _ := NewClient(cfg)
	defer client.Close()
	if err := client.EnqueueNotificationDispatch(NotificationDispatchPayload{EventType: "test"}); err != nil {
		t.Fatal(err)
	}
	if err := client.EnqueueOrderStatusEmail(OrderStatusEmailPayload{OrderID: 1}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		select {
		case queued := <-delivered:
			if queued.IsZero() || time.Since(queued) > 10*time.Second {
				t.Fatal("mail missed latency bound")
			}
		case <-time.After(10 * time.Second):
			t.Fatal("mail blocked behind business work or retry")
		}
	}
	if attempts.Load() != 2 {
		t.Fatalf("unexpected retry count: %d", attempts.Load())
	}
}
