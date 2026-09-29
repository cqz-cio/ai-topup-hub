package paypal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/constants"
)

func TestValidateConfig(t *testing.T) {
	cfg := &Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		BaseURL:      "https://api-m.sandbox.paypal.com",
		ReturnURL:    "https://example.com/payment?order_id=1",
		CancelURL:    "https://example.com/payment?order_id=1",
		WebhookID:    "WH-123456",
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig should pass, got: %v", err)
	}
}

func TestValidateConfigAllowsMissingWebhookID(t *testing.T) {
	cfg := &Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		BaseURL:      "https://api-m.sandbox.paypal.com",
		ReturnURL:    "https://example.com/payment?order_id=1",
		CancelURL:    "https://example.com/payment?order_id=1",
	}
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("ValidateConfig should allow missing webhook_id, got: %v", err)
	}
}

func TestVerifyWebhookSignatureRequiresWebhookID(t *testing.T) {
	cfg := &Config{
		ClientID:     "cid",
		ClientSecret: "secret",
		BaseURL:      "https://api-m.sandbox.paypal.com",
		ReturnURL:    "https://example.com/payment?order_id=1",
		CancelURL:    "https://example.com/payment?order_id=1",
	}

	err := VerifyWebhookSignature(context.Background(), cfg, http.Header{}, []byte(`{}`))
	if !errors.Is(err, ErrConfigInvalid) {
		t.Fatalf("VerifyWebhookSignature should require webhook_id, got: %v", err)
	}
}

func TestVerifyWebhookSignaturePreservesRawEvent(t *testing.T) {
	raw := []byte("{\n  \"id\": \"WH-1\",\n  \"event_type\": \"PAYMENT.CAPTURE.COMPLETED\",\n  \"summary\": \"A&B <x>\"\n}\n")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/oauth2/token":
			_, _ = w.Write([]byte(`{"access_token":"test-token"}`))
		case "/v1/notifications/verify-webhook-signature":
			body, _ := io.ReadAll(r.Body)
			wantSuffix := append([]byte(`"webhook_event":`), raw...)
			wantSuffix = append(wantSuffix, '}')
			if !bytes.HasSuffix(body, wantSuffix) {
				t.Errorf("webhook_event changed:\nrequest: %s\n  event: %s", body, raw)
			}
			_, _ = w.Write([]byte(`{"verification_status":"SUCCESS"}`))
		default:
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	cfg := &Config{ClientID: "client-id", ClientSecret: "client-secret", BaseURL: server.URL, WebhookID: "WH-ID"}
	headers := http.Header{}
	headers.Set("Paypal-Transmission-Id", "transmission-id")
	headers.Set("Paypal-Transmission-Time", "2026-07-29T12:00:00Z")
	headers.Set("Paypal-Cert-Url", "https://api.paypal.com/cert?foo=1&bar=2")
	headers.Set("Paypal-Auth-Algo", "SHA256withRSA")
	headers.Set("Paypal-Transmission-Sig", "signature")

	if err := VerifyWebhookSignature(context.Background(), cfg, headers, raw); err != nil {
		t.Fatalf("VerifyWebhookSignature failed: %v", err)
	}
}

func TestParseConfigAndNormalize(t *testing.T) {
	raw := map[string]interface{}{
		"client_id":     " cid ",
		"client_secret": " secret ",
		"base_url":      "https://api-m.sandbox.paypal.com/",
		"return_url":    "https://example.com/return",
		"cancel_url":    "https://example.com/cancel",
	}
	cfg, err := ParseConfig(raw)
	if err != nil {
		t.Fatalf("ParseConfig error: %v", err)
	}
	if cfg.ClientID != "cid" {
		t.Fatalf("client id not normalized, got: %s", cfg.ClientID)
	}
	if cfg.BaseURL != "https://api-m.sandbox.paypal.com" {
		t.Fatalf("base url not normalized, got: %s", cfg.BaseURL)
	}
	if cfg.UserAction == "" {
		t.Fatalf("user action should have default value")
	}
}

func TestToPaymentStatus(t *testing.T) {
	tests := []struct {
		name           string
		eventType      string
		resourceStatus string
		expectStatus   string
		expectOK       bool
	}{
		{name: "EventCompleted", eventType: paypalEventCaptureCompleted, resourceStatus: "", expectStatus: constants.PaymentStatusSuccess, expectOK: true},
		{name: "EventPending", eventType: paypalEventCapturePending, resourceStatus: "", expectStatus: constants.PaymentStatusPending, expectOK: true},
		{name: "ResourceDeclined", eventType: "", resourceStatus: paypalResourceStatusDeclined, expectStatus: constants.PaymentStatusFailed, expectOK: true},
		{name: "ResourceCreated", eventType: "", resourceStatus: paypalResourceStatusCreated, expectStatus: constants.PaymentStatusPending, expectOK: true},
		{name: "Unknown", eventType: "UNKNOWN", resourceStatus: "UNKNOWN", expectStatus: "", expectOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, ok := ToPaymentStatus(tc.eventType, tc.resourceStatus)
			if ok != tc.expectOK {
				t.Fatalf("unexpected ok: got %v, want %v", ok, tc.expectOK)
			}
			if status != tc.expectStatus {
				t.Fatalf("unexpected status: got %s, want %s", status, tc.expectStatus)
			}
		})
	}
}

func TestWebhookEventHelpers(t *testing.T) {
	event := &WebhookEvent{
		EventType: "PAYMENT.CAPTURE.COMPLETED",
		Resource: map[string]interface{}{
			"supplementary_data": map[string]interface{}{
				"related_ids": map[string]interface{}{
					"order_id": "ORDER-123",
				},
			},
			"amount": map[string]interface{}{
				"value":         "10.00",
				"currency_code": "USD",
			},
			"create_time": "2026-02-09T12:00:00Z",
			"status":      "COMPLETED",
		},
	}
	if got := event.RelatedOrderID(); got != "ORDER-123" {
		t.Fatalf("unexpected order id: %s", got)
	}
	value, currency := event.CaptureAmount()
	if value != "10.00" || currency != "USD" {
		t.Fatalf("unexpected amount info: %s %s", value, currency)
	}
	if event.PaidAt() == nil {
		t.Fatalf("PaidAt should parse time")
	}
	if status := event.ResourceStatus(); status != "COMPLETED" {
		t.Fatalf("unexpected resource status: %s", status)
	}
}

func TestWebhookEventHelpersCaptureAmountFallback(t *testing.T) {
	event := &WebhookEvent{
		EventType: "CHECKOUT.ORDER.COMPLETED",
		Resource: map[string]interface{}{
			"purchase_units": []interface{}{
				map[string]interface{}{
					"amount": map[string]interface{}{
						"value":         "88.66",
						"currency_code": "USD",
					},
				},
			},
		},
	}

	value, currency := event.CaptureAmount()
	if value != "88.66" || currency != "USD" {
		t.Fatalf("unexpected fallback amount info: %s %s", value, currency)
	}
}
