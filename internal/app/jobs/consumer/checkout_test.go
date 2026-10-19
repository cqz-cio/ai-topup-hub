package consumer

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/constants"
	notificationapp "github.com/dujiao-next/internal/modules/notification/application"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/queue"
	"github.com/hibiken/asynq"
)

type latencySettings struct{}

func (latencySettings) GetNotificationCenterSetting() (settingsmessaging.NotificationCenterSetting, error) {
	s := settingsmessaging.NotificationCenterDefaultSetting()
	s.Scenes.OrderPaidSuccess = true
	s.Channels.Email = settingsmessaging.NotificationChannelSetting{Enabled: true, Recipients: []string{"owner@example.invalid"}}
	return s, nil
}
func (latencySettings) GetDashboardSetting() (settingsstorefront.DashboardSetting, error) {
	return settingsstorefront.DashboardSetting{}, nil
}

type latencySender struct{ count int }

func (s *latencySender) SendCustomEmail(string, string, string) error { s.count++; return nil }

type latencySummary struct{ allowed bool }

func (s *latencySummary) Build(_ context.Context, _ uint, _ string, allow bool) (map[string]interface{}, error) {
	s.allowed = allow
	if !allow {
		return nil, notificationcontract.ErrProcurementPending
	}
	return map[string]interface{}{"upstream_summary": "采购尚未完成，请关注订单"}, nil
}

func TestNotificationWorkerHasNoPurchaseCapabilities(t *testing.T) {
	mux := asynq.NewServeMux()
	New(&container.Container{}).RegisterNotifications(mux)
	for _, kind := range []string{queue.TaskProcurementSubmit, queue.TaskProcurementRecover, queue.TaskProcurementPollStatus, queue.TaskOrderAutoFulfill, queue.TaskReconciliationRun, queue.TaskUpstreamSyncStock} {
		if _, pattern := mux.Handler(asynq.NewTask(kind, nil)); pattern != "" {
			t.Fatalf("mail worker registered %s", kind)
		}
	}
	for _, kind := range []string{queue.TaskNotificationDispatch, queue.TaskOrderStatusEmail} {
		if _, pattern := mux.Handler(asynq.NewTask(kind, nil)); pattern != kind {
			t.Fatalf("missing %s", kind)
		}
	}
}

func TestSaleNotificationGraceIncludesQueueAgeAndLegacyTasks(t *testing.T) {
	for _, tc := range []struct {
		name     string
		age      time.Duration
		legacy   bool
		wantSend bool
	}{
		{"fresh", 0, false, false}, {"past_five_seconds", 6 * time.Second, false, true},
		{"queued_for_minutes", 4 * time.Minute, false, true}, {"legacy_delayed", time.Minute, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sender := &latencySender{}
			summary := &latencySummary{}
			svc := notificationapp.NewService(latencySettings{}, sender, nil, nil, nil, nil, nil)
			svc.SetSaleSummaryReader(summary)
			consumer := New(&container.Container{NotificationService: svc})
			payload := queue.NotificationDispatchPayload{EventType: constants.NotificationEventOrderPaidSuccess, BizType: constants.NotificationBizTypeOrder, BizID: 1, EnqueuedAt: time.Now().Add(-tc.age), Data: map[string]interface{}{}}
			if tc.legacy {
				payload.Data["sale_wait_started_at"] = payload.EnqueuedAt.Format(time.RFC3339Nano)
				payload.EnqueuedAt = time.Time{}
			}
			task, err := queue.NewNotificationDispatchTask(payload)
			if err != nil {
				t.Fatal(err)
			}
			err = consumer.handleNotificationDispatch(context.Background(), task)
			if tc.wantSend {
				if err != nil || sender.count != 1 || !summary.allowed {
					t.Fatalf("notification still delayed: %v, %d", err, sender.count)
				}
			} else if !errors.Is(err, notificationcontract.ErrProcurementPending) || sender.count != 0 {
				t.Fatalf("brief grace broken: %v", err)
			}
		})
	}
}

func TestCheckoutWorkerCannotConsumeProcurement(t *testing.T) {
	mux := asynq.NewServeMux()
	New(&container.Container{}).RegisterCheckout(mux)
	for _, taskType := range []string{queue.TaskOrderTimeoutCancel, queue.TaskOrderAutoFulfill, queue.TaskOrderStatusEmail} {
		_, pattern := mux.Handler(asynq.NewTask(taskType, nil))
		if pattern != taskType {
			t.Fatalf("checkout task %q is not registered", taskType)
		}
	}
	for _, taskType := range []string{queue.TaskProcurementSubmit, queue.TaskProcurementPollStatus, queue.TaskProcurementSyncAccepted, queue.TaskProcurementRecover, queue.TaskUpstreamSyncStock, queue.TaskReconciliationRun} {
		_, pattern := mux.Handler(asynq.NewTask(taskType, nil))
		if pattern != "" {
			t.Fatalf("checkout worker can consume forbidden task %q", taskType)
		}
	}
}

func TestStorefrontWorkerConsumesPaidOrderProcurementWithoutStockTasks(t *testing.T) {
	mux := asynq.NewServeMux()
	New(&container.Container{}).RegisterStorefront(mux)
	for _, taskType := range []string{queue.TaskOrderTimeoutCancel, queue.TaskProcurementSubmit, queue.TaskProcurementPollStatus, queue.TaskProcurementRecover, queue.TaskDownstreamCallback, queue.TaskResellerConfirmLedger, queue.TaskAffiliateConfirmCommissions, queue.TaskReconciliationRun} {
		_, pattern := mux.Handler(asynq.NewTask(taskType, nil))
		if pattern != taskType {
			t.Fatalf("storefront task %q is not registered", taskType)
		}
	}
	for _, taskType := range []string{queue.TaskUpstreamSyncStock, queue.TaskTelegramBroadcast} {
		_, pattern := mux.Handler(asynq.NewTask(taskType, nil))
		if pattern != "" {
			t.Fatalf("storefront consumed unrelated task %q", taskType)
		}
	}
}
