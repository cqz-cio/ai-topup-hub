package presenter

import (
	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// OrderDelivery is only returned by the separately authorized delivery endpoint.
// Ordinary order responses must never contain its payload, structured data or instructions.
type OrderDelivery struct {
	OrderNo     string          `json:"order_no"`
	Fulfillment FulfillmentResp `json:"fulfillment"`
	Items       []DeliveryItem  `json:"items"`
}

type DeliveryItem struct {
	Title        jsonmap.JSON `json:"title"`
	Instructions jsonmap.JSON `json:"instructions,omitempty"`
}

func NewOrderDelivery(o *orderdomain.Order) *OrderDelivery {
	if o == nil || o.PaidAt == nil || o.Fulfillment == nil || o.Fulfillment.Status != "delivered" {
		return nil
	}
	d := &OrderDelivery{OrderNo: o.OrderNo, Fulfillment: newFulfillmentResp(o.Fulfillment), Items: []DeliveryItem{}}
	d.Fulfillment.truncatePayload(fulfillmentdomain.PayloadMaxPreviewLines)
	for _, item := range o.Items {
		d.Items = append(d.Items, DeliveryItem{Title: item.TitleJSON, Instructions: item.InstructionsJSON})
	}
	return d
}

// AdminOrderMetadata returns an independent copy, including nested child orders.
// Keep delivery material out of list, detail and status-update responses alike.
func AdminOrderMetadata(o orderdomain.Order) orderdomain.Order {
	if o.Fulfillment != nil {
		f := *o.Fulfillment
		f.Payload = ""
		f.LogisticsJSON = nil
		o.Fulfillment = &f
	}
	o.Items = append([]orderdomain.OrderItem(nil), o.Items...)
	for i := range o.Items {
		o.Items[i].InstructionsJSON = nil
	}
	o.Children = append([]orderdomain.Order(nil), o.Children...)
	for i := range o.Children {
		o.Children[i] = AdminOrderMetadata(o.Children[i])
	}
	return o
}
