package consumer

import (
	"testing"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/queue"
	"github.com/hibiken/asynq"
)

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
