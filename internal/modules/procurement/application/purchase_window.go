package application

import (
	"github.com/dujiao-next/internal/constants"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
)

// These checks run before creating/resetting/claiming any purchase intent or
// opening a supplier connection. Excluded history is left completely unchanged.
func (s *Service) orderWithinPurchaseWindow(order *procurementdomain.LocalOrder, requirePurchasable bool) (bool, error) {
	if s.purchaseWindow == nil {
		return true, nil
	}
	eligible := func(o *procurementdomain.LocalOrder) bool {
		return o != nil && o.ID > s.purchaseWindow.OrderIDFloor && o.PaidAt != nil &&
			!s.purchaseWindow.PaidAfter.IsZero() && o.PaidAt.After(s.purchaseWindow.PaidAfter)
	}
	if !eligible(order) {
		return false, nil
	}
	parentActive := func(o *procurementdomain.LocalOrder) bool {
		return o.Status == constants.OrderStatusPaid || o.Status == constants.OrderStatusFulfilling ||
			o.Status == constants.OrderStatusPartiallyDelivered || o.Status == constants.OrderStatusPartiallyRefunded
	}
	if requirePurchasable {
		if order.ParentID == nil && len(order.Children) > 0 {
			return parentActive(order), nil
		}
		if (order.Status != constants.OrderStatusPaid && order.Status != constants.OrderStatusFulfilling) || !order.RefundedAmount.Decimal.IsZero() {
			return false, nil
		}
	}
	if order.ParentID != nil {
		parent, err := s.orderRepo.GetByID(*order.ParentID)
		if err != nil {
			return false, err
		}
		return eligible(parent) && parent.ParentID == nil && (!requirePurchasable || parentActive(parent)), nil
	}
	return true, nil
}

func (s *Service) procurementWithinPurchaseWindow(order *procurementdomain.Order) (bool, error) {
	if s.purchaseWindow == nil {
		return true, nil
	}
	if order == nil || !order.CreatedAt.After(s.purchaseWindow.PaidAfter) {
		return false, nil
	}
	local, err := s.orderRepo.GetByID(order.LocalOrderID)
	if err != nil {
		return false, err
	}
	if local == nil {
		return false, procurementcontract.ErrOrderNotFound
	}
	// An accepted purchase only queues a read-only poll. A crash may leave the
	// local delivery committed before the procurement's final status is saved.
	return s.orderWithinPurchaseWindow(local, order.Status != "accepted")
}
