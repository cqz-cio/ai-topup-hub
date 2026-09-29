package epusdt

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestParseConfigAndNormalizeDefaults(t *testing.T) {
	cfg, err := ParseConfig(map[string]interface{}{
		"gateway_url": " https://epusdt.example.com/ ",
		"pid":         " 1000 ",
		"secret_key":  " sk-test ",
		"token":       " USDT ",
		"network":     " TRON ",
		"notify_url":  " https://example.com/notify ",
		"return_url":  " https://example.com/return ",
	})
	if err != nil {
		t.Fatalf("parse config failed: %v", err)
	}
	cfg.Normalize()
	if cfg.GatewayURL != "https://epusdt.example.com" {
		t.Fatalf("unexpected gateway url: %q", cfg.GatewayURL)
	}
	if cfg.PID != "1000" {
		t.Fatalf("unexpected pid: %q", cfg.PID)
	}
	if cfg.Token != "usdt" {
		t.Fatalf("unexpected token (should lowercase): %q", cfg.Token)
	}
	if cfg.Network != "tron" {
		t.Fatalf("unexpected network (should lowercase): %q", cfg.Network)
	}
	if cfg.Currency != strings.ToLower(constants.SiteCurrencyDefault) {
		t.Fatalf("unexpected default currency (should be lowercased): %q", cfg.Currency)
	}
}

func TestValidateConfigRequiresAllFields(t *testing.T) {
	tests := []struct {
		name string
		cfg  *Config
	}{
		{name: "nil", cfg: nil},
		{name: "missing gateway_url", cfg: &Config{PID: "1", SecretKey: "s", Token: "usdt", Network: "tron", NotifyURL: "n", ReturnURL: "r"}},
		{name: "missing pid", cfg: &Config{GatewayURL: "g", SecretKey: "s", Token: "usdt", Network: "tron", NotifyURL: "n", ReturnURL: "r"}},
		{name: "missing secret_key", cfg: &Config{GatewayURL: "g", PID: "1", Token: "usdt", Network: "tron", NotifyURL: "n", ReturnURL: "r"}},
		{name: "missing token", cfg: &Config{GatewayURL: "g", PID: "1", SecretKey: "s", Network: "tron", NotifyURL: "n", ReturnURL: "r"}},
		{name: "missing network", cfg: &Config{GatewayURL: "g", PID: "1", SecretKey: "s", Token: "usdt", NotifyURL: "n", ReturnURL: "r"}},
		{name: "missing notify_url", cfg: &Config{GatewayURL: "g", PID: "1", SecretKey: "s", Token: "usdt", Network: "tron", ReturnURL: "r"}},
		{name: "missing return_url", cfg: &Config{GatewayURL: "g", PID: "1", SecretKey: "s", Token: "usdt", Network: "tron", NotifyURL: "n"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := ValidateConfig(tc.cfg); err == nil {
				t.Fatalf("expected error for case %q, got nil", tc.name)
			}
		})
	}
}

func TestValidateConfigPassesWhenAllRequiredFieldsPresent(t *testing.T) {
	cfg := &Config{
		GatewayURL: "https://x", PID: "1", SecretKey: "s",
		Token: "usdt", Network: "tron",
		NotifyURL: "https://n", ReturnURL: "https://r",
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidateConfigCashierDoesNotRequireTokenAndNetwork(t *testing.T) {
	cfg := &Config{
		GatewayURL: "https://x", PID: "1", SecretKey: "s",
		OrderMode: constants.PaymentEpusdtOrderModeCashier,
		NotifyURL: "https://n", ReturnURL: "https://r",
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("cashier config should not require token/network: %v", err)
	}
}

func TestValidateConfigTransactionRequiresTokenAndNetwork(t *testing.T) {
	base := Config{
		GatewayURL: "https://x", PID: "1", SecretKey: "s",
		OrderMode: constants.PaymentEpusdtOrderModeTransaction,
		NotifyURL: "https://n", ReturnURL: "https://r",
	}
	if err := ValidateConfig(&base); err == nil {
		t.Fatal("transaction config without token/network should be rejected")
	}
}

func TestSignDeterministicAndOmitsEmptyAndSignature(t *testing.T) {
	params := map[string]interface{}{
		"pid":          "1000",
		"order_id":     "ORD-1",
		"currency":     "cny",
		"token":        "usdt",
		"network":      "tron",
		"amount":       100.0,
		"notify_url":   "https://example.com/notify",
		"redirect_url": "", // 空值应被排除
		"signature":    "should-be-ignored",
	}
	got := Sign(params, "sk-test")
	expected := hmacSHA256LowerHex("amount=100&currency=cny&network=tron&notify_url=https://example.com/notify&order_id=ORD-1&pid=1000&token=usdt", "sk-test")
	if got != expected {
		t.Fatalf("signature mismatch:\n  got:  %s\n want: %s", got, expected)
	}
}

func TestToPaymentStatus(t *testing.T) {
	tests := []struct {
		name   string
		status int
		expect string
	}{
		{"Success", StatusSuccess, constants.PaymentStatusSuccess},
		{"Expired", StatusExpired, constants.PaymentStatusExpired},
		{"Waiting", StatusWaiting, constants.PaymentStatusPending},
		{"Unknown", 999, constants.PaymentStatusPending},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ToPaymentStatus(tc.status); got != tc.expect {
				t.Fatalf("got %s, want %s", got, tc.expect)
			}
		})
	}
}

// hmacSHA256LowerHex 测试辅助：避免和 Sign 内部实现耦合，独立计算 HMAC-SHA256
func hmacSHA256LowerHex(s, key string) string {
	mac := hmac.New(sha256.New, []byte(key))
	_, _ = mac.Write([]byte(s))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestCreatePayment_BuildsRequestAndConstructsPaymentURL(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/payments/gmpay/v1/order/create-transaction" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status_code":200,"message":"ok","data":{"trade_id":"T20260509ABC","order_id":"ORD-1","amount":"100","actual_amount":"13.45","token":"usdt","network":"tron"}}`))
	}))
	defer srv.Close()

	cfg := &Config{
		GatewayURL: srv.URL,
		PID:        "1000",
		SecretKey:  "sk-test",
		Token:      "usdt",
		Network:    "tron",
		Currency:   "cny",
		NotifyURL:  "https://example.com/notify",
		ReturnURL:  "https://example.com/return",
	}
	cfg.Normalize()

	result, err := CreatePayment(context.Background(), cfg, CreateInput{
		OrderNo:   "ORD-1",
		Amount:    "100",
		Name:      "VIP Plan",
		NotifyURL: cfg.NotifyURL,
		ReturnURL: cfg.ReturnURL,
	})
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if result.TradeID != "T20260509ABC" {
		t.Fatalf("unexpected trade_id: %s", result.TradeID)
	}
	expectedURL := srv.URL + "/pay/checkout-counter/T20260509ABC"
	if result.PaymentURL != expectedURL {
		t.Fatalf("unexpected payment url: got %s, want %s", result.PaymentURL, expectedURL)
	}

	if capturedBody["pid"] != "1000" {
		t.Fatalf("pid mismatch: %v", capturedBody["pid"])
	}
	if capturedBody["currency"] != "cny" {
		t.Fatalf("currency mismatch: %v", capturedBody["currency"])
	}
	if capturedBody["token"] != "usdt" {
		t.Fatalf("token mismatch: %v", capturedBody["token"])
	}
	if capturedBody["network"] != "tron" {
		t.Fatalf("network mismatch: %v", capturedBody["network"])
	}
	if capturedBody["order_id"] != "ORD-1" {
		t.Fatalf("order_id mismatch: %v", capturedBody["order_id"])
	}
	if amt, ok := capturedBody["amount"].(float64); !ok || amt != 100 {
		t.Fatalf("amount mismatch: %v", capturedBody["amount"])
	}
	if _, hasSig := capturedBody["signature"]; !hasSig {
		t.Fatalf("signature missing")
	}
	if capturedBody["notify_url"] != "https://example.com/notify" {
		t.Fatalf("notify_url mismatch: %v", capturedBody["notify_url"])
	}
	if capturedBody["redirect_url"] != "https://example.com/return" {
		t.Fatalf("redirect_url mismatch: %v", capturedBody["redirect_url"])
	}
	if capturedBody["name"] != "VIP Plan" {
		t.Fatalf("name mismatch: %v", capturedBody["name"])
	}
}

func TestCreatePaymentCashierUsesCreateTransactionAndOmitsTokenNetwork(t *testing.T) {
	var capturedBody map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/payments/gmpay/v1/order/create-transaction" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		body, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(body, &capturedBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"trade_id":"CASHIER-1"}}`))
	}))
	defer srv.Close()

	cfg := &Config{
		GatewayURL: srv.URL, PID: "1000", SecretKey: "sk-test",
		OrderMode: constants.PaymentEpusdtOrderModeCashier,
		Currency:  "cny", NotifyURL: "https://example.com/notify", ReturnURL: "https://example.com/return",
	}
	result, err := CreatePayment(context.Background(), cfg, CreateInput{OrderNo: "ORD-1", Amount: "100"})
	if err != nil {
		t.Fatalf("CreatePayment cashier failed: %v", err)
	}
	if result.TradeID != "CASHIER-1" {
		t.Fatalf("trade_id = %q, want CASHIER-1", result.TradeID)
	}
	if _, ok := capturedBody["token"]; ok {
		t.Fatalf("cashier request must omit token: %v", capturedBody["token"])
	}
	if _, ok := capturedBody["network"]; ok {
		t.Fatalf("cashier request must omit network: %v", capturedBody["network"])
	}
	if _, ok := capturedBody["signature"]; !ok {
		t.Fatal("cashier request signature missing")
	}
}

func TestCreatePayment_RejectsAmountUnderMinimum(t *testing.T) {
	cfg := &Config{
		GatewayURL: "https://x.example.com",
		PID:        "1", SecretKey: "s", Token: "usdt", Network: "tron",
		Currency: "cny", NotifyURL: "https://n", ReturnURL: "https://r",
	}
	_, err := CreatePayment(context.Background(), cfg, CreateInput{
		OrderNo: "ORD-1", Amount: "0.005",
		NotifyURL: cfg.NotifyURL, ReturnURL: cfg.ReturnURL,
	})
	if err == nil {
		t.Fatalf("expected error for amount under 0.01")
	}
}

func TestCreatePayment_RejectsOrderNoTooLong(t *testing.T) {
	cfg := &Config{
		GatewayURL: "https://x.example.com",
		PID:        "1", SecretKey: "s", Token: "usdt", Network: "tron",
		Currency: "cny", NotifyURL: "https://n", ReturnURL: "https://r",
	}
	_, err := CreatePayment(context.Background(), cfg, CreateInput{
		OrderNo: strings.Repeat("X", 33), Amount: "1.00",
		NotifyURL: cfg.NotifyURL, ReturnURL: cfg.ReturnURL,
	})
	if err == nil {
		t.Fatalf("expected error for order_no len > 32")
	}
}

func TestParseAndVerifyCallback_Success(t *testing.T) {
	cfg := &Config{SecretKey: "sk-test"}

	params := map[string]interface{}{
		"pid":                  "1000",
		"trade_id":             "T1",
		"order_id":             "ORD-1",
		"amount":               100.0,
		"actual_amount":        13.45,
		"receive_address":      "TXxxx",
		"token":                "usdt",
		"block_transaction_id": "0xabc",
		"status":               StatusSuccess,
	}
	sig := Sign(params, cfg.SecretKey)

	body := []byte(`{
		"pid": "1000",
		"trade_id": "T1",
		"order_id": "ORD-1",
		"amount": 100,
		"actual_amount": 13.45,
		"receive_address": "TXxxx",
		"token": "usdt",
		"block_transaction_id": "0xabc",
		"status": 2,
		"signature": "` + sig + `"
	}`)

	data, err := ParseCallback(body)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := VerifyCallback(cfg, data); err != nil {
		t.Fatalf("verify failed: %v", err)
	}
	if data.GetAmount() != 100 {
		t.Fatalf("amount: got %v", data.GetAmount())
	}
	if data.GetActualAmount() != 13.45 {
		t.Fatalf("actual_amount: got %v", data.GetActualAmount())
	}
}

func TestVerifyCallback_RejectsBadSignature(t *testing.T) {
	cfg := &Config{SecretKey: "sk-test"}
	body := []byte(`{
		"pid": "1000", "trade_id": "T1", "order_id": "ORD-1",
		"amount": 100, "actual_amount": 13.45,
		"receive_address": "TXxxx", "token": "usdt",
		"block_transaction_id": "0xabc", "status": 2,
		"signature": "wrong-sig"
	}`)
	data, err := ParseCallback(body)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if err := VerifyCallback(cfg, data); err == nil {
		t.Fatalf("expected ErrSignatureInvalid")
	}
}

func TestVerifyCallback_RejectsNonSuccessStatus(t *testing.T) {
	cfg := &Config{SecretKey: "sk-test"}
	data := &CallbackData{Status: StatusWaiting, Signature: "anything"}
	if err := VerifyCallback(cfg, data); err == nil {
		t.Fatalf("expected error for non-success status")
	}
}

func TestCreatePayment_GatewayHTTP5xxBecomesRequestFailed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	cfg := &Config{
		GatewayURL: srv.URL, PID: "1", SecretKey: "s",
		Token: "usdt", Network: "tron", Currency: "cny",
		NotifyURL: "https://n", ReturnURL: "https://r",
	}
	_, err := CreatePayment(context.Background(), cfg, CreateInput{
		OrderNo: "ORD-1", Amount: "1.00",
		NotifyURL: cfg.NotifyURL, ReturnURL: cfg.ReturnURL,
	})
	if err == nil {
		t.Fatalf("expected error for 5xx")
	}
	if !errors.Is(err, ErrRequestFailed) {
		t.Fatalf("expected ErrRequestFailed, got %v", err)
	}
}

func TestCreatePayment_NonJSONResponseBecomesResponseInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not a json body"))
	}))
	defer srv.Close()

	cfg := &Config{
		GatewayURL: srv.URL, PID: "1", SecretKey: "s",
		Token: "usdt", Network: "tron", Currency: "cny",
		NotifyURL: "https://n", ReturnURL: "https://r",
	}
	_, err := CreatePayment(context.Background(), cfg, CreateInput{
		OrderNo: "ORD-1", Amount: "1.00",
		NotifyURL: cfg.NotifyURL, ReturnURL: cfg.ReturnURL,
	})
	if err == nil {
		t.Fatalf("expected error for non-JSON response")
	}
	if !errors.Is(err, ErrResponseInvalid) {
		t.Fatalf("expected ErrResponseInvalid, got %v", err)
	}
}

func TestCreatePayment_MissingTradeIDBecomesResponseInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status_code":200,"message":"ok","data":{"order_id":"ORD-1"}}`))
	}))
	defer srv.Close()

	cfg := &Config{
		GatewayURL: srv.URL, PID: "1", SecretKey: "s",
		Token: "usdt", Network: "tron", Currency: "cny",
		NotifyURL: "https://n", ReturnURL: "https://r",
	}
	_, err := CreatePayment(context.Background(), cfg, CreateInput{
		OrderNo: "ORD-1", Amount: "1.00",
		NotifyURL: cfg.NotifyURL, ReturnURL: cfg.ReturnURL,
	})
	if err == nil {
		t.Fatalf("expected error when trade_id missing")
	}
	if !errors.Is(err, ErrResponseInvalid) {
		t.Fatalf("expected ErrResponseInvalid, got %v", err)
	}
}
