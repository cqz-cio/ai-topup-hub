package alipayadapter

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/alipay"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

func TestAlipayAdapter_Type(t *testing.T) {
	a := NewAlipayAdapter()
	want := constants.PaymentProviderOfficial + ":" + constants.PaymentChannelTypeAlipay
	if got := a.Type(); got != want {
		t.Fatalf("Type() = %q, want %q", got, want)
	}
}

func TestAlipayAdapter_ValidateConfig_EmptyRejected(t *testing.T) {
	a := NewAlipayAdapter()
	err := a.ValidateConfig(jsonmap.JSON{}, "")
	if err == nil {
		t.Fatalf("expected error from empty config")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

func TestAlipayAdapter_CreatePayment_ConfigInvalidMapped(t *testing.T) {
	a := NewAlipayAdapter()
	_, err := a.CreatePayment(context.Background(), jsonmap.JSON{}, paymentcontract.GatewayCreateInput{
		OrderNo:  "ORDER_1",
		Currency: "CNY",
	})
	if err == nil {
		t.Fatalf("expected error from empty config")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

// TestAlipayAdapter_ValidateConfig_ValidConfig_C3Regression 守护 C3 regression fix:
// ValidateConfig 不再将 interactionMode 参数丢弃传空字符串，而是正确使用传入的 interactionMode。
// service 层 official provider 分支传入 channel.InteractionMode；为空时用 QR 作 default。
// valid alipay config + 合法 interactionMode 必须通过校验，不能因 interaction_mode 为空/无效而被拒绝。
func TestAlipayAdapter_ValidateConfig_ValidConfig_C3Regression(t *testing.T) {
	a := NewAlipayAdapter()
	// 使用 alipay native test 中确认有效的最小配置（QR 模式不要求 return_url）
	raw := jsonmap.JSON{
		"app_id":            "2026000000000000",
		"private_key":       "k",
		"alipay_public_key": "p",
		"gateway_url":       "https://openapi.alipay.com/gateway.do",
		"notify_url":        "https://example.com/api/v1/payments/callback",
		"sign_type":         "rsa2",
	}
	// 传 interactionMode=qr（service 层会从 channel.InteractionMode 取值传入）
	err := a.ValidateConfig(raw, constants.PaymentInteractionQR)
	if err != nil {
		t.Fatalf("ValidateConfig() should pass valid alipay config with QR mode, got: %v", err)
	}
}

// TestAlipayAdapter_ValidateConfig_EmptyInteractionModeUsesDefault 验证修复后
// 外部传空 interactionMode 不再让 ValidateConfig 永远失败（C3 修复前的 bug 路径）。
// 原因：wrapper 内部当 interactionMode="" 时用 QR 作 default。
func TestAlipayAdapter_ValidateConfig_EmptyInteractionModeUsesDefault(t *testing.T) {
	a := NewAlipayAdapter()
	raw := jsonmap.JSON{
		"app_id":            "2026000000000000",
		"private_key":       "k",
		"alipay_public_key": "p",
		"gateway_url":       "https://openapi.alipay.com/gateway.do",
		"notify_url":        "https://example.com/api/v1/payments/callback",
		"sign_type":         "rsa2",
	}
	// 第二参数传空字符串：
	// C3 修复前，这会把 "" 传给 alipay.ValidateConfig 导致 paymentcontract.ErrGatewayConfigInvalid；
	// C3 修复后，wrapper 用 QR 作 default，valid config 应当通过。
	err := a.ValidateConfig(raw, "")
	if err != nil {
		t.Fatalf("ValidateConfig() should use QR default when interactionMode is empty, got: %v", err)
	}
}

// TestAlipayAdapter_CreatePayment_ExchangeRate_AuditFields 守护 P1.2c audit
// 字段写入回归。模式见 stripe_adapter_test.go 同名测试。
func TestAlipayAdapter_CreatePayment_ExchangeRate_AuditFields(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟 alipay precreate(QR mode)成功响应
		_ = json.NewEncoder(w).Encode(map[string]any{
			"alipay_trade_precreate_response": map[string]any{
				"code":         "10000",
				"msg":          "Success",
				"out_trade_no": "ORDER-ALIPAY-AUDIT",
				"trade_no":     "20260516000001",
				"qr_code":      "https://qr.alipay.com/audit-001",
			},
			"sign": "test-sign",
		})
	}))
	defer server.Close()

	privateKeyPEM, publicKeyPEM := buildAlipayTestKeyPair(t)

	a := NewAlipayAdapter()
	raw := jsonmap.JSON{
		"app_id":            "2026000000000000",
		"private_key":       privateKeyPEM,
		"alipay_public_key": publicKeyPEM,
		"gateway_url":       server.URL,
		"notify_url":        "https://example.com/api/v1/payments/callback",
		"sign_type":         "RSA2",
		// 跨币种:10 USD → 72 CNY (rate 7.2)
		"target_currency": "CNY",
		"exchange_rate":   "7.2",
	}

	input := paymentcontract.GatewayCreateInput{
		OrderNo:   "ORDER-ALIPAY-USD-10",
		Subject:   "audit field test",
		Currency:  "USD",
		Amount:    money.FromDecimal(decimal.NewFromInt(10)),
		Extra:     jsonmap.JSON{"interaction_mode": constants.PaymentInteractionQR},
		NotifyURL: "https://example.com/api/v1/payments/callback",
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

// buildAlipayTestKeyPair 为 wrapper 测试生成临时 RSA2 密钥对。
// 等价于 alipay/alipay_test.go 的 buildTestConfig 中的密钥生成代码。
func buildAlipayTestKeyPair(t *testing.T) (privateKeyPEM, publicKeyPEM string) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	privateKeyDER, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal private key: %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal public key: %v", err)
	}
	privateKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privateKeyDER}))
	publicKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: publicKeyDER}))
	return
}

func TestAlipayAdapter_MapAlipayError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"config", alipay.ErrConfigInvalid, paymentcontract.ErrGatewayConfigInvalid},
		{"sign_generate→config", alipay.ErrSignGenerate, paymentcontract.ErrGatewayConfigInvalid},
		{"request", alipay.ErrRequestFailed, paymentcontract.ErrGatewayRequestFailed},
		{"response", alipay.ErrResponseInvalid, paymentcontract.ErrGatewayResponseInvalid},
		{"signature", alipay.ErrSignatureInvalid, paymentcontract.ErrGatewaySignatureInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapAlipayError(tc.in)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapAlipayError(%v) errors.Is %v = false, want true", tc.in, tc.want)
			}
		})
	}
}
