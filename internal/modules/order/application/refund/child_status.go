package refund

import (
	"strings"
	"time"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/logger"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
)

// applyParentRefundChildStatusUpdates 根据父订单退款结果统一更新子订单状态。
// 规则:子订单退款状态始终跟随父订单,避免分支逻辑造成状态混乱。
//
// orderStore 必须来自订单工作单元，确保父子状态在同一事务中更新。
func applyParentRefundChildStatusUpdates(orderStore ordercontract.Store, parentOrderID uint, parentTargetStatus string, now time.Time) error {
	if orderStore == nil || parentOrderID == 0 {
		return nil
	}

	target := strings.ToLower(strings.TrimSpace(parentTargetStatus))
	if target != constants.OrderStatusPartiallyRefunded && target != constants.OrderStatusRefunded {
		return nil
	}

	affected, err := orderStore.UpdateChildrenStatus(parentOrderID, target, now)
	if err != nil {
		return err
	}
	if affected > 0 {
		logger.Debugw("refund_child_status_propagated",
			"parent_order_id", parentOrderID,
			"target_status", target,
			"rows_affected", affected,
		)
	}
	return nil
}
