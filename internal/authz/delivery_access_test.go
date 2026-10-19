package authz

import "testing"

func TestDeliveryAccessRequiresSeparatePermission(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatal(err)
	}
	for _, role := range []string{"finance", "readonly_auditor", "operations", "support"} {
		if err := svc.SetAdminRoles(41, []string{role}); err != nil {
			t.Fatal(err)
		}
		for _, path := range []string{"/admin/orders/23/fulfillment", "/admin/orders/23/fulfillment/download"} {
			allowed, err := svc.EnforceAdmin(41, path, "GET")
			if err != nil || allowed != (role == "support") {
				t.Fatalf("role=%s path=%s allowed=%t err=%v", role, path, allowed, err)
			}
		}
	}
	if err := svc.GrantRolePolicy("order_viewer", "/admin/orders/:id", "GET"); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetAdminRoles(42, []string{"order_viewer"}); err != nil {
		t.Fatal(err)
	}
	allowed, err := svc.EnforceAdmin(42, "/admin/orders/23/fulfillment", "GET")
	if err != nil || allowed {
		t.Fatalf("ordinary order permission grants delivery: %t %v", allowed, err)
	}
}
