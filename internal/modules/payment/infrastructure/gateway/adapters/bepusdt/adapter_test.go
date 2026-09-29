package bepusdtadapter

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	paymentcontract "github.com/dujiao-next/internal/modules/payment/contract"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/gateway/bepusdt"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

func TestBepusdtAdapter_Type(t *testing.T) {
	a := NewBepusdtAdapter()
	want := constants.PaymentProviderBepusdt + ":"
	if got := a.Type(); got != want {
		t.Fatalf("Type() = %q, want %q", got, want)
	}
}

func TestBepusdtAdapter_ValidateConfig_UnsupportedChannel(t *testing.T) {
	a := NewBepusdtAdapter()
	err := a.ValidateConfig(jsonmap.JSON{}, "no-such-channel-type")
	if err == nil {
		t.Fatalf("expected error for unsupported channel")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayUnsupportedChannel) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayUnsupportedChannel, got %v", err)
	}
}

func TestBepusdtAdapter_CreatePayment_ConfigInvalidMapped(t *testing.T) {
	a := NewBepusdtAdapter()
	// 用 bepusdt 真实支持的 channelType（usdt-trc20 / usdc-trc20 / trx）
	_, err := a.CreatePayment(context.Background(), jsonmap.JSON{}, paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER_1",
		Currency:    "USDT",
		ChannelType: "usdt-trc20",
	})
	if err == nil {
		t.Fatalf("expected error from empty config")
	}
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected wrapped paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

func TestBepusdtAdapter_CreatePayment_QRModeUsesWalletAddress(t *testing.T) {
	a := NewBepusdtAdapter()
	server := newBepusdtCreatePaymentServer(t, "usdt.trc20")
	defer server.Close()

	result, err := a.CreatePayment(context.Background(), validBepusdtConfig(server.URL), paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER-QR-1",
		Subject:     "测试商品",
		Amount:      money.FromDecimal(decimal.RequireFromString("28.88")),
		ChannelType: constants.PaymentChannelTypeUsdtTrc20,
		Extra:       jsonmap.JSON{"interaction_mode": constants.PaymentInteractionQR},
	})
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	if result.RedirectURL != "" {
		t.Fatalf("RedirectURL = %q, want empty in qr mode", result.RedirectURL)
	}
	if result.QRCodeURL != "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t" {
		t.Fatalf("QRCodeURL = %q, want wallet address", result.QRCodeURL)
	}
	data := result.Payload["data"].(map[string]interface{})
	if data["actual_amount"] != "4.25" {
		t.Fatalf("actual_amount = %v, want 4.25", data["actual_amount"])
	}
	if data["trade_type"] != "usdt.trc20" {
		t.Fatalf("trade_type = %v, want usdt.trc20", data["trade_type"])
	}
	if data["chain"] != "tron" || data["token_id"] != "tron-usdt" {
		t.Fatalf("unexpected chain labels: chain=%v token_id=%v", data["chain"], data["token_id"])
	}
}

func TestBepusdtAdapter_CreatePayment_RedirectModeKeepsCashierURL(t *testing.T) {
	a := NewBepusdtAdapter()
	server := newBepusdtCreatePaymentServer(t, "usdt.trc20")
	defer server.Close()

	result, err := a.CreatePayment(context.Background(), validBepusdtConfig(server.URL), paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER-REDIRECT-1",
		Subject:     "测试商品",
		Amount:      money.FromDecimal(decimal.RequireFromString("28.88")),
		ChannelType: constants.PaymentChannelTypeUsdtTrc20,
		Extra:       jsonmap.JSON{"interaction_mode": constants.PaymentInteractionRedirect},
	})
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	wantURL := "https://bepusdt.example/pay/checkout-counter/BEP-1"
	if result.RedirectURL != wantURL {
		t.Fatalf("RedirectURL = %q, want %q", result.RedirectURL, wantURL)
	}
	if result.QRCodeURL != wantURL {
		t.Fatalf("QRCodeURL = %q, want %q", result.QRCodeURL, wantURL)
	}
}

func TestBepusdtAdapter_CreatePayment_ProviderChannelUsesConfiguredTradeType(t *testing.T) {
	a := NewBepusdtAdapter()
	server := newBepusdtCreatePaymentServer(t, "usdc.trc20")
	defer server.Close()

	cfg := validBepusdtConfig(server.URL)
	cfg["trade_type"] = "usdc.trc20"
	result, err := a.CreatePayment(context.Background(), cfg, paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER-PROVIDER-CHANNEL-1",
		Subject:     "测试商品",
		Amount:      money.FromDecimal(decimal.RequireFromString("28.88")),
		ChannelType: constants.PaymentProviderBepusdt,
		Extra:       jsonmap.JSON{"interaction_mode": constants.PaymentInteractionRedirect},
	})
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}
	data := result.Payload["data"].(map[string]interface{})
	if data["trade_type"] != "usdc.trc20" {
		t.Fatalf("trade_type = %v, want usdc.trc20", data["trade_type"])
	}
	if result.DisplayChannelType != "usdc.trc20" {
		t.Fatalf("DisplayChannelType = %q, want usdc.trc20", result.DisplayChannelType)
	}
}

func TestBepusdtAdapter_CreatePayment_MissingTradeTypeUsesLegacyDefault(t *testing.T) {
	a := NewBepusdtAdapter()
	server := newBepusdtCreatePaymentServer(t, "usdt.trc20")
	defer server.Close()

	cfg := validBepusdtConfig(server.URL)
	delete(cfg, "trade_type")
	result, err := a.CreatePayment(context.Background(), cfg, paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER-LEGACY-CHANNEL-1",
		Subject:     "测试商品",
		Amount:      money.FromDecimal(decimal.RequireFromString("28.88")),
		ChannelType: constants.PaymentChannelTypeTrx,
		Extra:       jsonmap.JSON{"interaction_mode": constants.PaymentInteractionRedirect},
	})
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}
	data := result.Payload["data"].(map[string]interface{})
	if data["trade_type"] != "usdt.trc20" {
		t.Fatalf("trade_type = %v, want usdt.trc20", data["trade_type"])
	}
	if result.DisplayChannelType != "usdt.trc20" {
		t.Fatalf("DisplayChannelType = %q, want usdt.trc20", result.DisplayChannelType)
	}
}

func TestBepusdtAdapter_ValidateConfig_ProviderChannelUsesDefaultTradeType(t *testing.T) {
	a := NewBepusdtAdapter()
	cfg := validBepusdtConfig("https://bepusdt.example")
	delete(cfg, "trade_type")

	if err := a.ValidateConfig(cfg, constants.PaymentProviderBepusdt); err != nil {
		t.Fatalf("ValidateConfig() failed: %v", err)
	}
}

func TestBepusdtAdapter_CreatePayment_CashierModeUsesCreateOrder(t *testing.T) {
	a := NewBepusdtAdapter()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/order/create-order" {
			t.Fatalf("path = %s, want /api/v1/order/create-order", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		if _, ok := payload["trade_type"]; ok {
			t.Fatalf("trade_type should not be sent for cashier order mode")
		}
		if payload["currencies"] != "USDT,USDC" {
			t.Fatalf("currencies = %v, want USDT,USDC", payload["currencies"])
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status_code": 200,
			"message": "success",
			"data": {
				"fiat": "CNY",
				"trade_id": "BEP-CASHIER-1",
				"order_id": "ORDER-CASHIER-1",
				"amount": "28.88",
				"expiration_time": 1200,
				"status": 1,
				"payment_url": "https://bepusdt.example/pay/cashier/BEP-CASHIER-1",
				"reselect": true
			}
		}`))
	}))
	defer server.Close()

	cfg := validBepusdtConfig(server.URL)
	cfg["order_mode"] = constants.PaymentBepusdtOrderModeCashier
	cfg["trade_type"] = "usdt.trc20"
	cfg["currencies"] = " usdt, usdc "
	result, err := a.CreatePayment(context.Background(), cfg, paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER-CASHIER-1",
		Subject:     "测试商品",
		Amount:      money.FromDecimal(decimal.RequireFromString("28.88")),
		ChannelType: constants.PaymentProviderBepusdt,
		Extra:       jsonmap.JSON{"interaction_mode": constants.PaymentInteractionRedirect},
	})
	if err != nil {
		t.Fatalf("CreatePayment() failed: %v", err)
	}

	wantURL := "https://bepusdt.example/pay/cashier/BEP-CASHIER-1"
	if result.RedirectURL != wantURL {
		t.Fatalf("RedirectURL = %q, want %q", result.RedirectURL, wantURL)
	}
	data := result.Payload["data"].(map[string]interface{})
	if data["order_mode"] != constants.PaymentBepusdtOrderModeCashier {
		t.Fatalf("order_mode = %v, want cashier", data["order_mode"])
	}
	if _, ok := data["trade_type"]; ok {
		t.Fatalf("trade_type should be empty for cashier order mode")
	}
	if _, ok := data["token"]; ok {
		t.Fatalf("token should be empty for cashier order mode")
	}
	if result.DisplayChannelType != "" {
		t.Fatalf("DisplayChannelType = %q, want empty for cashier order mode", result.DisplayChannelType)
	}
}

func TestBepusdtAdapter_CreatePayment_CashierModeRejectsQR(t *testing.T) {
	a := NewBepusdtAdapter()
	cfg := validBepusdtConfig("https://bepusdt.example")
	cfg["order_mode"] = constants.PaymentBepusdtOrderModeCashier

	_, err := a.CreatePayment(context.Background(), cfg, paymentcontract.GatewayCreateInput{
		OrderNo:     "ORDER-CASHIER-QR",
		Amount:      money.FromDecimal(decimal.RequireFromString("28.88")),
		ChannelType: constants.PaymentProviderBepusdt,
		Extra:       jsonmap.JSON{"interaction_mode": constants.PaymentInteractionQR},
	})
	if !errors.Is(err, paymentcontract.ErrGatewayConfigInvalid) {
		t.Fatalf("expected paymentcontract.ErrGatewayConfigInvalid, got %v", err)
	}
}

func TestBepusdtAdapter_ValidateConfig_CashierModeRequiresCashierChannel(t *testing.T) {
	a := NewBepusdtAdapter()
	cfg := validBepusdtConfig("https://bepusdt.example")
	cfg["order_mode"] = constants.PaymentBepusdtOrderModeCashier

	if err := a.ValidateConfig(cfg, constants.PaymentProviderBepusdt); err != nil {
		t.Fatalf("ValidateConfig() cashier channel failed: %v", err)
	}
	if err := a.ValidateConfig(cfg, constants.PaymentChannelTypeUsdtTrc20); !errors.Is(err, paymentcontract.ErrGatewayUnsupportedChannel) {
		t.Fatalf("expected paymentcontract.ErrGatewayUnsupportedChannel for transaction channel in cashier mode, got %v", err)
	}
}

func TestBepusdtAdapter_MapBepusdtError(t *testing.T) {
	cases := []struct {
		name string
		in   error
		want error
	}{
		{"config", bepusdt.ErrConfigInvalid, paymentcontract.ErrGatewayConfigInvalid},
		{"trade_type→unsupported", bepusdt.ErrTradeTypeNotSupport, paymentcontract.ErrGatewayUnsupportedChannel},
		{"request", bepusdt.ErrRequestFailed, paymentcontract.ErrGatewayRequestFailed},
		{"response", bepusdt.ErrResponseInvalid, paymentcontract.ErrGatewayResponseInvalid},
		{"signature", bepusdt.ErrSignatureInvalid, paymentcontract.ErrGatewaySignatureInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := mapBepusdtError(tc.in)
			if !errors.Is(got, tc.want) {
				t.Fatalf("mapBepusdtError(%v) errors.Is %v = false, want true", tc.in, tc.want)
			}
		})
	}
}

func TestResolveBepusdtTradeLabels(t *testing.T) {
	tests := []struct {
		tradeType string
		chain     string
		tokenID   string
	}{
		{tradeType: "usdt.trc20", chain: "tron", tokenID: "tron-usdt"},
		{tradeType: "usdc.erc20", chain: "ethereum", tokenID: "ethereum-usdc"},
		{tradeType: "usdt.bep20", chain: "bsc", tokenID: "bsc-usdt"},
		{tradeType: "tron.trx", chain: "tron", tokenID: "tron-trx"},
		{tradeType: "ethereum.eth", chain: "ethereum", tokenID: "ethereum-eth"},
		{tradeType: "eth.eth", chain: "ethereum", tokenID: "ethereum-eth"},
		{tradeType: "bsc.bnb", chain: "bsc", tokenID: "bsc-bnb"},
		{tradeType: "ton.gram", chain: "ton", tokenID: "ton-gram"},
		{tradeType: "solana.sol", chain: "solana", tokenID: "solana-sol"},
		{tradeType: "aptos.apt", chain: "aptos", tokenID: "aptos-apt"},
		{tradeType: "usdt.arbitrum", chain: "arbitrum", tokenID: "arbitrum-usdt"},
		{tradeType: "usdc.solana", chain: "solana", tokenID: "solana-usdc"},
		{tradeType: "usdt.x-layer", chain: "x-layer", tokenID: "x-layer-usdt"},
	}
	for _, tc := range tests {
		t.Run(tc.tradeType, func(t *testing.T) {
			chain, tokenID := resolveBepusdtTradeLabels(tc.tradeType)
			if chain != tc.chain || tokenID != tc.tokenID {
				t.Fatalf("resolveBepusdtTradeLabels(%q) = (%q, %q), want (%q, %q)", tc.tradeType, chain, tokenID, tc.chain, tc.tokenID)
			}
		})
	}
}

func validBepusdtConfig(gatewayURL string) jsonmap.JSON {
	return jsonmap.JSON{
		"gateway_url": gatewayURL,
		"auth_token":  "token-001",
		"trade_type":  "usdt.trc20",
		"fiat":        "CNY",
		"notify_url":  "https://api.example.com/api/v1/payments/callback",
		"return_url":  "https://example.com/pay",
	}
}

func newBepusdtCreatePaymentServer(t *testing.T, wantTradeType string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/order/create-transaction" {
			t.Fatalf("path = %s, want /api/v1/order/create-transaction", r.URL.Path)
		}
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		if payload["trade_type"] != wantTradeType {
			t.Fatalf("trade_type = %v, want %s", payload["trade_type"], wantTradeType)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"status_code": 200,
			"message": "success",
			"data": {
				"fiat": "CNY",
				"trade_id": "BEP-1",
				"order_id": "ORDER-1",
				"amount": "28.88",
				"actual_amount": "4.25",
				"token": "TR7NHqjeKQxGTCi8q8ZY4pL8otSzgjLj6t",
				"expiration_time": 1200,
				"status": 1,
				"payment_url": "https://bepusdt.example/pay/checkout-counter/BEP-1"
			}
		}`))
	}))
}
