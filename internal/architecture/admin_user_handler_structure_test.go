package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdminUserHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	transportRoot := filepath.Join(
		repositoryRoot,
		"internal", "modules", "identity", "user", "transport", "http", "admin",
	)

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "UserDirectory", "WalletBalances", "OAuthIdentityUnbinder", "AuthStateCache",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"NewAdminHandler", "GetAdminUsers", "GetAdminUser", "UpdateAdminUser",
		"UnbindAdminUserTelegram", "UnbindAdminUserGoogle", "GetAdminUserCouponUsages", "BatchUpdateUserStatus",
	})
	assertDirectoryGoFileBudget(t, transportRoot, 3)

	legacy := filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_user.go")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatalf("legacy admin user handler must stay removed: %s", legacy)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy admin user handler: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, "internal", "router", "adminuser_adapter.go")); err == nil {
		t.Fatal("adminuser composition adapters belong in internal/bootstrap/adminuser")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy adminuser router adapter: %v", err)
	}
	assertDirectoryGoFileBudget(t, filepath.Join(repositoryRoot, "internal", "bootstrap", "adminuser"), 4)
}
