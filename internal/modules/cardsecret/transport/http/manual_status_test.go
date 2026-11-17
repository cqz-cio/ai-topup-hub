package cardsecrethttp

import (
	"context"
	"encoding/json"
	"github.com/dujiao-next/internal/modules/cardsecret/contract"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

type manualManagementStub struct {
	calls     int
	actor     uint
	operation string
	input     contract.ManagementInput
}

func (m *manualManagementStub) Call(_ context.Context, op string, input contract.ManagementInput, actor uint) (json.RawMessage, error) {
	m.calls++
	m.actor = actor
	m.operation = op
	m.input = input
	return json.RawMessage(`{"card":{"status":"used"}}`), nil
}
func TestManualStatusUsesAuthenticatedActorAndValidatesInput(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name          string
		confirmed     bool
		state         string
		authenticated bool
		want          int
	}{
		{"valid", true, strings.Repeat("a", 64), true, 1},
		{"missing confirmation", false, strings.Repeat("a", 64), true, 0},
		{"missing state", true, "", true, 0},
		{"unauthenticated", true, strings.Repeat("a", 64), false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stub := &manualManagementStub{}
			handler := &AdminHandler{management: stub}
			body, _ := json.Marshal(map[string]any{"card_id": "card-fixture-123456", "request_id": "request-fixture-123456", "status": "used", "reason": "已核对", "confirmed": tc.confirmed, "expected_state": tc.state, "actor": "shop-admin:999"})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("POST", "/", strings.NewReader(string(body)))
			c.Request.Header.Set("Content-Type", "application/json")
			if tc.authenticated {
				c.Set("admin_id", uint(42))
			}
			handler.ManualPAIPLUSCardStatus(c)
			if stub.calls != tc.want {
				t.Fatalf("calls=%d want=%d body=%s", stub.calls, tc.want, w.Body.String())
			}
			if stub.calls > 0 && (stub.actor != 42 || stub.operation != "manual-status" || stub.input.ExpectedState != tc.state) {
				t.Fatalf("untrusted actor or lost state: %+v", stub)
			}
			if !strings.Contains(w.Header().Get("Cache-Control"), "no-store") {
				t.Fatal("private response must not be cached")
			}
		})
	}
}
