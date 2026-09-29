package adgateway

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientRenderSlotPreservesPathQueryAndResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method got %s want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/public/ad-slots/dashboard/render" {
			t.Errorf("path got %q", r.URL.Path)
		}
		if r.URL.Query().Get("locale") != "zh-CN" || r.URL.Query().Get("tenant") != "admin" {
			t.Errorf("query was not preserved: %v", r.URL.Query())
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"status_code":0,"msg":"","data":{"slot":{"code":"dashboard","scene":"admin","layout":"banner","render_mode":"list","max_items":1},"items":[{"id":7,"title":"hello"}]}}`)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.Client(), server.URL+"/")
	response, err := client.RenderSlot(context.Background(), "dashboard", map[string]string{
		"locale": "zh-CN",
		"tenant": "admin",
	})
	if err != nil {
		t.Fatalf("render slot failed: %v", err)
	}
	if response == nil || response.Slot.Code != "dashboard" || len(response.Items) != 1 || response.Items[0].ID != 7 {
		t.Fatalf("unexpected render response: %#v", response)
	}
}

func TestClientReportImpressionPreservesJSONPayload(t *testing.T) {
	want := json.RawMessage(`{"impression_token":"token-1"}`)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method got %s want POST", r.Method)
		}
		if r.URL.Path != "/api/v1/public/ad-events/impression" {
			t.Errorf("path got %q", r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type got %q", r.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		} else if string(body) != string(want) {
			t.Errorf("body got %s want %s", body, want)
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.Client(), server.URL)
	if err := client.ReportImpression(context.Background(), want); err != nil {
		t.Fatalf("report impression failed: %v", err)
	}
}

func TestClientRejectsNonSuccessGatewayResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)

	client := NewClient(server.Client(), server.URL)
	if _, err := client.RenderSlot(context.Background(), "dashboard", nil); err == nil {
		t.Fatal("render slot must reject non-success gateway response")
	}
	if err := client.ReportImpression(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("report impression must reject non-success gateway response")
	}
}
