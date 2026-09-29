package adminauthzhttp

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestBuildAuthzPolicyAuditRecord(t *testing.T) {
	gin.SetMode(gin.TestMode)

	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set("admin_id", uint(7))
	c.Set("username", " admin ")
	c.Set("request_id", " req-1 ")

	req := authzPolicyPayload{
		Role:   "operator",
		Object: "orders",
		Action: " get ",
	}

	got := buildAuthzPolicyAuditRecord(c, req, "policy_grant")

	if got.OperatorAdminID != 7 {
		t.Fatalf("OperatorAdminID = %d, want 7", got.OperatorAdminID)
	}
	if got.OperatorUsername != "admin" {
		t.Fatalf("OperatorUsername = %q, want %q", got.OperatorUsername, "admin")
	}
	if got.Action != "policy_grant" {
		t.Fatalf("Action = %q, want %q", got.Action, "policy_grant")
	}
	if got.Role != req.Role {
		t.Fatalf("Role = %q, want %q", got.Role, req.Role)
	}
	if got.Object != req.Object {
		t.Fatalf("Object = %q, want %q", got.Object, req.Object)
	}
	if got.Method != req.Action {
		t.Fatalf("Method = %q, want %q", got.Method, req.Action)
	}
	if got.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want %q", got.RequestID, "req-1")
	}
	if got.Detail["role"] != req.Role {
		t.Fatalf("Detail role = %v, want %q", got.Detail["role"], req.Role)
	}
	if got.Detail["object"] != req.Object {
		t.Fatalf("Detail object = %v, want %q", got.Detail["object"], req.Object)
	}
	if got.Detail["method"] != "GET" {
		t.Fatalf("Detail method = %v, want %q", got.Detail["method"], "GET")
	}
}

func TestBuildAuthzRoleItemsUsesBackendImmutableSeeds(t *testing.T) {
	items := buildAuthzRoleItems([]string{"role:readonly_auditor", "role:custom_billing"})
	if len(items) != 2 {
		t.Fatalf("items length = %d, want 2", len(items))
	}
	if items[0].Role != "role:readonly_auditor" || !items[0].Immutable {
		t.Fatalf("builtin role metadata = %+v, want immutable", items[0])
	}
	if items[1].Role != "role:custom_billing" || items[1].Immutable {
		t.Fatalf("custom role metadata = %+v, want mutable", items[1])
	}
}

func TestBuildAuthzRoleListPayloadKeepsLegacyStringShape(t *testing.T) {
	roles := []string{"role:readonly_auditor", "role:custom_billing"}
	legacy, ok := buildAuthzRoleListPayload(roles, false).([]string)
	if !ok || len(legacy) != 2 || legacy[0] != roles[0] {
		t.Fatalf("legacy payload = %#v, want string role array", legacy)
	}
	metadata, ok := buildAuthzRoleListPayload(roles, true).([]AuthzRoleItem)
	if !ok || len(metadata) != 2 || !metadata[0].Immutable || metadata[1].Immutable {
		t.Fatalf("metadata payload = %#v, want role descriptors", metadata)
	}
}
