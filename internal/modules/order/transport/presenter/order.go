package presenter

import (
	"strings"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/jsonslice"
	"github.com/dujiao-next/internal/shared/money"
)

// OrderSummary 订单列表响应（精简字段）
type OrderSummary struct {
	OrderNo                 string          `json:"order_no"`
	Status                  string          `json:"status"`
	Currency                string          `json:"currency"`
	DiscountAmount          money.Amount    `json:"discount_amount"`
	MemberDiscountAmount    money.Amount    `json:"member_discount_amount"`
	PromotionDiscountAmount money.Amount    `json:"promotion_discount_amount"`
	WholesaleDiscountAmount money.Amount    `json:"wholesale_discount_amount"`
	TotalAmount             money.Amount    `json:"total_amount"`
	CreatedAt               time.Time       `json:"created_at"`
	Items                   []OrderItemResp `json:"items,omitempty"`
	Children                []OrderSummary  `json:"children,omitempty"`
}

// NewOrderSummary 从 orderdomain.Order 构造 OrderSummary
func NewOrderSummary(o *orderdomain.Order) OrderSummary {
	s := OrderSummary{
		OrderNo:                 o.OrderNo,
		Status:                  o.Status,
		Currency:                o.Currency,
		DiscountAmount:          o.DiscountAmount,
		MemberDiscountAmount:    o.MemberDiscountAmount,
		PromotionDiscountAmount: o.PromotionDiscountAmount,
		WholesaleDiscountAmount: o.WholesaleDiscountAmount,
		TotalAmount:             o.TotalAmount,
		CreatedAt:               o.CreatedAt,
	}
	for _, item := range o.Items {
		resp := newOrderItemResp(&item)
		resp.Instructions = nil
		s.Items = append(s.Items, resp)
	}
	for i := range o.Children {
		child := NewOrderSummary(&o.Children[i])
		s.Children = append(s.Children, child)
	}
	return s
}

// NewOrderSummaryList 批量转换订单列表
func NewOrderSummaryList(orders []orderdomain.Order) []OrderSummary {
	result := make([]OrderSummary, 0, len(orders))
	for i := range orders {
		result = append(result, NewOrderSummary(&orders[i]))
	}
	return result
}

// OrderDetail 订单详情响应（完整字段）
type OrderDetail struct {
	OrderNo                  string            `json:"order_no"`
	GuestEmail               string            `json:"guest_email,omitempty"`
	GuestLocale              string            `json:"guest_locale,omitempty"`
	Status                   string            `json:"status"`
	Currency                 string            `json:"currency"`
	OriginalAmount           money.Amount      `json:"original_amount"`
	DiscountAmount           money.Amount      `json:"discount_amount"`
	MemberDiscountAmount     money.Amount      `json:"member_discount_amount"`
	PromotionDiscountAmount  money.Amount      `json:"promotion_discount_amount"`
	WholesaleDiscountAmount  money.Amount      `json:"wholesale_discount_amount"`
	TotalAmount              money.Amount      `json:"total_amount"`
	WalletPaidAmount         money.Amount      `json:"wallet_paid_amount"`
	OnlinePaidAmount         money.Amount      `json:"online_paid_amount"`
	RefundedAmount           money.Amount      `json:"refunded_amount"`
	ExpiresAt                *time.Time        `json:"expires_at"`
	PaidAt                   *time.Time        `json:"paid_at"`
	CanceledAt               *time.Time        `json:"canceled_at"`
	CreatedAt                time.Time         `json:"created_at"`
	AllowedPaymentChannelIDs []uint            `json:"allowed_payment_channel_ids,omitempty"`
	RefundRecords            []OrderRefundResp `json:"refund_records,omitempty"`
	Items                    []OrderItemResp   `json:"items,omitempty"`
	Fulfillment              *FulfillmentResp  `json:"fulfillment,omitempty"`
	Children                 []OrderDetail     `json:"children,omitempty"`
}

// OrderRefundResp 用户侧订单退款记录响应
type OrderRefundResp struct {
	Type      string       `json:"type"`
	Amount    money.Amount `json:"amount"`
	Currency  string       `json:"currency"`
	Remark    string       `json:"remark,omitempty"`
	CreatedAt time.Time    `json:"created_at"`
}

// NewOrderDetail 从 orderdomain.Order 构造 OrderDetail，
// 内部自动处理 upstream 类型伪装和成本价清除。
func NewOrderDetail(o *orderdomain.Order) OrderDetail {
	d := OrderDetail{
		OrderNo:                 o.OrderNo,
		GuestEmail:              o.GuestEmail,
		GuestLocale:             o.GuestLocale,
		Status:                  o.Status,
		Currency:                o.Currency,
		OriginalAmount:          o.OriginalAmount,
		DiscountAmount:          o.DiscountAmount,
		MemberDiscountAmount:    o.MemberDiscountAmount,
		PromotionDiscountAmount: o.PromotionDiscountAmount,
		WholesaleDiscountAmount: o.WholesaleDiscountAmount,
		TotalAmount:             o.TotalAmount,
		WalletPaidAmount:        o.WalletPaidAmount,
		OnlinePaidAmount:        o.OnlinePaidAmount,
		RefundedAmount:          o.RefundedAmount,
		ExpiresAt:               o.ExpiresAt,
		PaidAt:                  o.PaidAt,
		CanceledAt:              o.CanceledAt,
		CreatedAt:               o.CreatedAt,
	}
	paid := o.PaidAt != nil
	for _, item := range o.Items {
		resp := newOrderItemResp(&item)
		if !paid {
			resp.Instructions = nil
		}
		d.Items = append(d.Items, resp)
	}
	if o.Fulfillment != nil {
		fr := newFulfillmentResp(o.Fulfillment)
		d.Fulfillment = &fr
	}
	for i := range o.Children {
		d.Children = append(d.Children, NewOrderDetail(&o.Children[i]))
	}
	return d
}

// NewOrderDetailTruncated 同 NewOrderDetail，但额外截断交付内容。
func NewOrderDetailTruncated(o *orderdomain.Order) OrderDetail {
	d := NewOrderDetail(o)
	truncateFulfillment(&d)
	return d
}

func truncateFulfillment(d *OrderDetail) {
	if d.Fulfillment != nil {
		d.Fulfillment.truncatePayload(fulfillmentdomain.PayloadMaxPreviewLines)
	}
	for i := range d.Children {
		truncateFulfillment(&d.Children[i])
	}
}

// OrderItemResp 订单项响应
type OrderItemResp struct {
	Title                    jsonmap.JSON      `json:"title"`
	SKUSnapshot              jsonmap.JSON      `json:"sku_snapshot"`
	Tags                     jsonslice.Strings `json:"tags"`
	Quantity                 int               `json:"quantity"`
	OriginalUnitPrice        money.Amount      `json:"original_unit_price"`
	UnitPrice                money.Amount      `json:"unit_price"`
	OriginalTotalPrice       money.Amount      `json:"original_total_price"`
	TotalPrice               money.Amount      `json:"total_price"`
	CouponDiscountAmount     money.Amount      `json:"coupon_discount_amount"`
	MemberDiscountAmount     money.Amount      `json:"member_discount_amount"`
	PromotionDiscountAmount  money.Amount      `json:"promotion_discount_amount"`
	WholesaleDiscountAmount  money.Amount      `json:"wholesale_discount_amount"`
	FulfillmentType          string            `json:"fulfillment_type"`
	ManualFormSchemaSnapshot jsonmap.JSON      `json:"manual_form_schema_snapshot"`
	ManualFormSubmission     jsonmap.JSON      `json:"manual_form_submission"`
	// Instructions 交付使用说明（多语言 raw JSON，与 Title 字段契约一致，由前端按 locale 解析）。
	// 仅在订单已付款时填充，未付款订单会在 NewOrderDetail 中置 nil；列表场景（NewOrderSummary）永远为 nil。
	// 注意：渠道 API（channel_order.go）为服务端消费者（如 Telegram Bot），那里返回已按 locale 解析的字符串而非 raw JSON。
	Instructions jsonmap.JSON `json:"instructions,omitempty"`
}

func newOrderItemResp(item *orderdomain.OrderItem) OrderItemResp {
	ft := item.FulfillmentType
	if ft == "upstream" {
		ft = "manual"
	}
	return OrderItemResp{
		Title:                    item.TitleJSON,
		SKUSnapshot:              item.SKUSnapshotJSON,
		Tags:                     item.Tags,
		Quantity:                 item.Quantity,
		OriginalUnitPrice:        item.OriginalUnitPrice,
		UnitPrice:                item.UnitPrice,
		OriginalTotalPrice:       item.OriginalTotalPrice,
		TotalPrice:               item.TotalPrice,
		CouponDiscountAmount:     item.CouponDiscount,
		MemberDiscountAmount:     item.MemberDiscount,
		PromotionDiscountAmount:  item.PromotionDiscount,
		WholesaleDiscountAmount:  item.WholesaleDiscount,
		FulfillmentType:          ft,
		ManualFormSchemaSnapshot: item.ManualFormSchemaSnapshotJSON,
		ManualFormSubmission:     item.ManualFormSubmissionJSON,
		Instructions:             item.InstructionsJSON,
	}
	// 注意：CostPrice 不在 DTO 中，白名单模式天然排除
}

// FulfillmentResp 交付记录响应
type FulfillmentResp struct {
	Type             string       `json:"type"`
	Status           string       `json:"status"`
	Payload          string       `json:"payload"`
	PayloadLineCount int          `json:"payload_line_count"`
	DeliveryData     jsonmap.JSON `json:"delivery_data"`
	DeliveredAt      *time.Time   `json:"delivered_at,omitempty"`
}

func newFulfillmentResp(f *fulfillmentdomain.Fulfillment) FulfillmentResp {
	typ := f.Type
	if typ == "upstream" {
		typ = "manual"
	}
	return FulfillmentResp{
		Type:             typ,
		Status:           f.Status,
		Payload:          f.Payload,
		PayloadLineCount: f.PayloadLineCount,
		DeliveryData:     f.LogisticsJSON,
		DeliveredAt:      f.DeliveredAt,
	}
	// 注意：OrderID、DeliveredBy、CreatedAt、UpdatedAt 不在 DTO 中
}

func (fr *FulfillmentResp) truncatePayload(maxLines int) {
	if fr.Payload == "" {
		return
	}
	lines := strings.Split(fr.Payload, "\n")
	fr.PayloadLineCount = len(lines)
	if len(lines) > maxLines {
		fr.Payload = strings.Join(lines[:maxLines], "\n")
	}
}
