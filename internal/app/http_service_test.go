package app

import (
	"net/http"
	"testing"
)

func TestNewHTTPServiceConfiguresDefensiveTimeouts(t *testing.T) {
	service := NewHTTPService(":0", http.NotFoundHandler())
	if service.server.ReadHeaderTimeout <= 0 ||
		service.server.ReadTimeout <= 0 ||
		service.server.WriteTimeout <= 0 ||
		service.server.IdleTimeout <= 0 {
		t.Fatalf("HTTP server timeouts must all be configured: %+v", service.server)
	}
	if service.server.MaxHeaderBytes != httpMaxHeaderBytes {
		t.Fatalf("MaxHeaderBytes = %d, want %d", service.server.MaxHeaderBytes, httpMaxHeaderBytes)
	}
}
