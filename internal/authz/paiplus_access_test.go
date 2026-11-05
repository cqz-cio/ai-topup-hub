package authz

import "testing"

func TestPAIPLUSManagementRequiresSeparatePermissions(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"operations", "support", "finance", "readonly_auditor", "integration"} {
		if err := svc.SetAdminRoles(51, []string{role}); err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"query", "reveal", "check", "revoke"} {
			allowed, err := svc.EnforceAdmin(51, "/admin/card-secrets/paiplus/"+action, "POST")
			if err != nil || allowed != (role == "operations") {
				t.Fatalf("role=%s action=%s allowed=%v err=%v", role, action, allowed, err)
			}
		}
	}
	if err := svc.GrantRolePolicy("paiplus_viewer", "/admin/card-secrets/paiplus/query", "POST"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetAdminRoles(52, []string{"paiplus_viewer"}); err != nil {
		t.Fatal(err)
	}
	for _, action := range []string{"reveal", "check", "revoke"} {
		allowed, err := svc.EnforceAdmin(52, "/admin/card-secrets/paiplus/"+action, "POST")
		if err != nil || allowed {
			t.Fatalf("query-only role received %s", action)
		}
	}
}
