package dujiaopayadapter

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/shopspring/decimal"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
)

func TestDujiaoPayAdapter_Type(t *testing.T) {
	a := NewDujiaoPayAdapter()
	want := constants.PaymentProviderDujiaoPay + ":"
	if got := a.Type(); got != want {
		t.Fatalf("Type() = %q, want %q", got, want)
	}
}

func TestDujiaoPayAdapter_ValidateConfig_UnsupportedToken(t *testing.T) {
	a := NewDujiaoPayAdapter()
	base := jsonmap.JSON{
		"api_base_url":   "https://api.example.com",
		"api_key_id":     "key-1",
		"api_secret":     "secret-1",
		"webhook_secret": "whsec-1",
		"fiat_currency":  "USD",
	}

	// 未收录但符合 "<chain>-<token>" 约定的 token_id 允许配置，DujiaoPay 新增链时无需改代码。
	if err := a.ValidateConfig(base, "doge-usdt"); err != nil {
		t.Fatalf("unlisted but resolvable token_id should pass: %v", err)
	}

	// 无法推导出 chain 的 token_id 仍然拒绝，避免发出 chain 为空的建单请求。
	err := a.ValidateConfig(base, "dogeusdt")
	if err == nil {
		t.Fatalf("expected unsupported token error")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayUnsupportedChannel) {
		t.Fatalf("expected paymentcontract.ErrGatewayUnsupportedChannel, got %v", err)
	}

	// 逃生舱：命名不符合约定时，显式配置 chain 后仍可使用。
	withExplicitChain := jsonmap.JSON{}
	for k, v := range base {
		withExplicitChain[k] = v
	}
	withExplicitChain["chain"] = "doge"
	if err := a.ValidateConfig(withExplicitChain, "dogeusdt"); err != nil {
		t.Fatalf("explicit chain should allow an unconventional token_id: %v", err)
	}
}

func TestDujiaoPayAdapter_ValidateConfig_CashierMode(t *testing.T) {
	a := NewDujiaoPayAdapter()
	base := jsonmap.JSON{
		"api_base_url":    "https://api.example.com",
		"api_key_id":      "key-1",
		"api_secret":      "secret-1",
		"webhook_secret":  "whsec-1",
		"fiat_currency":   "USD",
		"order_mode":      "cashier",
		"allowed_methods": "tron-usdt,base-usdc",
	}

	if err := a.ValidateConfig(base, "dujiaopay"); err != nil {
		t.Fatalf("cashier channel_type dujiaopay should pass: %v", err)
	}
	if err := a.ValidateConfig(base, "tron-usdt"); !errors.Is(err, paymentcontract.ErrGatewayUnsupportedChannel) {
		t.Fatalf("cashier with token channel_type should fail, got %v", err)
	}

	transaction := jsonmap.JSON{
		"api_base_url":   "https://api.example.com",
		"api_key_id":     "key-1",
		"api_secret":     "secret-1",
		"webhook_secret": "whsec-1",
		"fiat_currency":  "USD",
	}
	if err := a.ValidateConfig(transaction, "dujiaopay"); err == nil {
		t.Fatalf("transaction with channel_type dujiaopay should fail")
	}
	if err := a.ValidateConfig(transaction, "tron-usdt"); err != nil {
		t.Fatalf("transaction with token channel_type should pass: %v", err)
	}
}

func TestDujiaoPayAdapter_CreatePaymentCashierRejectsQRMode(t *testing.T) {
	a := NewDujiaoPayAdapter()
	_, err := a.CreatePayment(context.Background(), jsonmap.JSON{
		"api_base_url":   "https://api.example.com",
		"api_key_id":     "key-1",
		"api_secret":     "secret-1",
		"webhook_secret": "whsec-1",
		"fiat_currency":  "USD",
		"order_mode":     "cashier",
	}, paymentcontract.GatewayCreateInput{
		OrderNo:     "PAY-2",
		Amount:      money.FromDecimal(decimal.RequireFromString("10")),
		Currency:    "USD",
		ChannelType: "dujiaopay",
		Extra:       jsonmap.JSON{"interaction_mode": constants.PaymentInteractionQR},
	})
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("cashier + qr should fail with paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

func TestDujiaoPayAdapter_CreatePaymentCashierRedirect(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order_id":"do_2","status":"awaiting_payment","selection_deadline":"2026-06-11T00:15:00Z","checkout_token":"ct_2","checkout_url":"https://pay.example.com/c/ct_2"}`))
	}))
	defer server.Close()

	a := NewDujiaoPayAdapter()
	result, err := a.CreatePayment(context.Background(), jsonmap.JSON{
		"api_base_url":    server.URL,
		"api_key_id":      "key-1",
		"api_secret":      "secret-1",
		"webhook_secret":  "whsec-1",
		"fiat_currency":   "USD",
		"order_mode":      "cashier",
		"allowed_methods": "tron-usdt,base-usdc",
	}, paymentcontract.GatewayCreateInput{
		OrderNo:     "PAY-2",
		Amount:      money.FromDecimal(decimal.RequireFromString("10")),
		Currency:    "USD",
		ChannelType: "dujiaopay",
		Extra:       jsonmap.JSON{"interaction_mode": constants.PaymentInteractionRedirect},
	})
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if result.ProviderRef != "do_2" {
		t.Fatalf("ProviderRef = %q, want do_2", result.ProviderRef)
	}
	if result.RedirectURL != "https://pay.example.com/c/ct_2" {
		t.Fatalf("RedirectURL = %q", result.RedirectURL)
	}
	if result.Payload["checkout_url"] != "https://pay.example.com/c/ct_2" {
		t.Fatalf("payload checkout_url = %v", result.Payload["checkout_url"])
	}
}

func TestDujiaoPayAdapter_CreatePaymentQRCodeModeUsesWalletAddress(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order_id":"do_1","chain":"tron","token_id":"tron-usdt","checkout_url":"https://pay.example.com/c/ct_1","pay_address":"TAddr","payable_amount":"10.0001","status":"pending"}`))
	}))
	defer server.Close()

	a := NewDujiaoPayAdapter()
	result, err := a.CreatePayment(context.Background(), jsonmap.JSON{
		"api_base_url":   server.URL,
		"api_key_id":     "key-1",
		"api_secret":     "secret-1",
		"webhook_secret": "whsec-1",
		"fiat_currency":  "USD",
	}, paymentcontract.GatewayCreateInput{
		OrderNo:        "PAY-1",
		Amount:         money.FromDecimal(decimal.RequireFromString("10")),
		Currency:       "USD",
		ChannelType:    "tron-usdt",
		ReturnURLQuery: map[string]string{"biz_type": "order", "order_no": "ORDER-1"},
		Extra:          jsonmap.JSON{"interaction_mode": constants.PaymentInteractionQR},
	})
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if result.ProviderRef != "do_1" {
		t.Fatalf("ProviderRef = %q, want do_1", result.ProviderRef)
	}
	if result.RedirectURL != "https://pay.example.com/c/ct_1" {
		t.Fatalf("RedirectURL = %q", result.RedirectURL)
	}
	if result.QRCodeURL != "TAddr" {
		t.Fatalf("QRCodeURL = %q", result.QRCodeURL)
	}
	if result.Payload["pay_address"] != "TAddr" {
		t.Fatalf("payload pay_address = %v", result.Payload["pay_address"])
	}
	if result.Payload["chain"] != "tron" {
		t.Fatalf("payload chain = %v", result.Payload["chain"])
	}
	if result.Payload["token_id"] != "tron-usdt" {
		t.Fatalf("payload token_id = %v", result.Payload["token_id"])
	}
	if result.AmountSent != "10" || result.CurrencySent != "USD" {
		t.Fatalf("sent payment facts = %s %s, want 10 USD", result.AmountSent, result.CurrencySent)
	}
	if got := result.Payload[paymentcontract.GatewayPayloadFiatCurrencySent]; got != "USD" {
		t.Fatalf("gateway fiat currency snapshot = %#v, want USD", got)
	}
}

func TestDujiaoPayAdapter_ParseWebhookMapsFiatFactsAndTransactionID(t *testing.T) {
	body := []byte(`{"event_id":"evt_paid","event_type":"order.paid","event_version":"v1","created_at":"2026-06-06T12:00:00Z","data":{"order_id":"do_paid","merchant_order_id":"PAY-PAID","fiat_currency":"usd","fiat_amount":"20.00","payable_amount":"20.145","tx_id":"0xpaid","paid_at":"2026-06-06T12:00:01Z"}}`)
	mac := hmac.New(sha256.New, []byte("whsec-1"))
	mac.Write([]byte("1750000000."))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	a := NewDujiaoPayAdapter()
	webhooker, ok := a.(paymentcontract.GatewayWebhooker)
	if !ok {
		t.Fatalf("DujiaoPay adapter must implement GatewayWebhooker")
	}
	result, err := webhooker.ParseWebhook(context.Background(), jsonmap.JSON{
		"webhook_secret": "whsec-1",
	}, map[string]string{
		"DJP-Webhook-Timestamp": "1750000000",
		"DJP-Webhook-Signature": signature,
	}, body, time.Unix(1750000010, 0))
	if err != nil {
		t.Fatalf("ParseWebhook failed: %v", err)
	}
	if result.Amount.Decimal.Cmp(decimal.RequireFromString("20.00")) != 0 {
		t.Fatalf("Amount = %s, want 20.00", result.Amount.String())
	}
	if result.Currency != "USD" {
		t.Fatalf("Currency = %q, want USD", result.Currency)
	}
	if result.Payload["tx_id"] != "0xpaid" {
		t.Fatalf("payload tx_id = %v, want 0xpaid", result.Payload["tx_id"])
	}
}
