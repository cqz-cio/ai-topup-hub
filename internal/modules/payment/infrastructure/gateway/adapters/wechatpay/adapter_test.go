package wechatpayadapter

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/wechatpay"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/shopspring/decimal"
)

// buildTestRSAPrivateKeyPEM 生成 PKCS8 格式 RSA 2048 私钥 PEM，用于 adapter 单元测试。
// wechatpay.ParseConfig 会 parse private key，所以测试 fixture 必须是合法的 PEM。
func buildTestRSAPrivateKeyPEM(t *testing.T) string {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(privateKey)
	if err != nil {
		t.Fatalf("marshal PKCS8 key: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func TestWechatpayAdapter_Type(t *testing.T) {
	a := NewWechatpayAdapter()
	want := constants.PaymentProviderOfficial + ":" + constants.PaymentChannelTypeWechat
	if got := a.Type(); got != want {
		t.Fatalf("Type() = %q, want %q", got, want)
	}
}

func TestWechatpayAdapter_ValidateConfig_EmptyRejected(t *testing.T) {
	a := NewWechatpayAdapter()
	// 空 config 传给 ValidateConfig，wechatpay.ParseConfig 因缺少必填字段拒绝，返回 paymentcontract.ErrGatewayConfigInvalid
	raw := jsonmap.JSON{}
	err := a.ValidateConfig(raw, "redirect")
	if err == nil {
		t.Fatalf("expected error for empty config")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

func TestWechatpayAdapter_CreatePayment_ConfigInvalidMapped(t *testing.T) {
	a := NewWechatpayAdapter()
	raw := jsonmap.JSON{} // 空 config
	_, err := a.CreatePayment(context.Background(), raw, paymentcontract.GatewayCreateInput{
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

// buildMinimalWechatRaw 构造 wechatpay.ParseConfig 可通过的最小 config。
// wechatpay ParseConfig 会 parse merchant_private_key（RSA PEM），所以必须使用真实格式。
// api_v3_key 必须是 32 字节（wechat 规定）。
func buildMinimalWechatRaw(t *testing.T) jsonmap.JSON {
	t.Helper()
	raw, _ := buildMinimalWechatRawWithSigner(t)
	return raw
}

func buildMinimalWechatRawWithSigner(t *testing.T) (jsonmap.JSON, *rsa.PrivateKey) {
	t.Helper()
	wechatPayPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate wechatpay RSA key: %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&wechatPayPrivateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal wechatpay public key: %v", err)
	}
	return jsonmap.JSON{
		"appid":                   "wx1234567890abcdef",
		"mchid":                   "1234567890",
		"merchant_serial_no":      "ABCDEF1234567890",
		"merchant_private_key":    buildTestRSAPrivateKeyPEM(t),
		"api_v3_key":              "01234567890123456789012345678901", // 32 bytes
		"verification_mode":       "wechatpay_public_key",
		"wechatpay_public_key_id": "PUB_KEY_ID_3000000001",
		"wechatpay_public_key": string(pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyDER,
		})),
		"notify_url": "https://example.com/api/v1/payments/webhook/wechat",
	}, wechatPayPrivateKey
}

func writeAdapterSignedWechatPayResponse(
	t *testing.T,
	w http.ResponseWriter,
	privateKey *rsa.PrivateKey,
	body string,
) {
	t.Helper()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "adapter-response"
	digest := sha256.Sum256([]byte(timestamp + "\n" + nonce + "\n" + body + "\n"))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign adapter response: %v", err)
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Wechatpay-Timestamp", timestamp)
	w.Header().Set("Wechatpay-Nonce", nonce)
	w.Header().Set("Wechatpay-Serial", "PUB_KEY_ID_3000000001")
	w.Header().Set("Wechatpay-Signature", base64.StdEncoding.EncodeToString(signature))
	_, _ = w.Write([]byte(body))
}

// TestWechatpayAdapter_ValidateConfig_ValidConfig_C1C2Regression 守护 C1+C2 regression fix:
// ValidateConfig 应当通过 valid config + 合法 interactionMode（QR）的校验，
// 证明 C1+C2 fix 中的 parseConfig 改动不影响 ValidateConfig 入口路径。
func TestWechatpayAdapter_ValidateConfig_ValidConfig_C1C2Regression(t *testing.T) {
	a := NewWechatpayAdapter()
	raw := buildMinimalWechatRaw(t)
	// 传合法 interactionMode（service 层 channel.InteractionMode 的值）
	err := a.ValidateConfig(raw, constants.PaymentInteractionQR)
	if err != nil {
		t.Fatalf("ValidateConfig() should pass valid wechat config with QR mode, got: %v", err)
	}
}

// TestWechatpayAdapter_QueryPayment_NotBlockedByInteractionMode 守护 C1 regression fix:
// parseConfig("") 在修复前会被 wechatpay.ValidateConfig 拒绝 interaction_mode 为空。
// 修复后跳过 ValidateConfig，直接 ParseConfig，
// QueryPayment 应走到 HTTP 失败（paymentcontract.ErrGatewayRequestFailed），而非 paymentcontract.ErrGatewayConfigInvalid。
func TestWechatpayAdapter_QueryPayment_NotBlockedByInteractionMode(t *testing.T) {
	a := NewWechatpayAdapter()
	c, ok := a.(paymentcontract.GatewayCapturer)
	if !ok {
		t.Fatalf("wechatpayAdapter must implement paymentcontract.GatewayCapturer")
	}
	raw := buildMinimalWechatRaw(t)
	_, err := c.QueryPayment(context.Background(), raw, "ORDER_1")
	if err == nil {
		t.Fatal("expected error (no real wechat endpoint), got nil")
	}
	// C1 修复前：errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) && contains "interaction_mode"
	// C1 修复后：应当是 paymentcontract.ErrGatewayRequestFailed 或其他网络错误，不应是 paymentcontract.ErrGatewayConfigInvalid
	if errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("QueryPayment should NOT fail with paymentcontract.ErrGatewayConfigInvalid after C1 fix, got: %v", err)
	}
}

// TestWechatpayAdapter_ParseWebhook_NotBlockedByInteractionMode 守护 C2 regression fix:
// ParseWebhook 不再被空 interactionMode 的 ValidateConfig 阻断。
// 传入无效 body 应走到签名/解析错误，而非 paymentcontract.ErrGatewayConfigInvalid。
func TestWechatpayAdapter_ParseWebhook_NotBlockedByInteractionMode(t *testing.T) {
	a := NewWechatpayAdapter()
	wh, ok := a.(paymentcontract.GatewayWebhooker)
	if !ok {
		t.Fatalf("wechatpayAdapter must implement paymentcontract.GatewayWebhooker")
	}
	raw := buildMinimalWechatRaw(t)
	_, err := wh.ParseWebhook(context.Background(), raw, map[string]string{}, []byte("{}"), time.Now())
	if err == nil {
		t.Fatal("expected error (invalid webhook body), got nil")
	}
	// C2 修复前：errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) — 被 parseConfig 拦截
	// C2 修复后：应当是 paymentcontract.ErrGatewaySignatureInvalid 或 paymentcontract.ErrGatewayResponseInvalid，不应是 paymentcontract.ErrGatewayConfigInvalid
	if errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("ParseWebhook should NOT fail with paymentcontract.ErrGatewayConfigInvalid after C2 fix, got: %v", err)
	}
}

// TestWechatpayAdapter_CreatePayment_ExchangeRate_AuditFields 守护 P1.2c audit
// 字段写入回归。模式见 stripe_adapter_test.go 同名测试。
// 用 QR 模式触发 /v3/pay/transactions/native;httptest 返回 code_url 让
// wechatpay native 解析成功,进入 wrapper 的 audit 写入块。
func TestWechatpayAdapter_CreatePayment_ExchangeRate_AuditFields(t *testing.T) {
	raw, wechatPayPrivateKey := buildMinimalWechatRawWithSigner(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v3/pay/transactions/native" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		writeAdapterSignedWechatPayResponse(
			t,
			w,
			wechatPayPrivateKey,
			`{"code_url":"weixin://wxpay/bizpayurl?pr=audit001","prepay_id":"wx-audit-001"}`,
		)
	}))
	defer server.Close()

	raw["base_url"] = server.URL
	// 跨币种:10 USD → 72 CNY (rate 7.2)
	raw["target_currency"] = "CNY"
	raw["exchange_rate"] = "7.2"

	a := NewWechatpayAdapter()
	input := paymentcontract.GatewayCreateInput{
		OrderNo:   "ORDER-WX-USD-10",
		Subject:   "audit field test",
		Currency:  "USD",
		Amount:    money.FromDecimal(decimal.NewFromInt(10)),
		ClientIP:  "127.0.0.1",
		Extra:     jsonmap.JSON{"interaction_mode": constants.PaymentInteractionQR},
		NotifyURL: "https://example.com/api/v1/payments/webhook/wechat",
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

func TestWechatpayAdapter_MapWechatpayError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"config", wechatpay.ErrConfigInvalid, paymentcontract.ErrGatewayConfigInvalid},
		{"request", wechatpay.ErrRequestFailed, paymentcontract.ErrGatewayRequestFailed},
		{"response", wechatpay.ErrResponseInvalid, paymentcontract.ErrGatewayResponseInvalid},
		{"signature", wechatpay.ErrSignatureInvalid, paymentcontract.ErrGatewaySignatureInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapWechatpayError(tc.in)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapWechatpayError(%v) errors.Is %v = false, want true", tc.in, tc.want)
			}
		})
	}
}
