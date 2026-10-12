package upstream

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
)

func TestACGFakaSignatureMatchesPHPDecodedSortedForm(t *testing.T) {
	values := url.Values{"app_id": {"1039"}, "contact": {"a+b@example.com"}, "num": {"1"}, "shared_code": {"货品"}, "race": {""}, "sign": {"discard"}}
	// This is the exact text produced by PHP's ksort/http_build_query/urldecode.
	canonical := "app_id=1039&contact=a+b@example.com&num=1&shared_code=货品&key=TESTKEY"
	digest := md5.Sum([]byte(canonical))
	if actual := signACGFaka(values, "TESTKEY"); actual != hex.EncodeToString(digest[:]) {
		t.Fatalf("signature does not match PHP: %s", actual)
	}
}

type acgFixture struct {
	balance, price, stock string
	config, widget        any
	delivery              int
	tradeCount            int
	tradeResponse         any
	htmlTrade             bool
	requestNo             string
	queryNo               string
}

func testACGFakaAdapter(t *testing.T, fixture *acgFixture) *ACGFakaAdapter {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			return
		}
		if r.PostForm.Get("app_id") != "1039" {
			t.Error("missing merchant authentication")
		}
		if _, transmitted := r.PostForm["app_key"]; transmitted {
			t.Error("merchant secret was transmitted in the request")
		}
		if r.PostForm.Get("sign") != signACGFaka(r.PostForm, "TESTKEY") {
			t.Error("invalid signature")
		}
		w.Header().Set("Content-Type", "application/json")
		product := map[string]any{"id": 4, "category_id": 4, "code": "ellaix-go", "name": "Go 兑换码", "description": "<p>兑换说明</p>", "cover": "/go.svg", "price": 40, "user_price": 40, "factory_price": 33, "stock": fixture.stock, "status": 1, "delivery_way": fixture.delivery, "config": fixture.config, "widget": fixture.widget, "minimum": 1, "maximum": 20}
		var data any
		switch r.URL.Path {
		case "/shared/authentication/connect":
			data = map[string]any{"shopName": "ELLAIXSHOP", "balance": fixture.balance}
		case "/shared/commodity/items":
			data = []any{map[string]any{"id": 4, "pid": 0, "name": "ChatGPT", "children": []any{product}}}
		case "/shared/commodity/item":
			data = product
		case "/shared/commodity/valuation":
			data = map[string]any{"price": fixture.price, "currency_code": "CNY"}
		case "/shared/commodity/stock":
			data = map[string]any{"stock": fixture.stock}
		case "/shared/commodity/trade":
			fixture.tradeCount++
			fixture.requestNo = r.PostForm.Get("request_no")
			if r.PostForm.Get("shared_code") != "ellaix-go" || r.PostForm.Get("num") != "1" || r.PostForm.Get("contact") != "orders@zhixiang.invalid" {
				t.Error("invalid purchase or leaked customer contact")
			}
			if fixture.htmlTrade {
				fmt.Fprint(w, "<html>404</html>")
				return
			}
			if fixture.tradeResponse != nil {
				json.NewEncoder(w).Encode(fixture.tradeResponse)
				return
			}
			data = map[string]any{"tradeNo": "001234567890123456", "amount": "33.00", "secret": "CARD-ONE\n兑换说明"}
		case "/shared/commodity/query":
			fixture.queryNo = r.URL.Query().Get("tradeNo")
			data = map[string]any{"status": 1, "secret": "CARD-ONE"}
		default:
			t.Errorf("unexpected supplier endpoint %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": data})
	}))
	t.Cleanup(server.Close)
	adapter, err := NewACGFakaAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, ApiKey: "1039", ApiSecret: "TESTKEY"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	adapter.client.Transport = server.Client().Transport
	return adapter
}

func TestACGFakaPostKeepsMerchantSecretLocal(t *testing.T) {
	adapter := testACGFakaAdapter(t, &acgFixture{balance: "100"})
	// Even a stale caller-supplied app_key must be removed before signing and sending.
	if err := adapter.post(context.Background(), "/shared/authentication/connect", url.Values{"app_key": {"TESTKEY"}}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestACGFakaMerchantPriceInventoryAndImmediateCard(t *testing.T) {
	fixture := &acgFixture{balance: "100.00", price: "33.00", stock: "8", config: []any{}, widget: []any{}}
	adapter := testACGFakaAdapter(t, fixture)
	product, err := adapter.GetProduct(context.Background(), 4)
	if err != nil {
		t.Fatal(err)
	}
	if product.PriceAmount != "33.00" || len(product.SKUs) != 1 || product.SKUs[0].StockQuantity != 8 || product.SKUs[0].ID != 4 {
		t.Fatalf("wrong merchant product: %#v", product)
	}
	result, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{SKUID: 4, Quantity: 1, DownstreamOrderNo: "DJ-1", TraceID: "trace-1", MaxAmount: "33.00", Currency: "CNY"})
	if err != nil {
		t.Fatal(err)
	}
	if !result.OK || result.OrderNo != "001234567890123456" || result.Fulfillment == nil || result.Fulfillment.Payload != "CARD-ONE\n兑换说明" || fixture.tradeCount != 1 {
		t.Fatalf("invalid purchase: %#v", result)
	}
	if len(fixture.requestNo) != 52 || !strings.HasPrefix(fixture.requestNo, "zxa-") {
		t.Fatalf("missing persistent request reference: %s", fixture.requestNo)
	}
	detail, err := adapter.GetOrderByReference(context.Background(), result.OrderNo)
	if err != nil || detail.Status != "delivered" || fixture.queryNo != result.OrderNo {
		t.Fatalf("order number was not preserved: %#v %v", detail, err)
	}
}

func TestACGFakaPreflightDoesNotConsumeBalance(t *testing.T) {
	tests := []struct {
		name, price, balance, stock string
		config, widget              any
		quantity, delivery          int
		want                        string
	}{
		{name: "zero balance", price: "33.00", balance: "0", stock: "8", quantity: 1, want: "insufficient_balance"},
		{name: "price changed", price: "34.00", balance: "100", stock: "8", quantity: 1, want: "upstream_price_changed"},
		{name: "sold out", price: "33.00", balance: "100", stock: "0", quantity: 1, want: "product_out_of_stock"},
		{name: "quantity limit", price: "33.00", balance: "100", stock: "100", quantity: 21, want: "invalid_request"},
		{name: "variant unsupported", price: "33.00", balance: "100", stock: "8", quantity: 1, config: map[string]any{"category": map[string]any{"Plus": 100}}, want: "product_unavailable"},
		{name: "extra form unsupported", price: "33.00", balance: "100", stock: "8", quantity: 1, widget: []any{map[string]any{"name": "session"}}, want: "product_unavailable"},
		{name: "manual goods", price: "33.00", balance: "100", stock: "8", quantity: 1, delivery: 1, want: "product_unavailable"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := &acgFixture{balance: test.balance, price: test.price, stock: test.stock, config: test.config, widget: test.widget, delivery: test.delivery}
			adapter := testACGFakaAdapter(t, fixture)
			result, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{SKUID: 4, Quantity: test.quantity, DownstreamOrderNo: "DJ-1", MaxAmount: "33.00", Currency: "CNY"})
			if err != nil || result.OK || result.ErrorCode != test.want || fixture.tradeCount != 0 {
				t.Fatalf("unsafe preflight: result=%#v err=%v purchases=%d", result, err, fixture.tradeCount)
			}
		})
	}
}

func TestACGFakaRechecksBalanceAfterMerchantTopup(t *testing.T) {
	fixture := &acgFixture{balance: "0.00", price: "33.00", stock: "8", config: []any{}, widget: []any{}}
	adapter := testACGFakaAdapter(t, fixture)
	request := CreateUpstreamOrderReq{SKUID: 4, Quantity: 1, DownstreamOrderNo: "DJ-TOPUP-1", MaxAmount: "33.00", Currency: "CNY"}
	result, err := adapter.CreateOrder(context.Background(), request)
	if err != nil || result == nil || result.OK || result.ErrorCode != "insufficient_balance" || fixture.tradeCount != 0 {
		t.Fatalf("empty balance must not submit a trade: result=%#v err=%v trades=%d", result, err, fixture.tradeCount)
	}
	fixture.balance = "100.00"
	result, err = adapter.CreateOrder(context.Background(), request)
	if err != nil || result == nil || !result.OK || result.Fulfillment == nil || result.Fulfillment.Payload != "CARD-ONE\n兑换说明" || fixture.tradeCount != 1 {
		t.Fatalf("funded in-stock purchase should deliver one card: result=%#v err=%v trades=%d", result, err, fixture.tradeCount)
	}
}

func TestACGFakaMalformedPurchaseResponseIsUncertain(t *testing.T) {
	fixture := &acgFixture{balance: "100", price: "33.00", stock: "8", htmlTrade: true}
	adapter := testACGFakaAdapter(t, fixture)
	result, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{SKUID: 4, Quantity: 1, DownstreamOrderNo: "DJ-1", MaxAmount: "33.00", Currency: "CNY"})
	if err == nil || result != nil || fixture.tradeCount != 1 {
		t.Fatalf("ambiguous charge was treated as a safe rejection: %#v %v", result, err)
	}
}

func TestACGFakaPurchaseResponseClassification(t *testing.T) {
	tests := []struct {
		name, code         string
		response           any
		uncertain, delayed bool
	}{
		{name: "duplicate request needs reconciliation", code: "duplicate_order", response: map[string]any{"code": 0, "msg": "The request ID already exists"}},
		{name: "failure after transaction may have charged", uncertain: true, response: map[string]any{"code": 0, "msg": "stock refresh failed TESTKEY"}},
		{name: "missing purchase reference", uncertain: true, response: map[string]any{"code": 200, "data": map[string]any{"amount": "33.00", "secret": "CARD"}}},
		{name: "delayed card keeps supplier reference", delayed: true, response: map[string]any{"code": 200, "data": map[string]any{"tradeNo": "001234567890123456", "amount": "33.00", "secret": ""}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fixture := &acgFixture{balance: "100", price: "33.00", stock: "8", tradeResponse: test.response}
			adapter := testACGFakaAdapter(t, fixture)
			result, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{SKUID: 4, Quantity: 1, DownstreamOrderNo: "DJ-1", MaxAmount: "33.00", Currency: "CNY"})
			if fixture.tradeCount != 1 {
				t.Fatalf("purchase request count=%d", fixture.tradeCount)
			}
			if test.uncertain {
				if err == nil || result != nil || strings.Contains(err.Error(), "TESTKEY") {
					t.Fatalf("ambiguous result or credential exposure: %#v %v", result, err)
				}
				return
			}
			if err != nil || result == nil {
				t.Fatalf("unexpected response: %#v %v", result, err)
			}
			if test.delayed {
				if !result.OK || result.Fulfillment != nil || result.OrderNo != "001234567890123456" {
					t.Fatalf("delayed fulfillment lost its reference: %#v", result)
				}
				return
			}
			if result.OK || result.ErrorCode != test.code {
				t.Fatalf("wrong business result: %#v", result)
			}
		})
	}
}

func TestACGFakaDoesNotRedirectMerchantCredentials(t *testing.T) {
	var received int
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { received++ }))
	defer destination.Close()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, destination.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	adapter, err := NewACGFakaAdapter(&siteconnectiondomain.Connection{BaseURL: server.URL, ApiKey: "1039", ApiSecret: "TESTKEY"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	adapter.client.Transport = server.Client().Transport
	_, err = adapter.Ping(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP 307") || received != 0 {
		t.Fatalf("credentials were redirected: %v, received %d", err, received)
	}
}
