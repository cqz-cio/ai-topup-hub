package jobs

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/app/container"
	jobconsumer "github.com/dujiao-next/internal/app/jobs/consumer"
	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	pa "github.com/dujiao-next/internal/modules/procurement/application"
	pc "github.com/dujiao-next/internal/modules/procurement/contract"
	pd "github.com/dujiao-next/internal/modules/procurement/domain"
	ps "github.com/dujiao-next/internal/modules/procurement/infrastructure/gormstore"
	"github.com/dujiao-next/internal/queue"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type scheduledRecoveryOrders struct{}

func (scheduledRecoveryOrders) ReadBalanceRecoveryOrder(context.Context, uint) (*pc.BalanceRecoveryOrder, error) {
	return &pc.BalanceRecoveryOrder{OrderNo: "SCHEDULE-TEST", Recipient: "test@example.invalid", Products: "5x 测试商品", Status: constants.OrderStatusPendingPayment}, nil
}

type scheduledRecoveryBalance struct {
	pc.UpstreamConnection
	reads atomic.Int32
}

func (b *scheduledRecoveryBalance) ReadBalance(context.Context) (string, string, error) {
	b.reads.Add(1)
	return "595", "CNY", nil
}
func (b *scheduledRecoveryBalance) Open(uint) (pc.UpstreamConnection, error) { return b, nil }

type scheduledRecoveryMail chan struct{}

func (m scheduledRecoveryMail) SendBalanceRecoveryEmail(context.Context, *pd.BalanceWait) error {
	m <- struct{}{}
	return nil
}

// The port must belong to a disposable Redis started by the test runner, never
// the daily or production instance. This exercises the real 60-second timer.
func TestBalanceRecoveryScheduledWorker(t *testing.T) {
	raw := os.Getenv("BALANCE_RECOVERY_TEST_REDIS_PORT")
	if raw == "" {
		t.Skip("requires a disposable Redis via BALANCE_RECOVERY_TEST_REDIS_PORT")
	}
	port, err := strconv.Atoi(raw)
	if err != nil || port < 1024 || port == 6379 || port == 6390 {
		t.Fatal("refusing shared Redis port")
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "recovery.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sql, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	defer sql.Close()
	sql.SetMaxOpenConns(1)
	if err := db.AutoMigrate(&pd.BalanceWait{}); err != nil {
		t.Fatal(err)
	}
	cfg := &config.QueueConfig{Enabled: true, Host: "127.0.0.1", Port: port, Concurrency: 2}
	client, err := queue.NewClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	balance := &scheduledRecoveryBalance{}
	mail := make(scheduledRecoveryMail, 4)
	recovery := pa.NewBalanceRecovery(ps.New(db), scheduledRecoveryOrders{}, balance, client, mail)
	ctx := context.Background()
	if err := recovery.Record(ctx, 1, []pd.FundingRequirement{{ConnectionID: 1, Amount: "595", Currency: "CNY"}}); err != nil {
		t.Fatal(err)
	}
	service, err := NewStorefrontService(cfg, jobconsumer.New(&container.Container{BalanceRecoveryService: recovery}))
	if err != nil {
		t.Fatal(err)
	}
	if err := service.notificationServer.Start(service.notificationMux); err != nil {
		t.Fatal(err)
	}
	defer service.notificationServer.Shutdown()
	if err := service.server.Start(service.mux); err != nil {
		t.Fatal(err)
	}
	defer service.server.Shutdown()
	started := time.Now()
	if err := service.scheduler.Start(); err != nil {
		t.Fatal(err)
	}
	defer service.scheduler.Shutdown()
	select {
	case <-mail:
		elapsed := time.Since(started)
		if elapsed < 55*time.Second || elapsed > 80*time.Second {
			t.Fatalf("unexpected polling interval: %s", elapsed)
		}
		t.Logf("real scheduled recovery delivered after %s; supplier reads=%d", elapsed.Round(time.Millisecond), balance.reads.Load())
	case <-time.After(80 * time.Second):
		t.Fatal("scheduled customer mail was not consumed")
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		var wait pd.BalanceWait
		if err := db.First(&wait).Error; err != nil {
			t.Fatal(err)
		}
		if wait.Status == pd.BalanceWaitSent {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("worker did not persist successful completion")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if balance.reads.Load() != 1 {
		t.Fatal("duplicate supplier polling")
	}
}
