package siteurl

import (
	"net/http/httptest"
	"testing"
)

func TestResolvePublicOrigin(t *testing.T) {
	for _, tt := range []struct {
		name, configured, requestURL, proto, forwardedHost, want string
	}{
		{"production stale preview URL", "http://127.0.0.1:4170", "http://www.qzweb.cloud/config", "https", "", "https://www.qzweb.cloud"},
		{"TLS fallback", "", "https://www.qzweb.cloud/config", "", "", "https://www.qzweb.cloud"},
		{"local preview port", "http://127.0.0.1:4170", "http://127.0.0.1:5173/config", "", "", "http://127.0.0.1:5173"},
		{"configured canonical", " https://www.qzweb.cloud/ ", "http://127.0.0.1:5173/config", "", "", "https://www.qzweb.cloud"},
		{"custom storefront", "https://shop.example///", "https://alias.example/config", "", "", "https://shop.example"},
		{"private address", "http://192.168.0.2:4170", "https://www.qzweb.cloud/config", "", "", "https://www.qzweb.cloud"},
		{"IPv6 loopback", "http://[::1]:4170", "https://www.qzweb.cloud/config", "", "", "https://www.qzweb.cloud"},
		{"localhost subdomain", "http://preview.localhost:4170", "https://www.qzweb.cloud/config", "", "", "https://www.qzweb.cloud"},
		{"credentials", "https://user:pass@example.com", "https://www.qzweb.cloud/config", "", "", "https://www.qzweb.cloud"},
		{"not an origin", "https://example.com/shop?token=x#top", "https://www.qzweb.cloud/config", "", "", "https://www.qzweb.cloud"},
		{"invalid scheme", "javascript:alert(1)", "https://www.qzweb.cloud/config", "", "", "https://www.qzweb.cloud"},
		{"forwarded host ignored", "", "https://www.qzweb.cloud/config", "garbage", "evil.example", "https://www.qzweb.cloud"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.requestURL, nil)
			req.Header.Set("X-Forwarded-Proto", tt.proto)
			req.Header.Set("X-Forwarded-Host", tt.forwardedHost)
			if got := Resolve(tt.configured, req); got != tt.want {
				t.Fatalf("Resolve() = %q, want %q", got, tt.want)
			}
		})
	}
}
