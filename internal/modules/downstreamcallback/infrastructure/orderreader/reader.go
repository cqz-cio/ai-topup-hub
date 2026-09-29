package orderreader

import (
	downstreamcontract "github.com/dujiao-next/internal/modules/downstreamcallback/contract"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
)

// Source 是旧订单上下文暴露给防腐适配器的最小读取端口。
type Source interface {
	GetByID(id uint) (*orderdomain.Order, error)
}

// Reader 将订单持久化模型投影为下游回调读模型。
type Reader struct {
	source Source
}

var _ downstreamcontract.OrderReader = (*Reader)(nil)

func New(source Source) *Reader {
	if source == nil {
		panic("downstream callback order reader: source is nil")
	}
	return &Reader{source: source}
}

func (r *Reader) GetByID(id uint) (*downstreamcontract.OrderSnapshot, error) {
	order, err := r.source.GetByID(id)
	if err != nil || order == nil {
		return nil, err
	}
	return projectOrder(order), nil
}

func projectOrder(order *orderdomain.Order) *downstreamcontract.OrderSnapshot {
	if order == nil {
		return nil
	}
	projection := &downstreamcontract.OrderSnapshot{
		ID:       order.ID,
		OrderNo:  order.OrderNo,
		ParentID: order.ParentID,
		Status:   order.Status,
	}
	if order.Fulfillment != nil {
		projection.Fulfillment = &downstreamcontract.Fulfillment{
			Type:         order.Fulfillment.Type,
			Status:       order.Fulfillment.Status,
			Payload:      order.Fulfillment.Payload,
			DeliveryData: order.Fulfillment.LogisticsJSON,
			DeliveredAt:  order.Fulfillment.DeliveredAt,
		}
	}
	projection.Children = make([]downstreamcontract.OrderSnapshot, 0, len(order.Children))
	for i := range order.Children {
		child := projectOrder(&order.Children[i])
		if child != nil {
			projection.Children = append(projection.Children, *child)
		}
	}
	return projection
}
