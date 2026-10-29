package redemptionclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/procurement/contract"
	"github.com/dujiao-next/internal/modules/procurement/domain"
)

func TestPaidDeliveryScopeAndStableReference(t *testing.T) {
	var refs []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/integration/deliver" || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("t", 32) {
			t.Error("wrong authenticated route")
		}
		var body struct {
			Reference string `json:"reference"`
			Payload   string `json:"payload"`
			Quantity  int    `json:"quantity"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		refs = append(refs, body.Reference)
		if body.Payload != "source-card" || body.Quantity != 1 {
			t.Error("changed supplier delivery")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true,"data":{"format":"paiplus-v1","count":1,"cards":[{"code":"PAIPLUS-ABCDEFGH23-ABCDEFGH45"}],"payload":"supplier text must never be trusted: source-card https://cdk.bar"}}`))
	}))
	defer server.Close()
	before := time.Now().Add(-time.Hour)
	paid := time.Now()
	client, err := New(server.URL, strings.Repeat("t", 32), []uint{30}, 1, 100, before)
	if err != nil {
		t.Fatal(err)
	}
	order := &domain.LocalOrder{ID: 101, PaidAt: &paid, Items: []domain.LocalOrderItem{{ProductID: 30, Quantity: 1}}}
	source := &contract.Fulfillment{Type: "auto", Status: "delivered", Payload: "source-card"}
	for i := 0; i < 2; i++ {
		result, err := client.Rewrite(order, 1, source)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(result.Payload, "PAIPLUS-") {
			t.Fatal("missing local card")
		}
		if strings.Contains(result.Payload, "source-card") || strings.Contains(result.Payload, "cdk.bar") || !strings.Contains(result.Payload, server.URL+"/") {
			t.Fatal("customer delivery was not reconstructed from validated local codes")
		}
	}
	if source.Payload != "source-card" || len(refs) != 2 || refs[0] != refs[1] {
		t.Fatal("rewrote original or changed idempotency reference")
	}
	order.ID = 99
	if result, err := client.Rewrite(order, 1, source); err != nil || result != source {
		t.Fatal("historical order changed")
	}
	order.ID = 101
	order.PaidAt = &before
	old := before.Add(-time.Second)
	order.PaidAt = &old
	_, _ = client.Rewrite(order, 1, source)
	order.PaidAt = &paid
	_, _ = client.Rewrite(order, 2, source)
	order.Items[0].ProductID = 999
	_, _ = client.Rewrite(order, 1, source)
	if len(refs) != 2 {
		t.Fatal("out-of-scope delivery called issuer")
	}
	order.Items[0].ProductID = 30
	order.PaidAt = nil
	if _, err := client.Rewrite(order, 1, source); err == nil {
		t.Fatal("unpaid order accepted")
	}
}

func TestIssuerFailureDoesNotReleaseRawCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "secret supplier card", 502) }))
	defer server.Close()
	paid := time.Now()
	c, _ := New(server.URL, strings.Repeat("t", 32), []uint{30}, 1, 1, paid.Add(-time.Hour))
	order := &domain.LocalOrder{ID: 10, PaidAt: &paid, Items: []domain.LocalOrderItem{{ProductID: 30, Quantity: 1}}}
	f := &contract.Fulfillment{Type: "auto", Payload: "private-source"}
	result, err := c.Rewrite(order, 1, f)
	if result != nil || err == nil || strings.Contains(err.Error(), "secret") || f.Payload != "private-source" {
		t.Fatal("issuer failure did not preserve private source")
	}
}

func TestConfigurationDefaultsOffAndRequiresBounds(t *testing.T) {
	t.Setenv("PAIPLUS_ENABLED", "")
	c, err := FromEnvironment()
	if c != nil || err != nil {
		t.Fatal("must default off")
	}
	t.Setenv("PAIPLUS_ENABLED", "true")
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("incomplete config accepted")
	}
	for _, origin := range []string{"http://remote.example", "https://user:pass@example.com", "https://example.com?secret=x", "https://example.com/path"} {
		if _, err := New(origin, strings.Repeat("t", 32), []uint{30}, 1, 1, time.Now()); err == nil {
			t.Fatal("unsafe origin accepted")
		}
	}
}
