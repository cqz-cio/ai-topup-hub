package orderreader

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	procurementdomain "github.com/dujiao-next/internal/modules/procurement/domain"
)

func (r *Reader) ReadBalanceRecoveryOrder(ctx context.Context, id uint) (*procurementcontract.BalanceRecoveryOrder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	order, err := r.source.GetByID(id)
	if err != nil || order == nil {
		return nil, err
	}
	recipient := strings.TrimSpace(order.GuestEmail)
	if order.UserID != 0 {
		resolver, ok := r.source.(interface{ ResolveReceiverEmailByOrderID(uint) (string, error) })
		if !ok {
			return nil, errors.New("order email resolver unavailable")
		}
		recipient, err = resolver.ResolveReceiverEmailByOrderID(id)
		if err != nil {
			return nil, err
		}
	}
	var products []string
	var walk func(*orderdomain.Order)
	walk = func(current *orderdomain.Order) {
		if len(current.Children) > 0 {
			for i := range current.Children {
				walk(&current.Children[i])
			}
			return
		}
		for _, item := range current.Items {
			title := "商品"
			keys := []string{"zh-CN", "zh-TW", "en-US"}
			var other []string
			for key := range item.TitleJSON {
				other = append(other, key)
			}
			sort.Strings(other)
			for _, key := range append(keys, other...) {
				if v, ok := item.TitleJSON[key].(string); ok && strings.TrimSpace(v) != "" {
					title = strings.TrimSpace(v)
					break
				}
			}
			products = append(products, fmt.Sprintf("%s × %d", title, item.Quantity))
		}
	}
	walk(order)
	return &procurementcontract.BalanceRecoveryOrder{OrderNo: order.OrderNo, Recipient: recipient, Products: strings.Join(products, "；"), Status: order.Status, ExpiresAt: order.ExpiresAt, CanceledAt: order.CanceledAt, PaidAt: order.PaidAt, ResellerID: order.ResellerID}, nil
}

type Source interface {
	GetByID(id uint) (*orderdomain.Order, error)
	GetByIDs(ids []uint) ([]orderdomain.Order, error)
	UpdateStatus(id uint, status string, updates map[string]interface{}) error
}

type Reader struct {
	source Source
}

var _ procurementcontract.OrderRepository = (*Reader)(nil)

func New(source Source) *Reader {
	if source == nil {
		panic("procurement order reader: source is nil")
	}
	return &Reader{source: source}
}

func (r *Reader) GetByID(id uint) (*procurementdomain.LocalOrder, error) {
	order, err := r.source.GetByID(id)
	if err != nil || order == nil {
		return nil, err
	}
	mapped := MapOrder(*order)
	return &mapped, nil
}

func (r *Reader) GetByIDs(ids []uint) ([]procurementdomain.LocalOrder, error) {
	orders, err := r.source.GetByIDs(ids)
	if err != nil {
		return nil, err
	}
	result := make([]procurementdomain.LocalOrder, 0, len(orders))
	for _, order := range orders {
		result = append(result, MapOrder(order))
	}
	return result, nil
}

func (r *Reader) UpdateStatus(id uint, status string, updates map[string]interface{}) error {
	return r.source.UpdateStatus(id, status, updates)
}

// MapOrder 将订单域的持久化实体收窄为采购上下文快照。
func MapOrder(order orderdomain.Order) procurementdomain.LocalOrder {
	result := procurementdomain.LocalOrder{
		ID: order.ID, OrderNo: order.OrderNo, ParentID: order.ParentID,
		UserID: order.UserID, GuestEmail: order.GuestEmail, Status: order.Status,
		PaidAt:   order.PaidAt,
		Currency: order.Currency, TotalAmount: order.TotalAmount, RefundedAmount: order.RefundedAmount,
		Items:    make([]procurementdomain.LocalOrderItem, 0, len(order.Items)),
		Children: make([]procurementdomain.LocalOrder, 0, len(order.Children)),
	}
	for _, item := range order.Items {
		result.Items = append(result.Items, procurementdomain.LocalOrderItem{
			ProductID: item.ProductID, SKUID: item.SKUID,
			Title: item.TitleJSON, SKUSnapshot: item.SKUSnapshotJSON,
			CostPrice: item.CostPrice, Quantity: item.Quantity, TotalPrice: item.TotalPrice,
			FulfillmentType: item.FulfillmentType, ManualFormSubmissionJSON: item.ManualFormSubmissionJSON,
		})
	}
	for _, child := range order.Children {
		result.Children = append(result.Children, MapOrder(child))
	}
	return result
}
