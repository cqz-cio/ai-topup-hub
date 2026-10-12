package upstream

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestDisabledPurchasesNeverContactSupplier(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "a disabled purchase must not contact the supplier", http.StatusInternalServerError)
	}))
	defer server.Close()
	request := CreateUpstreamOrderReq{SKUID: 4, Quantity: 1, DownstreamOrderNo: "local-test", MaxAmount: "33.00", Currency: "CNY"}
	for _, value := range []string{"false", "0", "", "misspelled"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("UPSTREAM_PURCHASE_ENABLED", value)
			adapters := []Adapter{
				&ACGFakaAdapter{baseURL: server.URL, client: server.Client()},
				&DujiaoNextAdapter{baseURL: server.URL, client: server.Client()},
			}
			for _, adapter := range adapters {
				result, err := adapter.CreateOrder(context.Background(), request)
				if err != nil || result == nil || result.OK || result.ErrorCode != "purchase_disabled" {
					t.Fatalf("disabled purchase was not rejected: result=%+v err=%v", result, err)
				}
			}
		})
	}
	if calls.Load() != 0 {
		t.Fatalf("disabled deployment made %d supplier requests", calls.Load())
	}
}

func TestDisabledPurchasesStillAllowInventoryAndBalanceReads(t *testing.T) {
	t.Setenv("UPSTREAM_PURCHASE_ENABLED", "false")
	fixture := &acgFixture{balance: "100.00", price: "33.00", stock: "8", config: []any{}, widget: []any{}}
	adapter := testACGFakaAdapter(t, fixture)
	product, err := adapter.GetProduct(context.Background(), 4)
	if err != nil || product == nil || product.SKUs[0].StockQuantity != 8 {
		t.Fatalf("inventory read failed with purchases disabled: product=%+v err=%v", product, err)
	}
	account, err := adapter.Ping(context.Background())
	if err != nil || account == nil || account.Balance != "100.00" || fixture.tradeCount != 0 {
		t.Fatalf("balance read failed or a purchase was submitted: account=%+v err=%v trades=%d", account, err, fixture.tradeCount)
	}
}
