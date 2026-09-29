package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdminAuthzHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	transportRoot := filepath.Join(
		repositoryRoot,
		"internal", "modules", "identity", "adminauthorization", "transport", "http",
	)

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "RolePolicyService", "AdminDirectory", "PasswordService", "AuthStateCache", "AuditRecorder", "Policy",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"NewAdminHandler", "GetAuthzMe", "ListAuthzRoles", "ListAuthzAdmins",
		"CreateAuthzRole", "DeleteAuthzRole", "GetAuthzRolePolicies",
		"GrantAuthzPolicy", "RevokeAuthzPolicy", "GetAuthzAdminRoles", "SetAuthzAdminRoles",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_account_handler.go"), []string{
		"CreateAuthzAdmin", "UpdateAuthzAdmin", "DeleteAuthzAdmin",
	})
	assertDirectoryGoFileBudget(t, transportRoot, 5)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_authz.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_authz_admins.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy admin authz handler must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy admin authz handler: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, "internal", "router", "adminauthz_adapter.go")); err == nil {
		t.Fatal("adminauthz composition adapters belong in internal/bootstrap/adminauthz")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy adminauthz router adapter: %v", err)
	}
	assertDirectoryGoFileBudget(t, filepath.Join(repositoryRoot, "internal", "bootstrap", "adminauthz"), 4)
}
