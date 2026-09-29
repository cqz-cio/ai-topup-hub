package okpay

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestSignPayloadMatchesOfficialVectorA 对齐官方文档 9. 测试向量 A(请求,扁平参数)。
// 官方 base 顺序: amount, coin, id, nonce, timestamp, unique_id(ASCII 升序)。
func TestSignPayloadMatchesOfficialVectorA(t *testing.T) {
	pairs := []OrderedPair{
		{Key: "amount", Value: "100.5"},
		{Key: "coin", Value: "USDT"},
		{Key: "id", Value: "10001"},
		{Key: "nonce", Value: "a1b2c3d4e5"},
		{Key: "timestamp", Value: "1782680000"},
		{Key: "unique_id", Value: "ORDER-20260628-001"},
	}
	sign := buildSignature(pairs, "TESTtoken123456789abcdefghijABCD")
	want := "7444ADFD8E4F4DA09D752DDF9345E0EE56DC25090FCFAF675DD042830E5E3F79"
	if sign != want {
		t.Fatalf("unexpected sign: %s, want %s", sign, want)
	}
}

// TestBuildSignatureMatchesOfficialVectorB 对齐官方文档 9. 测试向量 B(回调,含嵌套 data)。
func TestBuildSignatureMatchesOfficialVectorB(t *testing.T) {
	pairs := []OrderedPair{
		{Key: "code", Value: "200"},
		{Key: "data.amount", Value: "100.5"},
		{Key: "data.coin", Value: "USDT"},
		{Key: "data.order_id", Value: "abc123def456"},
		{Key: "data.pay_user_id", Value: "123456789"},
		{Key: "data.status", Value: "1"},
		{Key: "data.type", Value: "deposit"},
		{Key: "data.unique_id", Value: "ORDER-20260628-001"},
		{Key: "id", Value: "10001"},
		{Key: "status", Value: "success"},
	}
	sign := buildSignature(pairs, "TESTtoken123456789abcdefghijABCD")
	want := "64B09C8847849FA6921D8FFBDF8E406D4A8EA623E53970712350F61783403F7D"
	if sign != want {
		t.Fatalf("unexpected sign: %s, want %s", sign, want)
	}
}

// TestBuildSignatureMatchesOfficialVectorC 对齐官方文档 9. 测试向量 C:
// 保留 0/"0"/false,丢弃 null/空串,点号嵌套展开。
func TestBuildSignatureMatchesOfficialVectorC(t *testing.T) {
	pairs := []OrderedPair{
		{Key: "a", Value: "0"},
		{Key: "b", Value: "0"},
		{Key: "e", Value: "false"},
		{Key: "f", Value: "hello"},
		{Key: "id", Value: "7"},
		{Key: "nest.x", Value: "1"},
		{Key: "nest.y", Value: "2"},
	}
	sign := buildSignature(pairs, "TESTtoken123456789abcdefghijABCD")
	want := "8BC0AF979075038025DDD51B6F4A2E6CF3FF9B5B5371EB2268D303F89883E92A"
	if sign != want {
		t.Fatalf("unexpected sign: %s, want %s", sign, want)
	}
}

func TestVerifyCallbackMatchesOfficialVectorB(t *testing.T) {
	cfg := &Config{
		MerchantID:    "10001",
		MerchantToken: "TESTtoken123456789abcdefghijABCD",
	}
	body := `{"status":"success","code":200,"data":{"order_id":"abc123def456","unique_id":"ORDER-20260628-001","pay_user_id":123456789,"amount":"100.5","coin":"USDT","status":1,"type":"deposit"},"id":10001,"sign":"64B09C8847849FA6921D8FFBDF8E406D4A8EA623E53970712350F61783403F7D"}`
	data, err := ParseCallback([]byte(body))
	if err != nil {
		t.Fatalf("ParseCallback failed: %v", err)
	}
	if err := VerifyCallback(cfg, data); err != nil {
		t.Fatalf("VerifyCallback failed: %v", err)
	}
	if status := ToPaymentStatus(data.RequestStatus, data.PaymentStatus); status != "success" {
		t.Fatalf("unexpected payment status: %s", status)
	}
	if data.OrderID != "abc123def456" {
		t.Fatalf("unexpected order id: %s", data.OrderID)
	}
	if data.UniqueID != "ORDER-20260628-001" {
		t.Fatalf("unexpected unique id: %s", data.UniqueID)
	}
}

func TestVerifyCallbackRejectsBadSignature(t *testing.T) {
	cfg := &Config{
		MerchantID:    "10001",
		MerchantToken: "TESTtoken123456789abcdefghijABCD",
	}
	body := `{"status":"success","code":200,"data":{"order_id":"abc123def456","unique_id":"ORDER-20260628-001","pay_user_id":123456789,"amount":"100.5","coin":"USDT","status":1,"type":"deposit"},"id":10001,"sign":"0000000000000000000000000000000000000000000000000000000000000000"}`
	data, err := ParseCallback([]byte(body))
	if err != nil {
		t.Fatalf("ParseCallback failed: %v", err)
	}
	if err := VerifyCallback(cfg, data); err == nil {
		t.Fatal("expected signature mismatch error")
	}
}

func TestParseCallbackRejectsFormEncodedBody(t *testing.T) {
	body := "code=200&data.order_id=abc123&status=success&sign=DEADBEEF"
	if _, err := ParseCallback([]byte(body)); err == nil {
		t.Fatal("expected form-encoded callback to be rejected under the JSON-only protocol")
	}
}

func TestCreatePayment(t *testing.T) {
	var receivedValues map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatalf("ParseForm failed: %v", err)
		}
		receivedValues = map[string][]string(r.PostForm)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","code":200,"data":{"order_id":"OK-ORDER-1","pay_url":"https://pay.example.com/ok"}}`))
	}))
	defer server.Close()

	cfg := &Config{
		GatewayURL:    server.URL,
		MerchantID:    "shop-1",
		MerchantToken: "token-1",
		ReturnURL:     "https://shop.example.com/pay",
		CallbackURL:   "https://api.example.com/api/v1/payments/callback",
		ExchangeRate:  "7",
		Coin:          "USDT",
	}
	result, err := CreatePayment(context.Background(), cfg, CreateInput{
		UniqueID: "DJP1001",
		Name:     "支付订单",
		Amount:   "18.80",
	})
	if err != nil {
		t.Fatalf("CreatePayment failed: %v", err)
	}
	if result.OrderID != "OK-ORDER-1" {
		t.Fatalf("unexpected order id: %s", result.OrderID)
	}
	if result.PayURL != "https://pay.example.com/ok" {
		t.Fatalf("unexpected pay url: %s", result.PayURL)
	}
	if receivedValues["unique_id"][0] != "DJP1001" {
		t.Fatalf("request body should contain unique_id, got %v", receivedValues["unique_id"])
	}
	if receivedValues["amount"][0] != "131.60000000" {
		t.Fatalf("request body should contain converted amount, got %v", receivedValues["amount"])
	}
	if receivedValues["sign"][0] == "" {
		t.Fatal("request body should contain sign")
	}
	if receivedValues["timestamp"][0] == "" {
		t.Fatal("request body should contain timestamp")
	}
	if receivedValues["nonce"][0] == "" {
		t.Fatal("request body should contain nonce")
	}

	// 用收到的 timestamp/nonce 重新算一遍签名,确认服务端可逐字节复现。
	pairs := []OrderedPair{
		{Key: "amount", Value: receivedValues["amount"][0]},
		{Key: "callback_url", Value: receivedValues["callback_url"][0]},
		{Key: "coin", Value: receivedValues["coin"][0]},
		{Key: "id", Value: receivedValues["id"][0]},
		{Key: "name", Value: receivedValues["name"][0]},
		{Key: "nonce", Value: receivedValues["nonce"][0]},
		{Key: "return_url", Value: receivedValues["return_url"][0]},
		{Key: "timestamp", Value: receivedValues["timestamp"][0]},
		{Key: "unique_id", Value: receivedValues["unique_id"][0]},
	}
	expected := buildSignature(pairs, cfg.MerchantToken)
	if !strings.EqualFold(expected, receivedValues["sign"][0]) {
		t.Fatalf("signature does not match recomputed value: got %s want %s", receivedValues["sign"][0], expected)
	}
}

func TestConvertAmountByRate(t *testing.T) {
	converted, err := ConvertAmountByRate("1.00", "0.15")
	if err != nil {
		t.Fatalf("ConvertAmountByRate failed: %v", err)
	}
	if converted.StringFixed(8) != "0.15000000" {
		t.Fatalf("unexpected converted amount: %s", converted.StringFixed(8))
	}
}
