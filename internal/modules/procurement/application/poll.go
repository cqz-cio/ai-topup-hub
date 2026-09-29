package application

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/logger"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
)

// pollIntervals 短期轮询间隔：捕获自动交付等快速场景（共约30分钟后停止）
// 超时后不标记失败，交由回调和定时巡检接管
var pollIntervals = []time.Duration{
	30 * time.Second, 30 * time.Second,
	1 * time.Minute, 1 * time.Minute,
	2 * time.Minute, 2 * time.Minute,
	5 * time.Minute, 5 * time.Minute,
	10 * time.Minute,
}

// PollUpstreamStatus Worker 调用：轮询上游订单状态
func (s *Service) PollUpstreamStatus(procurementOrderID uint) error {
	procOrder, err := s.procRepo.GetByID(procurementOrderID)
	if err != nil {
		return fmt.Errorf("load procurement order: %w", err)
	}
	if procOrder == nil {
		return procurementcontract.ErrNotFound
	}

	// 只轮询 accepted 状态的订单
	if procOrder.Status != "accepted" {
		return nil
	}

	connection, err := s.connections.Open(procOrder.ConnectionID)
	if err != nil {
		return fmt.Errorf("load connection: %w", err)
	}
	if connection == nil {
		return procurementcontract.ErrConnectionNotFound
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	detail, err := connection.GetOrder(ctx, procOrder.UpstreamOrderID)
	if err != nil {
		logger.Warnw("procurement_poll_status_error",
			"procurement_order_id", procOrder.ID,
			"upstream_order_id", procOrder.UpstreamOrderID,
			"error", err,
		)
		// 轮询失败，重新入队
		return s.requeuePoll(procOrder)
	}

	mappedStatus := mapProcurementUpstreamStatus(detail.Status)
	switch mappedStatus {
	case "delivered":
		return s.HandleUpstreamCallback(procOrder.ID, mappedStatus, detail.Fulfillment)
	case "canceled":
		return s.HandleUpstreamCallback(procOrder.ID, mappedStatus, nil)
	case "refunded", "partially_refunded":
		return s.HandleUpstreamCallback(procOrder.ID, mappedStatus, detail.Fulfillment)
	default:
		// 状态未变，继续轮询
		return s.requeuePoll(procOrder)
	}
}

// requeuePoll 重新入队轮询任务
func (s *Service) requeuePoll(procOrder *procurementdomain.Order) error {
	if s.queue == nil {
		return nil
	}

	idx := procOrder.RetryCount
	if idx >= len(pollIntervals) {
		// 短期轮询结束，后续由定时巡检和回调接管，不标记失败
		logger.Infow("procurement_poll_handoff_to_periodic_sync",
			"procurement_order_id", procOrder.ID,
			"retry_count", procOrder.RetryCount,
		)
		return nil
	}

	delay := pollIntervals[idx]

	// 递增轮询计数
	now := time.Now()
	_ = s.procRepo.UpdateStatus(procOrder.ID, procOrder.Status, map[string]interface{}{
		"retry_count": procOrder.RetryCount + 1,
		"updated_at":  now,
	})

	return s.queue.EnqueuePoll(procOrder.ID, delay)
}

// SyncAcceptedOrders 定时巡检：检查所有 accepted 状态的采购单，向上游查询最新状态
// 由 worker 定时任务调用（每30分钟）
func (s *Service) SyncAcceptedOrders() {
	orders, _, err := s.procRepo.List(procurementcontract.ListFilter{
		Status:   "accepted",
		Page:     1,
		PageSize: 200,
	})
	if err != nil {
		logger.Warnw("procurement_sync_accepted_list_failed", "error", err)
		return
	}
	if len(orders) == 0 {
		return
	}

	logger.Infow("procurement_sync_accepted_start", "count", len(orders))

	for i := range orders {
		procOrder := &orders[i]
		if procOrder.UpstreamOrderID == 0 {
			continue
		}

		connection, err := s.connections.Open(procOrder.ConnectionID)
		if err != nil || connection == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		detail, err := connection.GetOrder(ctx, procOrder.UpstreamOrderID)
		cancel()

		if err != nil {
			logger.Warnw("procurement_sync_accepted_poll_error",
				"procurement_order_id", procOrder.ID,
				"upstream_order_id", procOrder.UpstreamOrderID,
				"error", err,
			)
			continue
		}

		mappedStatus := mapProcurementUpstreamStatus(detail.Status)
		switch mappedStatus {
		case "delivered":
			if cbErr := s.HandleUpstreamCallback(procOrder.ID, mappedStatus, detail.Fulfillment); cbErr != nil {
				logger.Warnw("procurement_sync_accepted_deliver_failed",
					"procurement_order_id", procOrder.ID,
					"error", cbErr,
				)
			} else {
				logger.Infow("procurement_sync_accepted_delivered",
					"procurement_order_id", procOrder.ID,
				)
			}
		case "canceled":
			_ = s.HandleUpstreamCallback(procOrder.ID, mappedStatus, nil)
			logger.Infow("procurement_sync_accepted_canceled",
				"procurement_order_id", procOrder.ID,
			)
		case "refunded", "partially_refunded":
			if cbErr := s.HandleUpstreamCallback(procOrder.ID, mappedStatus, detail.Fulfillment); cbErr != nil {
				logger.Warnw("procurement_sync_accepted_refund_failed",
					"procurement_order_id", procOrder.ID,
					"upstream_status", mappedStatus,
					"error", cbErr,
				)
			} else {
				logger.Infow("procurement_sync_accepted_refunded",
					"procurement_order_id", procOrder.ID,
					"upstream_status", mappedStatus,
				)
			}
		default:
			// 检查是否超时（超过 24 小时仍在 accepted 状态）
			acceptedDuration := time.Since(procOrder.UpdatedAt)
			if acceptedDuration > 24*time.Hour {
				logger.Warnw("procurement_accepted_timeout",
					"procurement_order_id", procOrder.ID,
					"upstream_order_id", procOrder.UpstreamOrderID,
					"accepted_duration", acceptedDuration.String(),
				)
				s.notifyProcurementFailure(procOrder, fmt.Sprintf(
					"procurement order stuck in accepted for %s, upstream status: %s",
					acceptedDuration.Round(time.Hour), detail.Status))
			}
		}
	}
}

// mapProcurementUpstreamStatus 统一映射上游状态别名，便于回调与轮询使用同一分支逻辑。
func mapProcurementUpstreamStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "delivered", "completed", "fulfilled":
		return "delivered"
	case "canceled", "cancelled":
		return "canceled"
	case "refunded", "partially_refunded":
		return normalized
	default:
		return normalized
	}
}
