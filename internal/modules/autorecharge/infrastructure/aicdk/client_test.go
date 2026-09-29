package aicdk

import (
	"context"
	"encoding/json"
	"github.com/dujiao-next/internal/modules/autorecharge/contract"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAsyncAcceptedUsesBearerAndStableUUID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/partner/v1/pay" || r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Idempotency-Key") != "task-uuid" {
			t.Error("headers or route")
		}
		var body contract.PayRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Channel != "3" {
			t.Error("channel")
		}
		w.WriteHeader(202)
		_, _ = w.Write([]byte(`{"ok":false,"pending":true,"operation_id":"string-op","state":"queued"}`))
	}))
	defer srv.Close()
	c := New("test-key")
	c.baseURL = srv.URL
	r, err := c.Pay(context.Background(), "task-uuid", contract.PayRequest{Channel: "3"})
	if err != nil || r.State != "queued" || r.OperationID != "string-op" {
		t.Fatalf("%+v %v", r, err)
	}
}
func TestProviderErrorBodyIsNeverExposed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		_, _ = w.Write([]byte(`{"error":"session=private-card-secret"}`))
	}))
	defer srv.Close()
	c := New("key")
	c.baseURL = srv.URL
	_, err := c.Pay(context.Background(), "id", contract.PayRequest{})
	if err == nil || strings.Contains(err.Error(), "private") {
		t.Fatal("unsafe error")
	}
}
func TestCredentialRequestDoesNotFollowRedirects(t *testing.T) {
	calls := 0
	dest := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls++ }))
	defer dest.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, dest.URL, 307) }))
	defer srv.Close()
	c := New("key")
	c.baseURL = srv.URL
	_, err := c.Pay(context.Background(), "id", contract.PayRequest{})
	if err == nil || calls != 0 {
		t.Fatal("redirect forwarded credentials")
	}
}

func TestAccountPrecheckRequiresProviderIdentityAndSubscription(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{`{"ok":true,"account_id":"account_1","subscription":{"plan_type":"free"}}`, true},
		{`{"ok":true,"subscription":{"plan_type":"free"}}`, false},
		{`{"ok":true,"account_id":"account_1","subscription":null}`, false},
		{`{"ok":true,"pending":true,"account_id":"account_1","subscription":{"plan_type":"free"}}`, false},
		{`{"ok":false,"account_id":"account_1","subscription":{"plan_type":"free"}}`, false},
	} {
		t.Run(tc.body, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != "/api/partner/v1/subscription" {
					t.Error("wrong precheck endpoint")
				}
				var body map[string]string
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["session"] != "authorized-session" {
					t.Error("session mapping")
				}
				_, _ = w.Write([]byte(tc.body))
			}))
			defer srv.Close()
			c := New("test-key")
			c.baseURL = srv.URL
			a, err := c.InspectAccount(context.Background(), "authorized-session")
			if tc.valid && (err != nil || a == nil || a.ID != "account_1" || a.Plan != "free") {
				t.Fatal("valid response rejected")
			}
			if !tc.valid && err == nil {
				t.Fatal("incomplete response accepted")
			}
		})
	}
}
