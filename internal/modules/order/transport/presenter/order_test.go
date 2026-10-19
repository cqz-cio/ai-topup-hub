package presenter

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	fulfillmentdomain "github.com/dujiao-next/internal/modules/fulfillment/domain"
	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func newMoney(value string) money.Amount {
	return money.FromDecimal(decimal.RequireFromString(value))
}

func TestOrderDetailOmitsSensitiveFields(t *testing.T) {
	now := time.Now()
	couponID := uint(10)
	promotionID := uint(20)
	affiliateID := uint(30)
	memberLevelID := uint(5)

	order := &orderdomain.Order{
		ID:                 1,
		OrderNo:            "ORD-001",
		UserID:             99,
		GuestEmail:         "guest@test.com",
		GuestPassword:      "secret123",
		Status:             "paid",
		Currency:           "CNY",
		TotalAmount:        newMoney("100.00"),
		ClientIP:           "192.168.1.1",
		AffiliateProfileID: &affiliateID,
		AffiliateCode:      "AFF-SECRET",
		CouponID:           &couponID,
		PromotionID:        &promotionID,
		MemberLevelID:      &memberLevelID,
		CreatedAt:          now,
		UpdatedAt:          now,
		Items: []orderdomain.OrderItem{
			{
				ID:                 1,
				OrderID:            1,
				ProductID:          5,
				SKUID:              10,
				TitleJSON:          jsonmap.JSON{"zh-CN": "商品A"},
				CostPrice:          newMoney("50.00"),
				OriginalUnitPrice:  newMoney("120.00"),
				UnitPrice:          newMoney("100.00"),
				OriginalTotalPrice: newMoney("120.00"),
				TotalPrice:         newMoney("100.00"),
				Quantity:           1,
				FulfillmentType:    "upstream",
			},
		},
		Fulfillment: &fulfillmentdomain.Fulfillment{
			ID:     1,
			Type:   "upstream",
			Status: "delivered",
		},
	}

	detail := NewOrderDetail(order)
	data, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	jsonStr := string(data)

	// 敏感字段不应出现
	sensitiveFields := []string{
		"client_ip", "affiliate_profile_id", "affiliate_code",
		"coupon_id", "promotion_id", "member_level_id",
		"user_id", "guest_password", "updated_at",
		"cost_price", "order_id", "product_id", "sku_id",
		"delivered_by", "parent_id",
	}
	for _, field := range sensitiveFields {
		if strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("sensitive field %q should not appear in JSON", field)
		}
	}

	// 公开字段应存在
	publicFields := []string{
		"order_no", "total_amount", "status", "currency",
		"guest_email", "original_unit_price", "unit_price",
		"original_total_price", "fulfillment_type",
	}
	for _, field := range publicFields {
		if !strings.Contains(jsonStr, `"`+field+`"`) {
			t.Errorf("public field %q should appear in JSON", field)
		}
	}

	// upstream 应被伪装为 manual
	if strings.Contains(jsonStr, `"upstream"`) {
		t.Error("upstream fulfillment type should be masked as manual")
	}
	if !strings.Contains(jsonStr, `"manual"`) {
		t.Error("fulfillment type should be manual after masking")
	}
}

func TestOrderDetailHidesInstructionsBeforePayment(t *testing.T) {
	instructions := jsonmap.JSON{"zh-CN": "账号使用方法：…"}
	mkOrder := func(paidAt *time.Time) *orderdomain.Order {
		return &orderdomain.Order{
			ID:          42,
			OrderNo:     "ORD-INST",
			Status:      "pending_payment",
			Currency:    "CNY",
			TotalAmount: newMoney("9.90"),
			PaidAt:      paidAt,
			Items: []orderdomain.OrderItem{
				{
					ID:               1,
					ProductID:        5,
					SKUID:            10,
					TitleJSON:        jsonmap.JSON{"zh-CN": "商品A"},
					InstructionsJSON: instructions,
					FulfillmentType:  "auto",
				},
			},
		}
	}

	// 未付款：instructions 必须被屏蔽
	unpaid := NewOrderDetail(mkOrder(nil))
	if unpaid.Items[0].Instructions != nil {
		t.Fatalf("unpaid order must not expose instructions, got %v", unpaid.Items[0].Instructions)
	}

	// 已付款：普通详情仍不返回交付说明
	now := time.Now()
	paid := NewOrderDetail(mkOrder(&now))
	if paid.Items[0].Instructions != nil {
		t.Fatal("paid order metadata must not expose instructions")
	}

	// Summary 路径永远不应包含 instructions
	summary := NewOrderSummary(mkOrder(&now))
	if summary.Items[0].Instructions != nil {
		t.Fatal("order summary must never expose instructions")
	}
}

func TestOrderSummaryOmitsSensitiveFields(t *testing.T) {
	order := &orderdomain.Order{
		ID:                      1,
		OrderNo:                 "ORD-002",
		UserID:                  99,
		ClientIP:                "10.0.0.1",
		AffiliateCode:           "AFF-X",
		Status:                  "pending",
		Currency:                "USD",
		DiscountAmount:          newMoney("5.00"),
		MemberDiscountAmount:    newMoney("2.00"),
		PromotionDiscountAmount: newMoney("3.00"),
		TotalAmount:             newMoney("50.00"),
		CreatedAt:               time.Now(),
	}

	summary := NewOrderSummary(order)
	data, _ := json.Marshal(summary)
	jsonStr := string(data)

	if strings.Contains(jsonStr, `"client_ip"`) {
		t.Error("client_ip should not appear in summary")
	}
	if strings.Contains(jsonStr, `"user_id"`) {
		t.Error("user_id should not appear in summary")
	}
	if summary.OrderNo != "ORD-002" {
		t.Errorf("expected order_no=ORD-002, got %s", summary.OrderNo)
	}
	if summary.DiscountAmount.String() != "5.00" {
		t.Errorf("expected discount_amount=5.00, got %s", summary.DiscountAmount.String())
	}
	if summary.MemberDiscountAmount.String() != "2.00" {
		t.Errorf("expected member_discount_amount=2.00, got %s", summary.MemberDiscountAmount.String())
	}
	if summary.PromotionDiscountAmount.String() != "3.00" {
		t.Errorf("expected promotion_discount_amount=3.00, got %s", summary.PromotionDiscountAmount.String())
	}
}

func TestNewOrderDetailTruncated(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "line"
	}
	order := &orderdomain.Order{
		ID:      1,
		OrderNo: "ORD-003",
		Status:  "delivered",
		Fulfillment: &fulfillmentdomain.Fulfillment{
			ID:      1,
			Type:    "auto",
			Status:  "delivered",
			Payload: strings.Join(lines, "\n"),
		},
	}

	now := time.Now()
	order.PaidAt = &now
	metadata := NewOrderDetailTruncated(order)
	if metadata.Fulfillment.Payload != "" {
		t.Fatal("ordinary detail exposed delivery preview")
	}
	detail := NewOrderDelivery(order)
	if detail == nil {
		t.Fatal("expected fulfillment")
	}
	if detail.Fulfillment.PayloadLineCount != 200 {
		t.Errorf("expected 200 lines, got %d", detail.Fulfillment.PayloadLineCount)
	}
	resultLines := strings.Split(detail.Fulfillment.Payload, "\n")
	if len(resultLines) != fulfillmentdomain.PayloadMaxPreviewLines {
		t.Errorf("expected truncated to %d lines, got %d", fulfillmentdomain.PayloadMaxPreviewLines, len(resultLines))
	}
}

func TestNewOrderDetailNilSafety(t *testing.T) {
	order := &orderdomain.Order{
		ID:      1,
		OrderNo: "ORD-NIL",
		Status:  "pending",
	}
	detail := NewOrderDetail(order)
	if detail.Fulfillment != nil {
		t.Error("expected nil fulfillment")
	}
	if detail.Items != nil {
		t.Error("expected nil items")
	}
}

func TestDeliverySecretsOnlyAppearInExplicitResponse(t *testing.T) {
	now := time.Now()
	child := orderdomain.Order{OrderNo: "FIXTURE-CHILD", PaidAt: &now,
		Items:       []orderdomain.OrderItem{{InstructionsJSON: jsonmap.JSON{"zh-CN": "PRIVATE-INSTRUCTIONS"}}},
		Fulfillment: &fulfillmentdomain.Fulfillment{Status: "delivered", Payload: "TEST-NOT-REDEEMABLE", LogisticsJSON: jsonmap.JSON{"note": "PRIVATE-STRUCTURED-DATA"}},
	}
	parent := child
	parent.OrderNo = "FIXTURE-PARENT"
	parent.Children = []orderdomain.Order{child}
	for name, response := range map[string]any{
		"customer list":             NewOrderSummary(&parent),
		"customer detail":           NewOrderDetail(&parent),
		"customer truncated detail": NewOrderDetailTruncated(&parent),
		"admin metadata":            AdminOrderMetadata(parent),
	} {
		t.Run(name, func(t *testing.T) {
			body, err := json.Marshal(response)
			if err != nil {
				t.Fatal(err)
			}
			for _, secret := range []string{"TEST-NOT-REDEEMABLE", "PRIVATE-STRUCTURED-DATA", "PRIVATE-INSTRUCTIONS"} {
				if strings.Contains(string(body), secret) {
					t.Fatalf("%s leaked in ordinary response", secret)
				}
			}
		})
	}
	for _, order := range []*orderdomain.Order{&parent, &parent.Children[0]} {
		delivery := NewOrderDelivery(order)
		if delivery == nil || delivery.Fulfillment.Payload != "TEST-NOT-REDEEMABLE" || delivery.Items[0].Instructions["zh-CN"] != "PRIVATE-INSTRUCTIONS" || delivery.Fulfillment.DeliveryData["note"] != "PRIVATE-STRUCTURED-DATA" {
			t.Fatal("explicit response lost delivery, or metadata redaction mutated source")
		}
	}
}

func TestDeliveryUnavailableBeforePaymentAndDelivery(t *testing.T) {
	now := time.Now()
	for _, order := range []*orderdomain.Order{
		nil, {},
		{Fulfillment: &fulfillmentdomain.Fulfillment{Status: "delivered", Payload: "TEST-UNPAID"}},
		{PaidAt: &now, Fulfillment: &fulfillmentdomain.Fulfillment{Status: "pending", Payload: "TEST-PENDING"}},
	} {
		if NewOrderDelivery(order) != nil {
			t.Fatal("unavailable delivery was exposed")
		}
	}
}
