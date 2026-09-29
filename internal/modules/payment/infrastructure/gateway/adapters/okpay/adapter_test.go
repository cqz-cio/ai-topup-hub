package okpayadapter

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/okpay"

	"github.com/shopspring/decimal"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

func TestOkpayAdapter_Type(t *testing.T) {
	a := NewOkpayAdapter()
	want := constants.PaymentProviderOkpay + ":"
	if got := a.Type(); got != want {
		t.Fatalf("Type() = %q, want %q", got, want)
	}
}

func TestOkpayAdapter_ValidateConfig_UnsupportedChannel(t *testing.T) {
	a := NewOkpayAdapter()
	err := a.ValidateConfig(jsonmap.JSON{}, "no-such-channel-type")
	if err == nil {
		t.Fatalf("expected error for unsupported channel")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayUnsupportedChannel) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayUnsupportedChannel, got %v", err)
	}
}

func TestOkpayAdapter_CreatePayment_ConfigInvalidMapped(t *testing.T) {
	a := NewOkpayAdapter()
	// 用 okpay 真实支持的 channelType("usdt" / "trx")
	_, err := a.CreatePayment(context.Background(), jsonmap.JSON{}, paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER_1",
		Currency:    "USDT",
		ChannelType: "usdt",
	})
	if err == nil {
		t.Fatalf("expected error from empty config")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

// TestOkpayAdapter_CreatePayment_ExchangeRate_C4Regression 守护 C4 regression fix:
// 当 okpay config 配有 exchange_rate（如 "7" CNY->USDT）时，
// CreatePayment 返回的 AmountSent/CurrencySent 必须反映转换后的数值，
// 使 service 层更新 payment.Amount/Currency 后与 callback 的 USDT amount 对齐。
func TestOkpayAdapter_CreatePayment_ExchangeRate_C4Regression(t *testing.T) {
	// mock okpay API：返回成功响应
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","code":200,"data":{"order_id":"OK-ORDER-C4","pay_url":"https://pay.example.com/c4"}}`))
	}))
	defer server.Close()

	a := NewOkpayAdapter()
	raw := jsonmap.JSON{
		"gateway_url":    server.URL,
		"merchant_id":    "shop-c4",
		"merchant_token": "token-c4",
		"return_url":     "https://shop.example.com/pay",
		"callback_url":   "https://api.example.com/api/v1/payments/callback/okpay",
		"exchange_rate":  "7.0", // 1 CNY = 7.0 USDT（测试用夸张汇率，验证换算逻辑）
		"coin":           "USDT",
	}

	// 原始金额 88 CNY，转换后应为 88 * 7.0 = 616 USDT（符合 spec 中的示例）
	originalAmountDec := decimal.NewFromFloat(88)
	input := paymentcontract.GatewayCreateInput{
		OrderNo:     "CNY-ORDER-88",
		Subject:     "test order",
		ChannelType: "usdt",
		Currency:    "CNY",
		Amount:      money.FromDecimal(originalAmountDec),
		ReturnURL:   "https://shop.example.com/pay",
	}

	result, err := a.CreatePayment(context.Background(), raw, input)
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	// C4 修复前：AmountSent="" / CurrencySent=""（没有填）
	// C4 修复后：AmountSent="616.00000000", CurrencySent="USDT"
	if result.AmountSent == "" {
		t.Fatal("CreatePayment() should fill AmountSent for cross-currency okpay channel (C4 fix)")
	}
	if result.CurrencySent == "" {
		t.Fatal("CreatePayment() should fill CurrencySent for cross-currency okpay channel (C4 fix)")
	}
	if result.CurrencySent != "USDT" {
		t.Fatalf("CurrencySent should be USDT, got %s", result.CurrencySent)
	}

	// 验证转换金额正确：88 * 7.0 = 616
	expectedAmount := decimal.NewFromFloat(88).Mul(decimal.NewFromFloat(7.0)).StringFixed(8)
	if result.AmountSent != expectedAmount {
		t.Fatalf("AmountSent should be %s, got %s", expectedAmount, result.AmountSent)
	}

	// 验证 Payload 里有 audit 字段
	if _, ok := result.Payload["exchange_rate"]; !ok {
		t.Fatal("Payload should contain exchange_rate audit field")
	}
	if _, ok := result.Payload["original_amount"]; !ok {
		t.Fatal("Payload should contain original_amount audit field")
	}
	if _, ok := result.Payload["original_currency"]; !ok {
		t.Fatal("Payload should contain original_currency audit field")
	}
}

// TestOkpayAdapter_CreatePayment_NoExchangeRate_AmountSentEqualsOriginal 验证无 exchange_rate 时
// AmountSent/CurrencySent 等于原始金额/币种（no-op conversion 路径）。
func TestOkpayAdapter_CreatePayment_NoExchangeRate_AmountSentEqualsOriginal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","code":200,"data":{"order_id":"OK-ORDER-NOFX","pay_url":"https://pay.example.com/nofx"}}`))
	}))
	defer server.Close()

	a := NewOkpayAdapter()
	raw := jsonmap.JSON{
		"gateway_url":    server.URL,
		"merchant_id":    "shop-nofx",
		"merchant_token": "token-nofx",
		"return_url":     "https://shop.example.com/pay",
		"callback_url":   "https://api.example.com/api/v1/payments/callback/okpay",
		"exchange_rate":  "1", // 无转换
		"coin":           "USDT",
	}

	input := paymentcontract.GatewayCreateInput{
		OrderNo:     "USDT-ORDER-1",
		Subject:     "test",
		ChannelType: "usdt",
		Currency:    "USDT",
		Amount:      money.FromDecimal(decimal.NewFromFloat(10)),
		ReturnURL:   "https://shop.example.com/pay",
	}

	result, err := a.CreatePayment(context.Background(), raw, input)
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	// exchange_rate="1" 时不做转换，AmountSent 应等于原始金额字符串
	if result.AmountSent != "10" {
		t.Fatalf("AmountSent should be original amount '10', got %s", result.AmountSent)
	}
	if result.CurrencySent != "USDT" {
		t.Fatalf("CurrencySent should be USDT, got %s", result.CurrencySent)
	}
	// 无 conversion 时不应有 audit 字段
	if _, ok := result.Payload["exchange_rate"]; ok {
		t.Fatal("Payload should NOT contain exchange_rate audit field when no conversion")
	}
}

// TestOkpayAdapter_CreatePayment_ExchangeRateOneUsesCoinCurrency reproduces the demo callback failure:
// the shop order is stored as USD, but okpay receives coin=USDT and callbacks with coin=USDT.
// Even when exchange_rate=1, the payment row must be updated to the gateway coin to avoid callback currency mismatch.
func TestOkpayAdapter_CreatePayment_ExchangeRateOneUsesCoinCurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","code":200,"data":{"order_id":"OK-ORDER-USD-USDT","pay_url":"https://pay.example.com/usd-usdt"}}`))
	}))
	defer server.Close()

	a := NewOkpayAdapter()
	raw := jsonmap.JSON{
		"gateway_url":    server.URL,
		"merchant_id":    "shop-usd-usdt",
		"merchant_token": "token-usd-usdt",
		"return_url":     "https://shop.example.com/pay",
		"callback_url":   "https://api.example.com/api/v1/payments/callback/okpay",
		"exchange_rate":  "1",
		"coin":           "USDT",
	}

	input := paymentcontract.GatewayCreateInput{
		OrderNo:     "USD-ORDER-1",
		Subject:     "test",
		ChannelType: "usdt",
		Currency:    "USD",
		Amount:      money.FromDecimal(decimal.NewFromFloat(1)),
		ReturnURL:   "https://shop.example.com/pay",
	}

	result, err := a.CreatePayment(context.Background(), raw, input)
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	if result.AmountSent != "1" {
		t.Fatalf("AmountSent should remain original amount '1', got %s", result.AmountSent)
	}
	if result.CurrencySent != "USDT" {
		t.Fatalf("CurrencySent should use okpay coin USDT, got %s", result.CurrencySent)
	}
	if _, ok := result.Payload["exchange_rate"]; ok {
		t.Fatal("Payload should NOT contain exchange_rate audit field when no amount conversion")
	}
}

func TestOkpayAdapter_CreatePayment_ExchangeRateOneResolvesTRXChannelCurrency(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","code":200,"data":{"order_id":"OK-ORDER-USD-TRX","pay_url":"https://pay.example.com/usd-trx"}}`))
	}))
	defer server.Close()

	a := NewOkpayAdapter()
	raw := jsonmap.JSON{
		"gateway_url":    server.URL,
		"merchant_id":    "shop-usd-trx",
		"merchant_token": "token-usd-trx",
		"return_url":     "https://shop.example.com/pay",
		"callback_url":   "https://api.example.com/api/v1/payments/callback/okpay",
		"exchange_rate":  "1",
	}

	input := paymentcontract.GatewayCreateInput{
		OrderNo:     "USD-ORDER-TRX-1",
		Subject:     "test",
		ChannelType: constants.PaymentChannelTypeTrx,
		Currency:    "USD",
		Amount:      money.FromDecimal(decimal.NewFromFloat(1)),
		ReturnURL:   "https://shop.example.com/pay",
	}

	result, err := a.CreatePayment(context.Background(), raw, input)
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	if result.AmountSent != "1" {
		t.Fatalf("AmountSent should remain original amount '1', got %s", result.AmountSent)
	}
	if result.CurrencySent != "TRX" {
		t.Fatalf("CurrencySent should use resolved okpay coin TRX, got %s", result.CurrencySent)
	}
	if _, ok := result.Payload["exchange_rate"]; ok {
		t.Fatal("Payload should NOT contain exchange_rate audit field when no amount conversion")
	}
}

func TestOkpayAdapter_MapOkpayError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"config", okpay.ErrConfigInvalid, paymentcontract.ErrGatewayConfigInvalid},
		{"request", okpay.ErrRequestFailed, paymentcontract.ErrGatewayRequestFailed},
		{"response", okpay.ErrResponseInvalid, paymentcontract.ErrGatewayResponseInvalid},
		{"signature", okpay.ErrSignatureInvalid, paymentcontract.ErrGatewaySignatureInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapOkpayError(tc.in)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapOkpayError(%v) errors.Is %v = false, want true", tc.in, tc.want)
			}
		})
	}
}
