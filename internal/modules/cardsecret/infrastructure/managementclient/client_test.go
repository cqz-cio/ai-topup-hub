package managementclient

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/modules/cardsecret/contract"
)

func TestManagementUsesSeparateCredentialAndTrustedActor(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/api/management/revoke" || r.URL.RawQuery != "" || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("m", 32) {
			t.Error("wrong management request")
		}
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["actor"] != "shop-admin:42" || body["confirmed"] != true || body["card_id"] != "card-identity-123456" {
			t.Error("lost trusted actor or confirmation")
		}
		_, _ = w.Write([]byte(`{"ok":true,"data":{"destroyed":true,"card":{"status":"revoked"}}}`))
	}))
	defer server.Close()
	client, err := New(server.URL, strings.Repeat("m", 32))
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Call(context.Background(), "revoke", contract.ManagementInput{CardID: "card-identity-123456", Confirmed: true, Reason: "refund", RequestID: "request-identity-123456"}, 42)
	if err != nil || !strings.Contains(string(result), `"revoked"`) {
		t.Fatalf("management failed: %s %v", result, err)
	}
}

func TestManagementErrorsCannotLeakSourceText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(409)
		_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"CARD_INVALID","message":"private-source-card"}}`))
	}))
	defer server.Close()
	client, _ := New(server.URL, strings.Repeat("m", 32))
	_, err := client.Call(context.Background(), "revoke", contract.ManagementInput{}, 1)
	var known *contract.ManagementError
	if !errors.As(err, &known) || !strings.Contains(known.Message, "卡密无效，销毁失败") || strings.Contains(known.Message, "private-source-card") {
		t.Fatalf("unsafe error: %v", err)
	}
}

func TestManagementRedirectDoesNotSendCredentialsToAnotherOrigin(t *testing.T) {
	visited := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { visited = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 307) }))
	defer server.Close()
	client, _ := New(server.URL, strings.Repeat("m", 32))
	if _, err := client.Call(context.Background(), "cards", contract.ManagementInput{}, 1); err == nil || visited {
		t.Fatal("followed management redirect")
	}
}

func TestManagementConfigurationAndActionsFailClosed(t *testing.T) {
	t.Setenv("PAIPLUS_MANAGEMENT_TOKEN", "")
	client, err := FromEnvironment()
	if err != nil || client != nil {
		t.Fatal("management must default off")
	}
	t.Setenv("PAIPLUS_MANAGEMENT_TOKEN", strings.Repeat("m", 32))
	t.Setenv("PAIPLUS_INTEGRATION_TOKEN", strings.Repeat("m", 32))
	if _, err := FromEnvironment(); err == nil {
		t.Fatal("shipping credential reused")
	}
	for _, origin := range []string{"http://external.example", "https://user:pass@example.com", "https://example.com/path", "https://example.com?secret=1"} {
		if _, err := New(origin, strings.Repeat("m", 32)); err == nil {
			t.Fatal("invalid origin accepted")
		}
	}
	c, _ := New("https://example.test", strings.Repeat("m", 32))
	if _, err := c.Call(context.Background(), "../integration/deliver", contract.ManagementInput{}, 1); err == nil {
		t.Fatal("unknown operation accepted")
	}
}
