package authz

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupAuthzServiceTest(t *testing.T) *Service {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	svc, err := NewService(db)
	if err != nil {
		t.Fatalf("new authz service failed: %v", err)
	}
	return svc
}

func TestEnforceAdminWithRolePolicy(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.GrantRolePolicy("ops", "/admin/products/:id", "GET"); err != nil {
		t.Fatalf("grant role policy failed: %v", err)
	}
	if err := svc.SetAdminRoles(1, []string{"ops"}); err != nil {
		t.Fatalf("set admin roles failed: %v", err)
	}

	allow, err := svc.EnforceAdmin(1, "/api/v1/admin/products/42", "get")
	if err != nil {
		t.Fatalf("enforce allow failed: %v", err)
	}
	if !allow {
		t.Fatalf("expected allow=true")
	}

	allow, err = svc.EnforceAdmin(1, "/api/v1/admin/products/42", "POST")
	if err != nil {
		t.Fatalf("enforce deny failed: %v", err)
	}
	if allow {
		t.Fatalf("expected allow=false")
	}
}

func TestSetAdminRolesOverride(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.GrantRolePolicy("ops", "/admin/orders", "GET"); err != nil {
		t.Fatalf("grant ops policy failed: %v", err)
	}
	if err := svc.GrantRolePolicy("billing_custom", "/admin/payments", "GET"); err != nil {
		t.Fatalf("grant billing policy failed: %v", err)
	}

	if err := svc.SetAdminRoles(2, []string{"ops"}); err != nil {
		t.Fatalf("set first role failed: %v", err)
	}
	roles, err := svc.GetAdminRoles(2)
	if err != nil {
		t.Fatalf("get roles failed: %v", err)
	}
	if len(roles) != 1 || roles[0] != "role:ops" {
		t.Fatalf("roles want [role:ops], got=%v", roles)
	}

	if err := svc.SetAdminRoles(2, []string{"billing_custom"}); err != nil {
		t.Fatalf("set second role failed: %v", err)
	}
	roles, err = svc.GetAdminRoles(2)
	if err != nil {
		t.Fatalf("get roles failed: %v", err)
	}
	if len(roles) != 1 || roles[0] != "role:billing_custom" {
		t.Fatalf("roles want [role:billing_custom], got=%v", roles)
	}

	allow, err := svc.EnforceAdmin(2, "/admin/orders", "GET")
	if err != nil {
		t.Fatalf("enforce old role failed: %v", err)
	}
	if allow {
		t.Fatalf("expected old role permission removed")
	}

	allow, err = svc.EnforceAdmin(2, "/admin/payments", "GET")
	if err != nil {
		t.Fatalf("enforce new role failed: %v", err)
	}
	if !allow {
		t.Fatalf("expected new role permission granted")
	}
}

func TestNormalizeObject(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{in: "/api/v1/admin/orders/:id", want: "/admin/orders/:id"},
		{in: "/admin/orders/:id", want: "/admin/orders/:id"},
		{in: "admin/orders", want: "/admin/orders"},
		{in: "/api/v1", want: "/"},
		{in: "", want: "/"},
	}
	for _, item := range cases {
		got := NormalizeObject(item.in)
		if got != item.want {
			t.Fatalf("normalize object failed, in=%q want=%q got=%q", item.in, item.want, got)
		}
	}
}

func TestBootstrapBuiltinRoles(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("bootstrap builtin roles failed: %v", err)
	}

	roles, err := svc.ListRoles()
	if err != nil {
		t.Fatalf("list roles failed: %v", err)
	}
	wantRoles := map[string]bool{
		"role:readonly_auditor": true,
		"role:operations":       true,
		"role:support":          true,
		"role:finance":          true,
	}
	for _, role := range roles {
		delete(wantRoles, role)
	}
	if len(wantRoles) != 0 {
		t.Fatalf("builtin roles missing: %v", wantRoles)
	}

	if err := svc.SetAdminRoles(3, []string{"operations"}); err != nil {
		t.Fatalf("set admin roles failed: %v", err)
	}

	allow, err := svc.EnforceAdmin(3, "/admin/dashboard/overview", "GET")
	if err != nil {
		t.Fatalf("enforce inherited readonly failed: %v", err)
	}
	if !allow {
		t.Fatalf("expected inherited readonly permission")
	}

	allow, err = svc.EnforceAdmin(3, "/admin/settings", "PUT")
	if err != nil {
		t.Fatalf("enforce readonly write failed: %v", err)
	}
	if allow {
		t.Fatalf("expected readonly inherited role deny write")
	}

	if err := svc.SetAdminRoles(4, []string{"system_admin"}); err != nil {
		t.Fatalf("set system admin role failed: %v", err)
	}
	for _, action := range []string{"GET", "PUT"} {
		allow, err = svc.EnforceAdmin(4, "/admin/settings/google-auth", action)
		if err != nil {
			t.Fatalf("enforce google auth settings %s failed: %v", action, err)
		}
		if !allow {
			t.Fatalf("expected system admin to access google auth settings with %s", action)
		}
	}

	for _, roleCase := range []struct {
		adminID uint
		role    string
		want    bool
	}{
		{adminID: 5, role: "support", want: true},
		{adminID: 6, role: "system_admin", want: true},
		{adminID: 7, role: "readonly_auditor", want: false},
	} {
		if err := svc.SetAdminRoles(roleCase.adminID, []string{roleCase.role}); err != nil {
			t.Fatalf("assign %s role failed: %v", roleCase.role, err)
		}
		allow, err = svc.EnforceAdmin(roleCase.adminID, "/admin/users/42/oauth/google", "DELETE")
		if err != nil {
			t.Fatalf("enforce google unbind for %s failed: %v", roleCase.role, err)
		}
		if allow != roleCase.want {
			t.Fatalf("google unbind for %s allow=%v, want %v", roleCase.role, allow, roleCase.want)
		}
	}
}

func TestBootstrapBuiltinRolesRemovesStaleImmutableWildcard(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if _, err := svc.EnsureRole("readonly_auditor"); err != nil {
		t.Fatalf("seed readonly role failed: %v", err)
	}
	if _, err := svc.enforcer.AddPolicy("role:readonly_auditor", "/admin/*", "GET"); err != nil {
		t.Fatalf("seed stale wildcard failed: %v", err)
	}
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("bootstrap builtin roles failed: %v", err)
	}
	if err := svc.SetAdminRoles(9, []string{"readonly_auditor"}); err != nil {
		t.Fatalf("assign readonly role failed: %v", err)
	}

	allow, err := svc.EnforceAdmin(9, "/admin/payment-channels/1", "GET")
	if err != nil {
		t.Fatalf("enforce sensitive route failed: %v", err)
	}
	if allow {
		t.Fatal("stale readonly wildcard must be removed")
	}
	allow, err = svc.EnforceAdmin(9, "/admin/dashboard/overview", "GET")
	if err != nil || !allow {
		t.Fatalf("explicit readonly dashboard policy should remain, allow=%v err=%v", allow, err)
	}
}

func TestImmutableBuiltinRoleRejectsPolicyMutationAndDeletion(t *testing.T) {
	svc := setupAuthzServiceTest(t)
	if err := svc.BootstrapBuiltinRoles(); err != nil {
		t.Fatalf("bootstrap builtin roles failed: %v", err)
	}

	if err := svc.GrantRolePolicy("readonly_auditor", "/admin/payment-channels", "GET"); !errors.Is(err, ErrImmutableBuiltinRole) {
		t.Fatalf("grant immutable role policy error = %v, want ErrImmutableBuiltinRole", err)
	}
	if err := svc.RevokeRolePolicy("readonly_auditor", "/admin/dashboard/overview", "GET"); !errors.Is(err, ErrImmutableBuiltinRole) {
		t.Fatalf("revoke immutable role policy error = %v, want ErrImmutableBuiltinRole", err)
	}
	if err := svc.DeleteRole("readonly_auditor"); !errors.Is(err, ErrImmutableBuiltinRole) {
		t.Fatalf("delete immutable role error = %v, want ErrImmutableBuiltinRole", err)
	}

	policies, err := svc.GetRolePolicies("readonly_auditor")
	if err != nil {
		t.Fatalf("get readonly policies failed: %v", err)
	}
	foundDashboard := false
	for _, policy := range policies {
		if policy.Object == "/admin/dashboard/overview" && policy.Action == "GET" {
			foundDashboard = true
		}
		if policy.Object == "/admin/payment-channels" {
			t.Fatal("rejected policy grant must not persist")
		}
	}
	if !foundDashboard {
		t.Fatal("rejected policy revoke must preserve builtin policy")
	}
}
