package contract

import (
	"net/http"
	"testing"

	"github.com/dujiao-next/internal/config"
)

func TestNormalizeHost(t *testing.T) {
	cases := map[string]string{
		"Shop.Example.COM":     "shop.example.com",
		"shop.example.com:443": "shop.example.com",
		"shop.example.com.":    "shop.example.com",
		"  LOCALHOST:5173  ":   "localhost",
		"[::1]:8080":           "::1",
	}
	for input, want := range cases {
		if got := NormalizeHost(input); got != want {
			t.Fatalf("NormalizeHost(%q)=%q want %q", input, got, want)
		}
	}
}

func TestResolveRequestHostUsesForwardedHostOnlyWhenTrusted(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "https://internal.example.test/api/v1/public/config", nil)
	if err != nil {
		t.Fatalf("new request failed: %v", err)
	}
	req.Host = "internal.example.test"
	req.Header.Set("X-Forwarded-Host", "Shop.Example.test")
	cfg := config.ResellerConfig{TrustedForwardedHost: false}
	if got := ResolveRequestHost(req, cfg); got != "internal.example.test" {
		t.Fatalf("untrusted forwarded host should use request host, got %s", got)
	}
	cfg.TrustedForwardedHost = true
	if got := ResolveRequestHost(req, cfg); got != "shop.example.test" {
		t.Fatalf("trusted forwarded host should use forwarded host, got %s", got)
	}
}

func TestTenantContextMainAndReseller(t *testing.T) {
	mainCtx := MainTenantContext("main.example.test")
	if !mainCtx.IsMain || mainCtx.ResellerID != nil {
		t.Fatalf("expected main tenant, got %+v", mainCtx)
	}
	id := uint(42)
	resellerCtx := ResellerTenantContext("shop.example.test", id, 100, "primary.example.test")
	if resellerCtx.IsMain || resellerCtx.ResellerID == nil || *resellerCtx.ResellerID != id {
		t.Fatalf("expected reseller tenant, got %+v", resellerCtx)
	}
	if resellerCtx.ResellerUserID != 100 {
		t.Fatalf("reseller user id mismatch: %d", resellerCtx.ResellerUserID)
	}
}
