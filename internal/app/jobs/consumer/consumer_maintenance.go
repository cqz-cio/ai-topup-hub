package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	notificationcontract "github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/queue"

	"github.com/hibiken/asynq"
)

// handleNotificationDispatch 处理通知中心异步分发任务。
func (c *Consumer) handleNotificationDispatch(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil {
		logger.Debugw("worker_notification_dispatch_skip_nil", "consumer_nil", c == nil, "task_nil", task == nil)
		return nil
	}
	if c.NotificationService == nil {
		logger.Warnw("worker_notification_dispatch_skip_service_nil")
		return nil
	}
	var payload queue.NotificationDispatchPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_notification_dispatch_unmarshal_failed", "error", err)
		return err
	}
	if strings.TrimSpace(payload.EventType) == "" {
		logger.Debugw("worker_notification_dispatch_skip_empty_event")
		return nil
	}
	// Waiting for a purchase must not consume SMTP retry attempts or acquire a
	// notification delivery lock. Requeue only the read-only notification task.
	if payload.EventType == constants.NotificationEventOrderPaidSuccess && payload.BizType == constants.NotificationBizTypeOrder {
		if payload.Data == nil {
			payload.Data = map[string]interface{}{}
		}
		started, _ := payload.Data["sale_wait_started_at"].(string)
		waitStarted, parseErr := time.Parse(time.RFC3339Nano, started)
		if parseErr != nil {
			waitStarted = time.Now()
			payload.Data["sale_wait_started_at"] = waitStarted.Format(time.RFC3339Nano)
		}
		if time.Since(waitStarted) >= 10*time.Minute {
			payload.Data["sale_wait_timeout"] = true
		}
	}
	if err := c.NotificationService.Dispatch(ctx, payload); err != nil {
		if errors.Is(err, notificationcontract.ErrProcurementPending) && c.QueueClient != nil {
			return c.QueueClient.EnqueueNotificationDispatch(payload, asynq.ProcessIn(15*time.Second), asynq.MaxRetry(5))
		}
		logger.Warnw("worker_notification_dispatch_failed",
			"event_type", payload.EventType,
			"biz_type", payload.BizType,
			"biz_id", payload.BizID,
			"error", err,
		)
		return err
	}
	return nil
}

// handleAffiliateConfirmCommissions 处理分销佣金确认任务。
func (c *Consumer) handleAffiliateConfirmCommissions(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.AffiliateService == nil {
		logger.Debugw("worker_affiliate_confirm_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	if err := c.AffiliateService.ConfirmDueCommissions(time.Now()); err != nil {
		logger.Warnw("worker_affiliate_confirm_due_failed", "error", err)
		return err
	}
	return nil
}

func (c *Consumer) handleResellerConfirmLedger(_ context.Context, _ *asynq.Task) error {
	if c == nil || c.ResellerAccountingLedger == nil {
		logger.Debugw("worker_reseller_confirm_ledger_skip_nil", "consumer_nil", c == nil)
		return nil
	}
	affected, err := c.ResellerAccountingLedger.ConfirmDueLedgerEntries(time.Now())
	if err != nil {
		logger.Warnw("worker_reseller_confirm_ledger_failed", "error", err)
		return err
	}
	logger.Debugw("worker_reseller_confirm_ledger_ok", "affected", affected)
	return nil
}

// handleReconciliationRun 处理对账任务执行。
func (c *Consumer) handleReconciliationRun(ctx context.Context, task *asynq.Task) error {
	if c == nil || task == nil || c.ReconciliationService == nil {
		logger.Debugw("worker_reconciliation_run_skip_nil")
		return nil
	}
	var payload queue.ReconciliationRunPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		logger.Warnw("worker_reconciliation_run_unmarshal_failed", "error", err)
		return err
	}
	if payload.JobID == 0 {
		return nil
	}
	if err := c.ReconciliationService.Execute(ctx, payload.JobID); err != nil {
		logger.Warnw("worker_reconciliation_run_failed",
			"job_id", payload.JobID,
			"error", err,
		)
		return err
	}
	return nil
}
