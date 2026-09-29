package application

import (
	"testing"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"
	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	"github.com/dujiao-next/internal/constants"
)

func TestShouldUseCNYPaymentCurrency(t *testing.T) {
	if shouldUseCNYPaymentCurrency(nil) {
		t.Fatalf("nil channel should not force CNY")
	}
	if !shouldUseCNYPaymentCurrency(&paymentdomain.PaymentChannel{ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypeWechat}) {
		t.Fatalf("official wechat should force CNY")
	}
	if !shouldUseCNYPaymentCurrency(&paymentdomain.PaymentChannel{ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypeAlipay}) {
		t.Fatalf("official alipay should force CNY")
	}
	if shouldUseCNYPaymentCurrency(&paymentdomain.PaymentChannel{ProviderType: constants.PaymentProviderOfficial, ChannelType: constants.PaymentChannelTypePaypal}) {
		t.Fatalf("official paypal should not force CNY")
	}
}

func TestPickFirstNonEmpty(t *testing.T) {
	if got := pickFirstNonEmpty("", " ", "abc", "def"); got != "abc" {
		t.Fatalf("expected abc, got %s", got)
	}
	if got := pickFirstNonEmpty("", " "); got != "" {
		t.Fatalf("expected empty value, got %s", got)
	}
}

func TestShouldMarkFulfilling(t *testing.T) {
	if shouldMarkFulfilling(nil) {
		t.Fatalf("nil order should not be fulfilling")
	}
	order := &orderdomain.Order{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeAuto}}}
	if shouldMarkFulfilling(order) {
		t.Fatalf("auto items should not require fulfilling")
	}
	order = &orderdomain.Order{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeManual}}}
	if !shouldMarkFulfilling(order) {
		t.Fatalf("manual items should require fulfilling")
	}
}

func TestHasManualFulfillmentItems(t *testing.T) {
	if hasManualFulfillmentItems(nil) {
		t.Fatalf("nil order should not have manual items")
	}
	upstreamOnly := &orderdomain.Order{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeUpstream}}}
	if hasManualFulfillmentItems(upstreamOnly) {
		t.Fatalf("upstream-only order should not trigger manual fulfillment pending")
	}
	autoOnly := &orderdomain.Order{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeAuto}}}
	if hasManualFulfillmentItems(autoOnly) {
		t.Fatalf("auto-only order should not have manual items")
	}
	manualOnly := &orderdomain.Order{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeManual}}}
	if !hasManualFulfillmentItems(manualOnly) {
		t.Fatalf("manual order should have manual items")
	}
	emptyType := &orderdomain.Order{Items: []orderdomain.OrderItem{{FulfillmentType: "  "}}}
	if !hasManualFulfillmentItems(emptyType) {
		t.Fatalf("empty fulfillment type should be treated as manual")
	}
	mixedUpstreamManual := &orderdomain.Order{Items: []orderdomain.OrderItem{
		{FulfillmentType: constants.FulfillmentTypeUpstream},
		{FulfillmentType: constants.FulfillmentTypeManual},
	}}
	if !hasManualFulfillmentItems(mixedUpstreamManual) {
		t.Fatalf("mixed upstream+manual order should have manual items")
	}
}

func TestIsOrderFullyAutoFulfill(t *testing.T) {
	if isOrderFullyAutoFulfill(nil) {
		t.Fatalf("nil order should not be fully auto")
	}

	autoSingle := &orderdomain.Order{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeAuto}}}
	if !isOrderFullyAutoFulfill(autoSingle) {
		t.Fatalf("single auto order should be fully auto")
	}

	manualSingle := &orderdomain.Order{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeManual}}}
	if isOrderFullyAutoFulfill(manualSingle) {
		t.Fatalf("single manual order should not be fully auto")
	}

	mixedSingle := &orderdomain.Order{Items: []orderdomain.OrderItem{
		{FulfillmentType: constants.FulfillmentTypeAuto},
		{FulfillmentType: constants.FulfillmentTypeManual},
	}}
	if isOrderFullyAutoFulfill(mixedSingle) {
		t.Fatalf("mixed single order should not be fully auto")
	}

	parentAllAuto := &orderdomain.Order{Children: []orderdomain.Order{
		{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeAuto}}},
		{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeAuto}}},
	}}
	if !isOrderFullyAutoFulfill(parentAllAuto) {
		t.Fatalf("parent with all auto children should be fully auto")
	}

	parentMixed := &orderdomain.Order{Children: []orderdomain.Order{
		{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeAuto}}},
		{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeManual}}},
	}}
	if isOrderFullyAutoFulfill(parentMixed) {
		t.Fatalf("parent with mixed children should not be fully auto")
	}

	parentAllManual := &orderdomain.Order{Children: []orderdomain.Order{
		{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeUpstream}}},
		{Items: []orderdomain.OrderItem{{FulfillmentType: constants.FulfillmentTypeManual}}},
	}}
	if isOrderFullyAutoFulfill(parentAllManual) {
		t.Fatalf("parent with all manual/upstream children should not be fully auto")
	}

	emptyOrder := &orderdomain.Order{}
	if isOrderFullyAutoFulfill(emptyOrder) {
		t.Fatalf("order without items or children should not be fully auto")
	}
}
