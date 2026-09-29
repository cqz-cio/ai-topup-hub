package wechatpay

import (
	"context"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"

	"github.com/wechatpay-apiv3/wechatpay-go/core"
)

type testWechatPayKey struct {
	id           string
	privateKey   *rsa.PrivateKey
	publicKeyPEM string
}

func newTestWechatPayKey(t *testing.T) testWechatPayKey {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate wechatpay key: %v", err)
	}
	publicKeyDER, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal wechatpay public key: %v", err)
	}
	return testWechatPayKey{
		id:         "PUB_KEY_ID_3000000001",
		privateKey: privateKey,
		publicKeyPEM: string(pem.EncodeToMemory(&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: publicKeyDER,
		})),
	}
}

func newTestPlatformCertificate(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate platform certificate key: %v", err)
	}
	now := time.Now()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(0x1234567890),
		NotBefore:    now.Add(-time.Hour),
		NotAfter:     now.Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatalf("create platform certificate: %v", err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("parse platform certificate: %v", err)
	}
	return privateKey, certificate
}

func signWechatPayMessage(t *testing.T, privateKey *rsa.PrivateKey, message string) string {
	t.Helper()
	digest := sha256.Sum256([]byte(message))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatalf("sign wechatpay message: %v", err)
	}
	return base64.StdEncoding.EncodeToString(signature)
}

func writeSignedWechatPayResponse(t *testing.T, w http.ResponseWriter, key testWechatPayKey, body string) {
	t.Helper()
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "response-nonce"
	signature := signWechatPayMessage(t, key.privateKey, timestamp+"\n"+nonce+"\n"+body+"\n")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Wechatpay-Timestamp", timestamp)
	w.Header().Set("Wechatpay-Nonce", nonce)
	w.Header().Set("Wechatpay-Serial", key.id)
	w.Header().Set("Wechatpay-Signature", signature)
	_, _ = w.Write([]byte(body))
}

func baseWechatPayConfig(t *testing.T) map[string]interface{} {
	t.Helper()
	return map[string]interface{}{
		"appid":                "wx1234567890",
		"mchid":                "1900000109",
		"merchant_serial_no":   "ABC123456789",
		"merchant_private_key": buildTestPrivateKey(),
		"api_v3_key":           "12345678901234567890123456789012",
		"notify_url":           "https://example.com/api/v1/payments/callback",
	}
}

func addPublicKeyMode(raw map[string]interface{}, key testWechatPayKey) {
	raw["verification_mode"] = verificationModeWechatPayPublicKey
	raw["wechatpay_public_key_id"] = key.id
	raw["wechatpay_public_key"] = key.publicKeyPEM
}

func TestParseConfigDefaultsToPlatformCertificateMode(t *testing.T) {
	cfg, err := ParseConfig(baseWechatPayConfig(t))
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if cfg.VerificationMode != verificationModePlatformCertificate {
		t.Fatalf("verification mode = %q, want %q", cfg.VerificationMode, verificationModePlatformCertificate)
	}
}

func TestValidateConfigWechatPayPublicKeyMode(t *testing.T) {
	key := newTestWechatPayKey(t)
	raw := baseWechatPayConfig(t)
	addPublicKeyMode(raw, key)
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	if err := ValidateConfig(cfg, constants.PaymentInteractionQR); err != nil {
		t.Fatalf("validate public key config: %v", err)
	}

	cfg.WechatPayPublicKeyID = ""
	if err := ValidateConfig(cfg, constants.PaymentInteractionQR); !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("missing public key id error = %v, want ErrConfigInvalid", err)
	}
}

func TestCreateAPIClientWithPlatformCertificateVerifier(t *testing.T) {
	platformPrivateKey, certificate := newTestPlatformCertificate(t)
	merchantPrivateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate merchant private key: %v", err)
	}
	cfg := &Config{
		MerchantID:       "1900000109",
		MerchantSerialNo: "ABC123456789",
		VerificationMode: verificationModePlatformCertificate,
	}
	visitor := core.NewCertificateMapWithList([]*x509.Certificate{certificate})
	verifier, err := createWechatPayVerifierWithVisitor(cfg, visitor)
	if err != nil {
		t.Fatalf("create platform verifier: %v", err)
	}
	client, err := createAPIClientWithVerifier(context.Background(), cfg, merchantPrivateKey, verifier)
	if err != nil {
		t.Fatalf("create API client: %v", err)
	}
	serial := fmt.Sprintf("%X", certificate.SerialNumber.Bytes())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Wechatpay-Serial"); got != serial {
			t.Errorf("Wechatpay-Serial = %q, want %q", got, serial)
		}
		body := `{"ok":true}`
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		nonce := "platform-response"
		w.Header().Set("Wechatpay-Timestamp", timestamp)
		w.Header().Set("Wechatpay-Nonce", nonce)
		w.Header().Set("Wechatpay-Serial", serial)
		w.Header().Set("Wechatpay-Signature", signWechatPayMessage(
			t,
			platformPrivateKey,
			timestamp+"\n"+nonce+"\n"+body+"\n",
		))
		_, _ = w.Write([]byte(body))
	}))
	defer server.Close()

	result, err := client.Get(context.Background(), server.URL)
	if err != nil {
		t.Fatalf("platform certificate response verification failed: %v", err)
	}
	_ = result.Response.Body.Close()
}

func TestCreatePaymentRejectsInvalidResponseSignature(t *testing.T) {
	configuredKey := newTestWechatPayKey(t)
	wrongSigningKey := newTestWechatPayKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wrongSigningKey.id = configuredKey.id
		writeSignedWechatPayResponse(t, w, wrongSigningKey, `{"code_url":"weixin://invalid-signature"}`)
	}))
	defer server.Close()

	raw := baseWechatPayConfig(t)
	addPublicKeyMode(raw, configuredKey)
	raw["base_url"] = server.URL
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	_, err = CreatePayment(context.Background(), cfg, CreateInput{
		OrderNo: "ORDER-BAD-SIGNATURE",
		Amount:  "1.00",
	}, constants.PaymentInteractionQR)
	if !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("invalid response signature error = %v, want ErrSignatureInvalid", err)
	}
}

func TestWechatPayPublicKeySecurityEchoSuccess(t *testing.T) {
	wechatPayKey := newTestWechatPayKey(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != securityEchoPath {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept = %q, want application/json", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", got)
		}
		if got := r.Header.Get("Wechatpay-Serial"); got != wechatPayKey.id {
			t.Errorf("Wechatpay-Serial = %q, want %q", got, wechatPayKey.id)
		}
		if got := r.Header.Get("Authorization"); !strings.HasPrefix(got, "WECHATPAY2-SHA256-RSA2048 ") {
			t.Errorf("Authorization header is missing or invalid: %q", got)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if _, exists := payload["notify_url"]; exists {
			t.Error("security echo request must not include notify_url")
		}
		if _, exists := payload["encrypted_echo_message"]; exists {
			t.Error("public-key signature test must not include encrypted_echo_message")
		}
		echoMessage, _ := payload["echo_message"].(string)
		if !strings.HasPrefix(echoMessage, "dujiao-next-") {
			t.Errorf("unexpected echo_message: %q", echoMessage)
		}
		responseBody, err := json.Marshal(map[string]string{"echo_message": echoMessage})
		if err != nil {
			t.Errorf("marshal response: %v", err)
			return
		}
		writeSignedWechatPayResponse(t, w, wechatPayKey, string(responseBody))
	}))
	defer server.Close()

	raw := baseWechatPayConfig(t)
	addPublicKeyMode(raw, wechatPayKey)
	raw["verification_mode"] = verificationModeCombined
	raw["base_url"] = server.URL
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	result, err := TestWechatPayPublicKey(context.Background(), cfg)
	if err != nil {
		t.Fatalf("security echo failed: %v", err)
	}
	if result.ResponseSerial != wechatPayKey.id || !result.RequestSignatureAccepted ||
		!result.ResponseSignatureValid || !result.EchoMessageMatched {
		t.Fatalf("unexpected security echo result: %+v", result)
	}
}

func TestWechatPayPublicKeySecurityEchoRejectsInvalidSignature(t *testing.T) {
	configuredKey := newTestWechatPayKey(t)
	wrongSigningKey := newTestWechatPayKey(t)
	wrongSigningKey.id = configuredKey.id
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		echoMessage, _ := payload["echo_message"].(string)
		responseBody, _ := json.Marshal(map[string]string{"echo_message": echoMessage})
		writeSignedWechatPayResponse(t, w, wrongSigningKey, string(responseBody))
	}))
	defer server.Close()

	raw := baseWechatPayConfig(t)
	addPublicKeyMode(raw, configuredKey)
	raw["base_url"] = server.URL
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}

	if _, err := TestWechatPayPublicKey(context.Background(), cfg); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("invalid security echo signature error = %v, want ErrSignatureInvalid", err)
	}
}

func TestCombinedVerifierAcceptsPlatformCertificateAndPublicKey(t *testing.T) {
	publicKey := newTestWechatPayKey(t)
	platformPrivateKey, certificate := newTestPlatformCertificate(t)
	cfg := &Config{
		VerificationMode:     verificationModeCombined,
		WechatPayPublicKeyID: publicKey.id,
		WechatPayPublicKey:   publicKey.publicKeyPEM,
	}
	visitor := core.NewCertificateMapWithList([]*x509.Certificate{certificate})
	verifier, err := createWechatPayVerifierWithVisitor(cfg, visitor)
	if err != nil {
		t.Fatalf("create combined verifier: %v", err)
	}
	message := "timestamp\nnonce\nbody\n"
	platformSerial := fmt.Sprintf("%X", certificate.SerialNumber.Bytes())
	tests := []struct {
		name       string
		serial     string
		privateKey *rsa.PrivateKey
	}{
		{name: "platform certificate", serial: platformSerial, privateKey: platformPrivateKey},
		{name: "wechatpay public key", serial: publicKey.id, privateKey: publicKey.privateKey},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			signature := signWechatPayMessage(t, tc.privateKey, message)
			if err := verifier.Verify(context.Background(), tc.serial, message, signature); err != nil {
				t.Fatalf("combined verifier rejected %s: %v", tc.name, err)
			}
		})
	}
}

func TestVerifyAndDecodeWebhookWechatPayPublicKey(t *testing.T) {
	key := newTestWechatPayKey(t)
	raw := baseWechatPayConfig(t)
	addPublicKeyMode(raw, key)
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	plaintext := `{
		"appid":"wx1234567890",
		"mchid":"1900000109",
		"out_trade_no":"ORDER-WEBHOOK-1",
		"transaction_id":"4200000000000000001",
		"trade_type":"NATIVE",
		"trade_state":"SUCCESS",
		"trade_state_desc":"支付成功",
		"bank_type":"OTHERS",
		"attach":"payment-1",
		"success_time":"2026-08-07T12:00:00+08:00",
		"amount":{"total":100,"payer_total":100,"currency":"CNY","payer_currency":"CNY"}
	}`
	headers, body := buildSignedWechatPayNotification(t, key, cfg.APIV3Key, plaintext)

	result, err := VerifyAndDecodeWebhook(context.Background(), cfg, headers, body)
	if err != nil {
		t.Fatalf("verify public key webhook: %v", err)
	}
	if result.OrderNo != "ORDER-WEBHOOK-1" || result.TransactionID != "4200000000000000001" {
		t.Fatalf("unexpected webhook result: %+v", result)
	}
	if result.Status != constants.PaymentStatusSuccess || result.Amount != "1.00" || result.Currency != "CNY" {
		t.Fatalf("unexpected webhook payment values: %+v", result)
	}

	headers["Wechatpay-Signature"] = "WECHATPAY/SIGNTEST/invalid"
	if _, err := VerifyAndDecodeWebhook(context.Background(), cfg, headers, body); !errors.Is(err, ErrSignatureInvalid) {
		t.Fatalf("SIGNTEST error = %v, want ErrSignatureInvalid", err)
	}
}

func buildSignedWechatPayNotification(
	t *testing.T,
	key testWechatPayKey,
	apiV3Key string,
	plaintext string,
) (map[string]string, []byte) {
	t.Helper()
	block, err := aes.NewCipher([]byte(apiV3Key))
	if err != nil {
		t.Fatalf("create AES cipher: %v", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("create GCM cipher: %v", err)
	}
	resourceNonce := "0123456789ab"
	associatedData := "transaction"
	ciphertext := aead.Seal(nil, []byte(resourceNonce), []byte(plaintext), []byte(associatedData))
	payload := map[string]interface{}{
		"id":            "EV-WEBHOOK-1",
		"create_time":   "2026-08-07T12:00:01+08:00",
		"event_type":    "TRANSACTION.SUCCESS",
		"resource_type": "encrypt-resource",
		"summary":       "支付成功",
		"resource": map[string]interface{}{
			"original_type":   "transaction",
			"algorithm":       "AEAD_AES_256_GCM",
			"ciphertext":      base64.StdEncoding.EncodeToString(ciphertext),
			"associated_data": associatedData,
			"nonce":           resourceNonce,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal notification: %v", err)
	}
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := "notification-nonce"
	return map[string]string{
		"Wechatpay-Timestamp": timestamp,
		"Wechatpay-Nonce":     nonce,
		"Wechatpay-Serial":    key.id,
		"Wechatpay-Signature": signWechatPayMessage(
			t,
			key.privateKey,
			timestamp+"\n"+nonce+"\n"+string(body)+"\n",
		),
		"Wechatpay-Signature-Type": "WECHATPAY2-SHA256-RSA2048",
	}, body
}
