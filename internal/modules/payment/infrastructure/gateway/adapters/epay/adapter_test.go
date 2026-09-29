package epayadapter

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/epay"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestEpayAdapter_Type(t *testing.T) {
	a := NewEpayAdapter()
	want := constants.PaymentProviderEpay + ":"
	if got := a.Type(); got != want {
		t.Fatalf("Type() = %q, want %q", got, want)
	}
}

func TestEpayAdapter_ValidateConfig_UnsupportedChannel(t *testing.T) {
	a := NewEpayAdapter()
	err := a.ValidateConfig(jsonmap.JSON{}, "no-such-channel-type")
	if err == nil {
		t.Fatalf("expected error for unsupported channel")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayUnsupportedChannel) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayUnsupportedChannel, got %v", err)
	}
}

func TestEpayAdapter_CreatePayment_ConfigInvalidMapped(t *testing.T) {
	a := NewEpayAdapter()
	// 传一个 epay.IsSupportedChannelType 接受的 channelType(让校验过),
	// 但 config 空导致 ParseConfig/ValidateConfig 失败
	_, err := a.CreatePayment(context.Background(), jsonmap.JSON{}, paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER_1",
		Currency:    "CNY",
		ChannelType: "alipay", // epay 支持 alipay/wxpay/qqpay
	})
	if err == nil {
		t.Fatalf("expected error from empty config")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

// TestEpayAdapter_CreatePayment_ExchangeRate_AuditFields 守护 P1.2c audit
// 字段写入回归。模式见 stripe_adapter_test.go 同名测试。
func TestEpayAdapter_CreatePayment_ExchangeRate_AuditFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// epay native 期望 JSON-encoded-string 响应,这里直接返回原始 JSON 即可
		// (epay_create_test.go 用 double-encoded 测了 string→json 的退化路径,
		// 这里仅需走标准 path)
		_, _ = w.Write([]byte(`{"code":1,"msg":"success","trade_no":"EPAY-AUDIT-001","payurl":"https://pay.example.com/audit"}`))
	}))
	defer server.Close()

	a := NewEpayAdapter()
	raw := jsonmap.JSON{
		"gateway_url":  server.URL,
		"epay_version": "v1",
		"merchant_id":  "M-AUDIT",
		"merchant_key": "K-AUDIT-001",
		"notify_url":   "https://api.example.com/api/v1/payments/callback/epay",
		"return_url":   "https://shop.example.com/pay",
		"sign_type":    "MD5",
		// 跨币种:10 USD → 72 CNY (rate 7.2)
		"target_currency": "CNY",
		"exchange_rate":   "7.2",
	}

	input := paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER-EPAY-USD-10",
		Subject:     "audit field test",
		Currency:    "USD",
		Amount:      money.FromDecimal(decimal.NewFromInt(10)),
		ChannelType: "alipay", // epay 支持 alipay/wxpay/qqpay
		ClientIP:    "127.0.0.1",
		Extra:       jsonmap.JSON{"interaction_mode": constants.PaymentInteractionQR},
	}

	result, err := a.CreatePayment(context.Background(), raw, input)
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	if result.CurrencySent != "CNY" {
		t.Fatalf("CurrencySent = %q, want CNY (converted target)", result.CurrencySent)
	}
	if result.AmountSent != "72" {
		t.Fatalf("AmountSent = %q, want 72 (10 USD * 7.2)", result.AmountSent)
	}

	if got := result.Payload["exchange_rate"]; got != "7.2" {
		t.Fatalf("Payload[exchange_rate] = %v, want 7.2", got)
	}
	if got := result.Payload["original_amount"]; got != "10" {
		t.Fatalf("Payload[original_amount] = %v, want 10", got)
	}
	if got := result.Payload["original_currency"]; got != "USD" {
		t.Fatalf("Payload[original_currency] = %v, want USD", got)
	}
}

func TestEpayAdapter_VerifyCallbackRejectsMerchantMismatch(t *testing.T) {
	a := NewEpayAdapter()
	raw := jsonmap.JSON{
		"gateway_url":  "https://epay.example.com",
		"epay_version": "v1",
		"merchant_id":  "1001",
		"merchant_key": "key-001",
		"notify_url":   "https://api.example.com/api/v1/payments/callback/epay",
		"return_url":   "https://shop.example.com/pay",
		"sign_type":    "MD5",
	}
	form := signEpayV1AdapterCallbackForm(map[string]string{
		"pid":          "1002",
		"out_trade_no": "ORDER-1001",
		"trade_no":     "EPAY-2001",
		"money":        "10.00",
		"trade_status": constants.EpayTradeStatusSuccess,
	}, "key-001")

	_, err := a.(paymentcontract.GatewayCallbackVerifier).VerifyCallback(raw, form, nil)
	if err == nil {
		t.Fatalf("expected callback ownership error")
	}
	if !errors.Is(err, paymentcontract.ErrGatewaySignatureInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewaySignatureInvalid, got %v", err)
	}
}

func TestEpayAdapter_MapEpayError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"config", epay.ErrConfigInvalid, paymentcontract.ErrGatewayConfigInvalid},
		{"channel_type→unsupported", epay.ErrChannelTypeNotOK, paymentcontract.ErrGatewayUnsupportedChannel},
		{"sign_generate→config", epay.ErrSignatureGenerate, paymentcontract.ErrGatewayConfigInvalid},
		{"sign_invalid→signature", epay.ErrSignatureInvalid, paymentcontract.ErrGatewaySignatureInvalid},
		{"request", epay.ErrRequestFailed, paymentcontract.ErrGatewayRequestFailed},
		{"response", epay.ErrResponseInvalid, paymentcontract.ErrGatewayResponseInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapEpayError(tc.in)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapEpayError(%v) errors.Is %v = false, want true", tc.in, tc.want)
			}
		})
	}
}

func signEpayV1AdapterCallbackForm(params map[string]string, merchantKey string) map[string][]string {
	form := make(map[string][]string, len(params)+2)
	for key, value := range params {
		form[key] = []string{value}
	}
	form["sign_type"] = []string{"MD5"}
	form["sign"] = []string{md5Hex(buildEpayAdapterSignContent(params) + merchantKey)}
	return form
}

func buildEpayAdapterSignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key, value := range params {
		if value == "" || key == "sign" || key == "sign_type" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	pairs := make([]string, 0, len(keys))
	for _, key := range keys {
		pairs = append(pairs, fmt.Sprintf("%s=%s", key, params[key]))
	}
	return strings.Join(pairs, "&")
}

func md5Hex(content string) string {
	sum := md5.Sum([]byte(content))
	return strings.ToLower(hex.EncodeToString(sum[:]))
}
