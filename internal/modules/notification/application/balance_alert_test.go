package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/queue"
)

type balanceStateStub struct {
	values map[string]string
	ttl    time.Duration
	err    error
}

func (*balanceStateStub) Ready() bool { return true }
func (s *balanceStateStub) Get(_ context.Context, k string) (string, error) {
	return s.values[k], s.err
}
func (s *balanceStateStub) Claim(_ context.Context, k, v string, ttl time.Duration) (bool, error) {
	if s.err != nil {
		return false, s.err
	}
	if _, ok := s.values[k]; ok {
		return false, nil
	}
	s.values[k] = v
	s.ttl = ttl
	return true, nil
}
func (s *balanceStateStub) Put(_ context.Context, k, v string, ttl time.Duration) error {
	s.values[k] = v
	s.ttl = ttl
	return s.err
}

type balanceQueueStub struct {
	payloads []queue.NotificationDispatchPayload
	err      error
}

func (q *balanceQueueStub) EnqueueNotificationDispatch(p queue.NotificationDispatchPayload, _ int) error {
	q.payloads = append(q.payloads, p)
	return q.err
}

func (r *notificationLogRepositoryStub) HasSuccessfulDelivery(event, biz string, id uint, channel, recipient string) (bool, error) {
	for _, log := range r.items {
		if log.EventType == event && log.BizType == biz && log.BizID == id && log.Channel == channel && log.Recipient == recipient && log.Status == "success" && !log.IsTest {
			return true, nil
		}
	}
	return false, nil
}

func TestBalanceAlertUsesActualSMTPResultAndDeduplicates(t *testing.T) {
	for _, tc := range []struct {
		name       string
		recipients []string
		status     string
	}{
		{"success", []string{"success@example.test"}, "sent"},
		{"failure", []string{"failure@example.test"}, "failed"},
		{"partial", []string{"success@example.test", "failure@example.test"}, "partial"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, logs := setupLogService(t)
			settings := svc.settingService.(notificationSettingsStub)
			settings.notification.Scenes.ExceptionAlert = false // Safety notices use email channel configuration.
			settings.notification.Channels.Email.Recipients = tc.recipients
			svc.settingService = settings
			state := &balanceStateStub{values: map[string]string{}}
			svc.balanceState = state
			q := &balanceQueueStub{}
			svc.queueClient = q
			ctx := context.Background()
			if status := svc.NotifyBalanceShortage(ctx, 7, "120", "119.99", "CNY"); status != "pending" {
				t.Fatalf("queued mail claimed %s", status)
			}
			for i := 0; i < 5; i++ {
				svc.NotifyBalanceShortage(ctx, 7, "120", "119.99", "CNY")
			}
			if len(q.payloads) != 1 {
				t.Fatal("repeated click enqueued duplicate mail")
			}
			err := svc.Dispatch(ctx, q.payloads[0])
			if (tc.status == "sent" && err != nil) || (tc.status != "sent" && !errors.Is(err, contract.ErrSendFailed)) {
				t.Fatalf("failed recipients must be retryable: %v", err)
			}
			if status := svc.BalanceAlertStatus(ctx, 7); status != tc.status {
				t.Fatalf("mail status %s, want %s", status, tc.status)
			}
			items, _, err := logs.repo.ListAdmin(structLogFilter())
			if err != nil || len(items) != len(tc.recipients) {
				t.Fatal("missing actual send logs")
			}
			if tc.status == "sent" {
				if state.ttl != 30*time.Minute {
					t.Fatal("missing success cooldown")
				}
				if err := svc.Dispatch(ctx, q.payloads[0]); err != nil {
					t.Fatal(err)
				}
				items, _, _ = logs.repo.ListAdmin(structLogFilter())
				if len(items) != 1 {
					t.Fatal("redelivered successful incident")
				}
			}
		})
	}
}

func TestBalanceAlertUnavailableNeverClaimsSent(t *testing.T) {
	svc, _ := setupLogService(t)
	ctx := context.Background()
	if svc.NotifyBalanceShortage(ctx, 7, "20", "0", "CNY") != "not_sent" {
		t.Fatal("no queue claimed mail sent")
	}
	svc.balanceState = &balanceStateStub{values: map[string]string{}, err: errors.New("redis unavailable")}
	q := &balanceQueueStub{}
	svc.queueClient = q
	if svc.NotifyBalanceShortage(ctx, 7, "20", "0", "CNY") != "not_sent" || len(q.payloads) != 0 {
		t.Fatal("cache failure sent mail")
	}
	svc.balanceState = &balanceStateStub{values: map[string]string{}}
	q.err = errors.New("queue unavailable")
	if svc.NotifyBalanceShortage(ctx, 7, "20", "0", "CNY") != "failed" {
		t.Fatal("queue failure claimed sent")
	}
	settings := svc.settingService.(notificationSettingsStub)
	settings.notification.Channels.Email.Enabled = false
	svc.settingService = settings
	if svc.NotifyBalanceShortage(ctx, 8, "20", "0", "CNY") != "not_sent" {
		t.Fatal("disabled email claimed sent")
	}
}

func structLogFilter() contract.LogListFilter {
	return contract.LogListFilter{EventType: constants.NotificationEventExceptionAlert}
}
