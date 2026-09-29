package autorechargebootstrap

import (
	"context"
	"fmt"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	fulfillmentapp "github.com/dujiao-next/internal/modules/fulfillment/application"
	ordercontract "github.com/dujiao-next/internal/modules/order/contract"
	"gorm.io/gorm"
	"strings"
)

type Orders struct {
	DB           *gorm.DB
	Store        ordercontract.Store
	Fulfillment  *fulfillmentapp.Service
	RechargeSKUs map[uint]bool
	RedeemURL    string
}

func (o *Orders) Resolve(ctx context.Context, orderNo string, userID uint) (uint, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	r, err := o.Store.GetAnyByOrderNoAndUser(orderNo, userID)
	if err != nil {
		return 0, err
	}
	if r == nil || r.ResellerID != nil {
		return 0, contract.ErrNotFound
	}
	return r.ID, nil
}

func (o *Orders) Get(ctx context.Context, id uint) (*contract.Order, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r, err := o.Store.GetByID(id)
	if err != nil || r == nil {
		return nil, err
	}
	// Phase one deliberately limits credentials to the main site, one account per order.
	if r.ResellerID != nil {
		return nil, nil
	}
	result := &contract.Order{ID: r.ID, UserID: r.UserID, Paid: r.PaidAt != nil && r.RefundedAmount.IsZero() && (r.Status == constants.OrderStatusPaid || r.Status == constants.OrderStatusFulfilling || r.Status == constants.OrderStatusDelivered || r.Status == constants.OrderStatusCompleted)}
	if len(r.Items) > 0 {
		result.SKUID = r.Items[0].SKUID
	}
	// Mixed orders must still be recorded for review when a later item is
	// mapped, otherwise the recovery scan would rediscover them indefinitely.
	for _, item := range r.Items {
		if o.RechargeSKUs[item.SKUID] {
			result.SKUID = item.SKUID
			break
		}
	}
	result.Eligible = len(r.Children) == 0 && len(r.Items) == 1 && r.Items[0].Quantity == 1 && r.Items[0].FulfillmentType == constants.FulfillmentTypeManual
	if r.Fulfillment != nil && !strings.HasPrefix(r.Fulfillment.Payload, "GPT 订阅兑换卡密\n") {
		result.Eligible = false
	}
	if r.DeletedAt != nil || ((r.Status == constants.OrderStatusDelivered || r.Status == constants.OrderStatusCompleted) && r.Fulfillment == nil) {
		result.Eligible = false
	}
	return result, nil
}
func (o *Orders) Children(ctx context.Context, id uint) ([]uint, error) {
	rows, err := o.Store.ListChildren(id)
	if err != nil {
		return nil, err
	}
	ids := make([]uint, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids, nil
}
func (o *Orders) Unscheduled(ctx context.Context, skus []uint, limit int) ([]uint, error) {
	var ids []uint
	err := o.DB.WithContext(ctx).Table("orders AS o").Joins("JOIN order_items AS i ON i.order_id = o.id").Joins("LEFT JOIN auto_recharge_tasks AS t ON t.order_id = o.id").Where("t.id IS NULL AND o.deleted_at IS NULL AND i.deleted_at IS NULL AND o.paid_at IS NOT NULL AND o.reseller_id IS NULL AND o.refunded_amount = 0 AND o.status IN ? AND i.sku_id IN ?", []string{constants.OrderStatusPaid, constants.OrderStatusFulfilling}, skus).Distinct("o.id").Order("o.id").Limit(limit).Pluck("o.id", &ids).Error
	return ids, err
}
func (o *Orders) DeliverCode(ctx context.Context, id uint, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	prefix := fmt.Sprintf("GPT 订阅兑换卡密\n%s\n", code)
	payload := prefix + "兑换地址：" + o.RedeemURL + "\n请核验账号后确认充值。不要向他人泄露卡密或 Session。"
	r, err := o.Store.GetByID(id)
	if err != nil {
		return err
	}
	if r == nil {
		return contract.ErrNotFound
	}
	if r.Fulfillment != nil {
		if strings.HasPrefix(r.Fulfillment.Payload, prefix) {
			return nil
		}
		return contract.ErrLocked
	}
	_, err = o.Fulfillment.DeliverRechargeCode(id, payload)
	return err
}
