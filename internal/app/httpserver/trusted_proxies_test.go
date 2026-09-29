package httpserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConfigureTrustedProxiesRejectsSpoofedForwardedIPFromUntrustedPeer(t *testing.T) {
	engine := gin.New()
	if err := configureTrustedProxies(engine, nil); err != nil {
		t.Fatal(err)
	}
	engine.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "203.0.113.9:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Body.String() != "203.0.113.9" {
		t.Fatalf("untrusted peer spoofed client IP: %q", resp.Body.String())
	}
}

func TestConfigureTrustedProxiesAcceptsForwardedIPFromConfiguredProxy(t *testing.T) {
	engine := gin.New()
	if err := configureTrustedProxies(engine, []string{"127.0.0.1/32"}); err != nil {
		t.Fatal(err)
	}
	engine.GET("/ip", func(c *gin.Context) { c.String(http.StatusOK, c.ClientIP()) })

	req := httptest.NewRequest(http.MethodGet, "/ip", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "1.2.3.4")
	resp := httptest.NewRecorder()
	engine.ServeHTTP(resp, req)
	if resp.Body.String() != "1.2.3.4" {
		t.Fatalf("trusted proxy client IP=%q, want 1.2.3.4", resp.Body.String())
	}
}

func TestConfigureTrustedProxiesRejectsInvalidCIDR(t *testing.T) {
	if err := configureTrustedProxies(gin.New(), []string{"not-a-cidr"}); err == nil {
		t.Fatal("expected invalid trusted proxy configuration to fail")
	}
}

func TestConfigureTrustedProxiesRejectsTrustAllNetworks(t *testing.T) {
	for _, network := range []string{"0.0.0.0/0", "::/0"} {
		t.Run(network, func(t *testing.T) {
			if err := configureTrustedProxies(gin.New(), []string{network}); err == nil {
				t.Fatalf("expected %s to be rejected", network)
			}
		})
	}
}
