package application

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"

	"github.com/shopspring/decimal"
)

// SubmitToUpstream Worker 调用：向上游站点提交采购单
func (s *Service) SubmitToUpstream(procurementOrderID uint) error {
	procOrder, err := s.procRepo.GetByID(procurementOrderID)
	if err != nil {
		return fmt.Errorf("load procurement order: %w", err)
	}
	if procOrder == nil {
		return procurementcontract.ErrNotFound
	}

	// 校验状态
	if procOrder.Status == "submitting" || procOrder.Status == "manual_review" {
		return nil // A purchase may already have consumed the supplier balance.
	}
	if procOrder.Status == "accepted" {
		return s.PollUpstreamStatus(procOrder.ID)
	}
	if procOrder.Status == "fulfilled" || procOrder.Status == "rejected" || procOrder.Status == "canceled" || procOrder.Status == "refunded" {
		return nil
	}
	if procOrder.Status != "pending" && procOrder.Status != "failed" {
		return procurementcontract.ErrStatusInvalid
	}
	if allowed, err := s.procurementWithinPurchaseWindow(procOrder); err != nil || !allowed {
		return err
	}
	if procOrder.NextRetryAt != nil && time.Now().Before(*procOrder.NextRetryAt) {
		if s.queue != nil {
			return s.queue.EnqueueSubmit(procOrder.ID, time.Until(*procOrder.NextRetryAt))
		}
		return nil
	}

	// 获取连接和适配器
	connection, err := s.connections.Open(procOrder.ConnectionID)
	if err != nil {
		s.markProcurementError(procOrder, fmt.Sprintf("load connection failed: %v", err))
		return fmt.Errorf("load connection: %w", err)
	}
	if connection == nil {
		s.rejectProcurement(procOrder, "", fmt.Sprintf("connection %d not found", procOrder.ConnectionID))
		return nil // 永久性错误，不重试
	}
	singleSubmission := requiresSingleSubmission(connection)

	// 加载本地订单获取 SKU 信息
	localOrder, err := s.orderRepo.GetByID(procOrder.LocalOrderID)
	if err != nil {
		s.markProcurementError(procOrder, fmt.Sprintf("load local order failed: %v", err))
		return fmt.Errorf("load local order: %w", err)
	}
	if localOrder == nil {
		s.rejectProcurement(procOrder, "", fmt.Sprintf("local order %d not found", procOrder.LocalOrderID))
		return nil // 永久性错误，不重试
	}
	if singleSubmission && localOrder.Status != constants.OrderStatusPaid && localOrder.Status != constants.OrderStatusFulfilling {
		s.rejectProcurement(procOrder, "", "supplier purchases require a confirmed paid local order")
		return nil
	}
	if len(localOrder.Items) == 0 {
		s.rejectProcurement(procOrder, "", fmt.Sprintf("local order %d has no items", localOrder.ID))
		return nil // 永久性错误，不重试
	}
	item := localOrder.Items[0]

	// 查找 SKU 映射
	upstreamSKUID, found, err := s.skuMapRepo.FindUpstreamSKUID(item.SKUID)
	if err != nil {
		s.markProcurementError(procOrder, fmt.Sprintf("lookup sku mapping failed: %v", err))
		return fmt.Errorf("lookup sku mapping: %w", err)
	}
	if !found {
		s.rejectProcurement(procOrder, "", fmt.Sprintf("no sku mapping for local sku %d", item.SKUID))
		return nil // 永久性错误，不重试
	}

	// 构建上游请求
	req := procurementcontract.CreateOrderRequest{
		SKUID:             upstreamSKUID,
		Quantity:          item.Quantity,
		DownstreamOrderNo: localOrder.OrderNo,
		TraceID:           procOrder.TraceID,
		CallbackURL:       connection.CallbackURL(),
		MaxAmount:         item.CostPrice.Decimal.Mul(decimal.NewFromInt(int64(item.Quantity))).StringFixed(2),
		Currency:          localOrder.Currency,
	}

	// 传递人工表单数据（如有）
	if len(item.ManualFormSubmissionJSON) > 0 {
		req.ManualFormData = item.ManualFormSubmissionJSON
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if singleSubmission {
		claimer, ok := s.procRepo.(procurementcontract.SubmissionClaimer)
		if !ok {
			return fmt.Errorf("supplier balance purchases require a durable submission claim")
		}
		claimed, err := claimer.ClaimSubmission(procOrder.ID)
		if err != nil {
			return err
		}
		if !claimed {
			return nil
		}
		procOrder.Status = "submitting"
	}

	resp, err := connection.CreateOrder(ctx, req)
	if err != nil {
		if singleSubmission {
			return s.markSubmissionUnknown(procOrder, "supplier purchase outcome is unknown; reconcile before retrying")
		}
		return s.handleSubmitFailure(procOrder, connection, "", fmt.Sprintf("upstream request error: %v", err), true)
	}

	if !resp.OK {
		if singleSubmission {
			if resp.ErrorCode == "supplier_unavailable" {
				// Read-only preflight failed before the balance purchase request.
				return s.handleSubmitFailure(procOrder, connection, resp.ErrorCode, resp.ErrorMessage, true)
			}
			if resp.ErrorCode == "duplicate_order" {
				return s.markSubmissionUnknown(procOrder, "supplier already has this request; retrieve the original order before retrying")
			}
			s.rejectProcurement(procOrder, resp.ErrorCode, resp.ErrorMessage)
			return nil
		}
		retryable := isRetryableErrorCode(resp.ErrorCode)
		errMsg := resp.ErrorMessage
		if errMsg == "" {
			errMsg = resp.ErrorCode
		}
		return s.handleSubmitFailure(procOrder, connection, resp.ErrorCode, errMsg, retryable)
	}

	// 成功：更新状态，重置 retry_count 用于轮询阶段
	now := time.Now()
	updates := map[string]interface{}{
		"upstream_order_id": resp.OrderID,
		"upstream_order_no": resp.OrderNo,
		"upstream_amount":   resp.Amount,
		"upstream_currency": resp.Currency,
		"error_code":        "",
		"error_message":     "",
		"retry_count":       0,
		"next_retry_at":     nil,
		"updated_at":        now,
	}
	if resp.Fulfillment != nil {
		updates["upstream_payload"] = resp.Fulfillment.Payload
	}
	if err := s.procRepo.UpdateStatus(procOrder.ID, "accepted", updates); err != nil {
		return fmt.Errorf("update procurement status: %w", err)
	}

	logger.Infow("procurement_order_accepted",
		"procurement_order_id", procOrder.ID,
		"upstream_order_id", resp.OrderID,
		"upstream_order_no", resp.OrderNo,
	)

	// 更新本地订单状态为 fulfilling
	_ = s.orderRepo.UpdateStatus(localOrder.ID, constants.OrderStatusFulfilling, map[string]interface{}{
		"updated_at": now,
	})

	if resp.Fulfillment != nil && strings.TrimSpace(resp.Fulfillment.Payload) != "" {
		// The card is persisted with the supplier reference before local delivery.
		return s.HandleUpstreamCallback(procOrder.ID, "delivered", resp.Fulfillment)
	}

	// 入队轮询任务（30s 延迟，作为回调的 fallback）
	if s.queue != nil {
		_ = s.queue.EnqueuePoll(procOrder.ID, 30*time.Second)
	}

	return nil
}

func requiresSingleSubmission(connection procurementcontract.UpstreamConnection) bool {
	capability, ok := connection.(procurementcontract.SingleSubmissionConnection)
	return ok && capability.RequiresSingleSubmission()
}

func (s *Service) markSubmissionUnknown(order *procurementdomain.Order, message string) error {
	changed, err := s.procRepo.UpdateStatusIfCurrent(order.ID, order.Status, "manual_review", map[string]interface{}{"error_code": "", "error_message": message, "updated_at": time.Now()})
	if err != nil {
		return err
	}
	if changed {
		s.notifyProcurementFailure(order, message)
	}
	return nil
}

// markProcurementError 记录错误信息但不改变状态（用于瞬态错误，asynq 可重试）
func (s *Service) markProcurementError(procOrder *procurementdomain.Order, errMsg string) {
	now := time.Now()
	_, _ = s.procRepo.UpdateStatusIfCurrent(procOrder.ID, procOrder.Status, procOrder.Status, map[string]interface{}{
		"error_message": errMsg,
		"updated_at":    now,
	})
	logger.Warnw("procurement_prepare_error",
		"procurement_order_id", procOrder.ID,
		"error", errMsg,
	)
}

// rejectProcurement 将采购单标记为 rejected（用于永久性配置错误，不值得重试）
// 同时回退本地订单状态并通知管理员
func (s *Service) rejectProcurement(procOrder *procurementdomain.Order, errorCode, errMsg string) {
	now := time.Now()
	changed, err := s.procRepo.UpdateStatusIfCurrent(procOrder.ID, procOrder.Status, "rejected", map[string]interface{}{
		"error_code":    strings.ToLower(strings.TrimSpace(errorCode)),
		"error_message": errMsg,
		"updated_at":    now,
	})
	if err != nil || !changed {
		return
	}
	logger.Warnw("procurement_rejected_config_error",
		"procurement_order_id", procOrder.ID,
		"error", errMsg,
	)
	s.rollbackLocalOrderOnProcurementFailure(procOrder, errMsg)
}

// rollbackLocalOrderOnProcurementFailure 采购单终态失败时回退本地订单状态并通知管理员
func (s *Service) rollbackLocalOrderOnProcurementFailure(procOrder *procurementdomain.Order, errMsg string) {
	localOrder, err := s.orderRepo.GetByID(procOrder.LocalOrderID)
	if err != nil || localOrder == nil {
		return
	}
	if localOrder.Status == constants.OrderStatusFulfilling {
		now := time.Now()
		_ = s.orderRepo.UpdateStatus(localOrder.ID, constants.OrderStatusPaid, map[string]interface{}{
			"updated_at": now,
		})
		// 如果是子订单，同步父订单状态
		if localOrder.ParentID != nil && s.orderLifecycle != nil {
			_, _ = s.orderLifecycle.SyncParentStatus(*localOrder.ParentID, now)
		}
		logger.Infow("procurement_failure_order_rolled_back",
			"procurement_order_id", procOrder.ID,
			"local_order_id", localOrder.ID,
			"from_status", constants.OrderStatusFulfilling,
			"to_status", constants.OrderStatusPaid,
		)
	}
	s.notifyProcurementFailure(procOrder, errMsg)
}

// notifyProcurementFailure 发送采购失败异常告警
func (s *Service) notifyProcurementFailure(procOrder *procurementdomain.Order, errMsg string) {
	if s.notifications == nil {
		return
	}
	_ = s.notifications.NotifyFailure(procOrder, errMsg)
}

// handleSubmitFailure 处理提交失败
func (s *Service) handleSubmitFailure(procOrder *procurementdomain.Order, connection procurementcontract.UpstreamConnection, errorCode, errMsg string, retryable bool) error {
	now := time.Now()

	if retryable && procOrder.RetryCount < connection.RetryMax() {
		intervals := parseRetryIntervals(connection.RetryIntervals())
		idx := procOrder.RetryCount
		if idx >= len(intervals) {
			idx = len(intervals) - 1
		}
		delay := intervals[idx]
		nextRetry := now.Add(delay)

		updates := map[string]interface{}{
			"retry_count":   procOrder.RetryCount + 1,
			"next_retry_at": &nextRetry,
			"error_code":    strings.ToLower(strings.TrimSpace(errorCode)),
			"error_message": errMsg,
			"updated_at":    now,
		}
		if err := s.procRepo.UpdateStatus(procOrder.ID, "failed", updates); err != nil {
			return fmt.Errorf("update procurement status (failed): %w", err)
		}

		logger.Warnw("procurement_submit_failed_retryable",
			"procurement_order_id", procOrder.ID,
			"retry_count", procOrder.RetryCount+1,
			"next_retry_at", nextRetry,
			"error", errMsg,
		)

		// 入队重试
		if s.queue != nil {
			return s.queue.EnqueueSubmit(procOrder.ID, delay)
		}

		return nil
	}

	// 不可重试或已达上限：拒绝
	updates := map[string]interface{}{
		"error_code":    strings.ToLower(strings.TrimSpace(errorCode)),
		"error_message": errMsg,
		"updated_at":    now,
	}
	if err := s.procRepo.UpdateStatus(procOrder.ID, "rejected", updates); err != nil {
		return fmt.Errorf("update procurement status (rejected): %w", err)
	}

	logger.Warnw("procurement_submit_rejected",
		"procurement_order_id", procOrder.ID,
		"error", errMsg,
	)

	// 回退本地订单状态并通知管理员
	s.rollbackLocalOrderOnProcurementFailure(procOrder, errMsg)

	return fmt.Errorf("procurement rejected: %s", errMsg)
}

// isRetryableErrorCode 判断上游错误码是否可重试
func isRetryableErrorCode(code string) bool {
	nonRetryable := map[string]bool{
		"insufficient_balance": true,
		"payment_failed":       true,
		"product_unavailable":  true,
		"sku_unavailable":      true,
		"invalid_request":      true,
		"unauthorized":         true,
		"forbidden":            true,
		"duplicate_order":      true,
		"product_out_of_stock": true,
	}
	return !nonRetryable[strings.ToLower(strings.TrimSpace(code))]
}

// parseRetryIntervals 解析重试间隔配置（JSON 数组格式如 "[30,60,300]"）
func parseRetryIntervals(raw string) []time.Duration {
	raw = strings.TrimSpace(raw)
	// 移除方括号
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")

	if raw == "" {
		return []time.Duration{30 * time.Second, 60 * time.Second, 300 * time.Second}
	}

	parts := strings.Split(raw, ",")
	intervals := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		seconds, err := strconv.Atoi(part)
		if err != nil || seconds <= 0 {
			continue
		}
		intervals = append(intervals, time.Duration(seconds)*time.Second)
	}

	if len(intervals) == 0 {
		return []time.Duration{30 * time.Second, 60 * time.Second, 300 * time.Second}
	}
	return intervals
}
