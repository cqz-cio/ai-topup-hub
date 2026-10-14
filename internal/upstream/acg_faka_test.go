package upstream

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	reconciliationapp "github.com/dujiao-next/internal/modules/reconciliation/application"
	reconciliationcontract "github.com/dujiao-next/internal/modules/reconciliation/contract"
	reconciliationdomain "github.com/dujiao-next/internal/modules/reconciliation/domain"
	reconciliationstore "github.com/dujiao-next/internal/modules/reconciliation/infrastructure/gormstore"
	siteconnectiondomain "github.com/dujiao-next/internal/modules/siteconnection/domain"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/glebarez/sqlite"
	"github.com/shopspring/decimal"
	"golang.org/x/time/rate"
	"gorm.io/gorm"
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
	queryResponse         any
	stateResponse         any
	stateCount            int
	queryCount            int
	stateHTTP             int
}

func testACGFakaAdapter(t *testing.T, fixture *acgFixture) *ACGFakaAdapter {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
			return
		}
		if r.URL.Path != "/user/api/order/state" && r.PostForm.Get("app_id") != "1039" {
			t.Error("missing merchant authentication")
		}
		if _, transmitted := r.PostForm["app_key"]; transmitted {
			t.Error("merchant secret was transmitted in the request")
		}
		if r.URL.Path != "/user/api/order/state" && r.PostForm.Get("sign") != signACGFaka(r.PostForm, "TESTKEY") {
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
			fixture.queryCount++
			fixture.queryNo = r.URL.Query().Get("tradeNo")
			if fixture.queryResponse != nil {
				json.NewEncoder(w).Encode(fixture.queryResponse)
				return
			}
			data = map[string]any{"status": 1, "secret": "CARD-ONE"}
		case "/user/api/order/state":
			fixture.stateCount++
			if fixture.queryCount == 0 || len(r.PostForm) != 1 || r.PostForm.Get("tradeNo") != fixture.queryNo || r.URL.RawQuery != "" {
				t.Error("amount query must follow ownership verification and send only the same reference")
			}
			if fixture.stateHTTP != 0 {
				w.WriteHeader(fixture.stateHTTP)
				return
			}
			json.NewEncoder(w).Encode(fixture.stateResponse)
			return
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
	adapter.amountLimiter = rate.NewLimiter(rate.Inf, 1)
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
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "true") // Purchases terminate at the test TLS server.
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
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "true")
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
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "true")
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
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "true")
	fixture := &acgFixture{balance: "100", price: "33.00", stock: "8", htmlTrade: true}
	adapter := testACGFakaAdapter(t, fixture)
	result, err := adapter.CreateOrder(context.Background(), CreateUpstreamOrderReq{SKUID: 4, Quantity: 1, DownstreamOrderNo: "DJ-1", MaxAmount: "33.00", Currency: "CNY"})
	if err == nil || result != nil || fixture.tradeCount != 1 {
		t.Fatalf("ambiguous charge was treated as a safe rejection: %#v %v", result, err)
	}
}

func TestACGFakaPurchaseResponseClassification(t *testing.T) {
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "true")
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

func TestACGFakaReconciliationUsesOriginalPaidAmount(t *testing.T) {
	for _, amount := range []any{"107.50", 107.5, "0.00", "99999999999999.99"} {
		t.Run(fmt.Sprint(amount), func(t *testing.T) {
			const reference = "001234567890123456789012345678"
			fixture := &acgFixture{price: "999", stateResponse: map[string]any{"code": 200, "data": map[string]any{"trade_no": reference, "amount": amount, "status": 1}}}
			adapter := testACGFakaAdapter(t, fixture)
			detail, err := adapter.GetOrderWithAmountByReference(context.Background(), reference)
			if err != nil || detail.OrderNo != reference || detail.Amount == "" || detail.Currency != "CNY" || detail.Status != "delivered" || fixture.stateCount != 1 || fixture.tradeCount != 0 {
				t.Fatalf("unexpected reconciliation: detail=%+v error=%v state=%d trades=%d", detail, err, fixture.stateCount, fixture.tradeCount)
			}
			if amount == 107.5 && detail.Amount != "107.50" {
				t.Fatalf("amount did not come from original order: %s", detail.Amount)
			}
		})
	}
}

func TestACGFakaReconciliationRejectsUnverifiableAmount(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(map[string]any)
	}{
		{"missing amount", func(d map[string]any) { delete(d, "amount") }},
		{"null amount", func(d map[string]any) { d["amount"] = nil }},
		{"blank amount", func(d map[string]any) { d["amount"] = "" }},
		{"negative amount", func(d map[string]any) { d["amount"] = "-1" }},
		{"invalid amount", func(d map[string]any) { d["amount"] = "unknown" }},
		{"fractional cent", func(d map[string]any) { d["amount"] = "107.501" }},
		{"object amount", func(d map[string]any) { d["amount"] = map[string]int{"value": 107} }},
		{"oversized amount", func(d map[string]any) { d["amount"] = strings.Repeat("9", 200) }},
		{"missing reference", func(d map[string]any) { delete(d, "trade_no") }},
		{"different reference", func(d map[string]any) { d["trade_no"] = "another-order" }},
		{"lost leading zero", func(d map[string]any) { d["trade_no"] = 123 }},
		{"missing paid state", func(d map[string]any) { delete(d, "status") }},
		{"unpaid state", func(d map[string]any) { d["status"] = 0 }},
		{"invalid state", func(d map[string]any) { d["status"] = 2 }},
		{"different currency", func(d map[string]any) { d["currency"] = "USD" }},
		{"conflicting currency code", func(d map[string]any) { d["currency"] = "CNY"; d["currency_code"] = "USD" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data := map[string]any{"trade_no": "00123", "amount": "107.50", "status": 1}
			tc.change(data)
			fixture := &acgFixture{stateResponse: map[string]any{"code": 200, "data": data}}
			adapter := testACGFakaAdapter(t, fixture)
			if detail, err := adapter.GetOrderWithAmountByReference(context.Background(), "00123"); err == nil || detail != nil || fixture.tradeCount != 0 {
				t.Fatalf("unverified amount accepted: %+v %v", detail, err)
			}
		})
	}
}

func TestACGFakaAmountQueryFailureDoesNotBlockDelivery(t *testing.T) {
	for _, httpStatus := range []int{http.StatusTooManyRequests, http.StatusNotFound, http.StatusTemporaryRedirect} {
		t.Run(fmt.Sprint(httpStatus), func(t *testing.T) {
			fixture := &acgFixture{stateHTTP: httpStatus}
			adapter := testACGFakaAdapter(t, fixture)
			if detail, err := adapter.GetOrderWithAmountByReference(context.Background(), "00123"); err == nil || detail != nil {
				t.Fatalf("query failure was accepted: %+v %v", detail, err)
			}
			detail, err := adapter.GetOrderByReference(context.Background(), "00123")
			if err != nil || detail.Status != "delivered" || fixture.stateCount != 1 || fixture.tradeCount != 0 {
				t.Fatalf("delivery polling depended on amount query: %+v %v", detail, err)
			}
		})
	}
}

func TestACGFakaAmountLookupRequiresMerchantOwnership(t *testing.T) {
	fixture := &acgFixture{queryResponse: map[string]any{"code": 0, "msg": "order does not exist"}}
	adapter := testACGFakaAdapter(t, fixture)
	if detail, err := adapter.GetOrderWithAmountByReference(context.Background(), "00123"); err == nil || detail != nil || fixture.stateCount != 0 {
		t.Fatalf("public amount read without authenticated ownership: %+v %v", detail, err)
	}
}

func TestACGFakaAmountRateLimitRespectsDeadline(t *testing.T) {
	fixture := &acgFixture{}
	adapter := testACGFakaAdapter(t, fixture)
	adapter.amountLimiter = rate.NewLimiter(rate.Every(time.Hour), 1)
	adapter.amountLimiter.Allow()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if detail, err := adapter.GetOrderWithAmountByReference(ctx, "00123"); err == nil || detail != nil || fixture.stateCount != 0 {
		t.Fatalf("deadline did not stop rate-limited lookup: %+v %v", detail, err)
	}
}

type acgReconciliationFlow struct {
	adapter *ACGFakaAdapter
	queued  uint
}

func (f *acgReconciliationFlow) Enqueue(id uint) error { f.queued = id; return nil }
func (f *acgReconciliationFlow) Open(uint) (reconciliationcontract.UpstreamOrderReader, error) {
	return f, nil
}
func (f *acgReconciliationFlow) ListByConnectionAndTimeRange(uint, time.Time, time.Time) ([]reconciliationcontract.ProcurementOrder, error) {
	return []reconciliationcontract.ProcurementOrder{{ID: 7, UpstreamOrderNo: "00123", Status: constants.ProcurementStatusFulfilled, UpstreamAmount: money.FromDecimal(decimal.RequireFromString("107.50")), Currency: "CNY"}}, nil
}
func (f *acgReconciliationFlow) Get(ctx context.Context, _ uint, ref string) (*reconciliationcontract.UpstreamOrder, error) {
	detail, err := f.adapter.GetOrderByReference(ctx, ref)
	if err != nil {
		return nil, err
	}
	return &reconciliationcontract.UpstreamOrder{Status: detail.Status, Amount: detail.Amount, Currency: detail.Currency}, nil
}
func (f *acgReconciliationFlow) GetWithAmount(ctx context.Context, _ uint, ref string) (*reconciliationcontract.UpstreamOrder, error) {
	detail, err := f.adapter.GetOrderWithAmountByReference(ctx, ref)
	if err != nil {
		return nil, err
	}
	return &reconciliationcontract.UpstreamOrder{Status: detail.Status, Amount: detail.Amount, Currency: detail.Currency}, nil
}

func TestACGFakaFinancialReconciliationPersistsVerifiedResults(t *testing.T) {
	for _, tc := range []struct {
		name, amount         string
		incomplete, mismatch bool
	}{
		{"match", "107.50", false, false},
		{"one cent discrepancy", "107.51", false, true},
		{"missing amount", "", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture := &acgFixture{stateResponse: map[string]any{"code": 200, "data": map[string]any{"trade_no": "00123", "amount": tc.amount, "status": 1}}}
			flow := &acgReconciliationFlow{adapter: testACGFakaAdapter(t, fixture)}
			db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "reconciliation.db")), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			sqlDB, _ := db.DB()
			t.Cleanup(func() { sqlDB.Close() })
			if err := db.AutoMigrate(&siteconnectiondomain.Connection{}, &reconciliationdomain.Job{}, &reconciliationdomain.Item{}); err != nil {
				t.Fatal(err)
			}
			if err := db.Create(&siteconnectiondomain.Connection{ID: 1, Name: "Mock supplier", BaseURL: flow.adapter.baseURL, Protocol: constants.ConnectionProtocolACGFaka}).Error; err != nil {
				t.Fatal(err)
			}
			jobs, items := reconciliationstore.NewJobStore(db), reconciliationstore.NewItemStore(db)
			svc := reconciliationapp.NewService(reconciliationapp.Options{Jobs: jobs, Items: items, Procurements: flow, Upstream: flow, Queue: flow})
			job, err := svc.CreateAndEnqueue(reconciliationcontract.RunInput{ConnectionID: 1, Type: constants.ReconciliationTypeFull, TimeRangeStart: time.Now().Add(-time.Hour), TimeRangeEnd: time.Now()})
			if err != nil || job.ID != flow.queued {
				t.Fatalf("enqueue: %v", err)
			}
			err = svc.Execute(context.Background(), flow.queued)
			if errors.Is(err, reconciliationcontract.ErrIncompleteResults) != tc.incomplete {
				t.Fatalf("execute: %v", err)
			}
			stored, err := jobs.GetByID(job.ID)
			if err != nil || stored.TotalCount != 1 || stored.FinishedAt == nil {
				t.Fatalf("result not persisted: %+v %v", stored, err)
			}
			if tc.incomplete {
				if stored.Status != constants.ReconciliationJobStatusFailed || stored.MatchedCount != 0 {
					t.Fatalf("unchecked lookup matched: %+v", stored)
				}
			} else if stored.Status != constants.ReconciliationJobStatusCompleted {
				t.Fatalf("valid lookup failed: %+v", stored)
			}
			rows, count, err := items.ListByJobID(job.ID, 1, 10)
			if err != nil {
				t.Fatal(err)
			}
			if tc.mismatch {
				if count != 1 || rows[0].MismatchType != constants.MismatchTypeAmount || !rows[0].LocalAmount.Equal(decimal.RequireFromString("107.50")) || !rows[0].UpstreamAmount.Equal(decimal.RequireFromString("107.51")) {
					t.Fatalf("wrong persisted discrepancy: %+v", rows)
				}
			} else if count != 0 {
				t.Fatalf("unexpected discrepancies: %+v", rows)
			}
			if fixture.tradeCount != 0 {
				t.Fatal("reconciliation submitted a purchase")
			}
		})
	}
}
