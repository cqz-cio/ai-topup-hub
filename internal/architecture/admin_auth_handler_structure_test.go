package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdminAuthHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	transportRoot := filepath.Join(
		repositoryRoot,
		"internal", "modules", "identity", "adminauth", "transport", "http",
	)

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterAdminLoginAuthRoutes",
		"RegisterAdminPasswordRoutes",
		"RegisterAdmin2FAAuthRoutes",
		"RegisterAdmin2FARoutes",
		"RegisterAdminUser2FARoutes",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_login_handler.go"), []string{
		"LoginAuthService", "CaptchaVerifier", "AdminLoginHandler", "WeakPasswordError",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_login_handler.go"), []string{
		"NewAdminLoginHandler", "AdminLogin", "UpdateAdminPassword", "NewWeakPasswordError",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_2fa_handler.go"), []string{
		"TOTPService", "AuthService", "ChallengeStore", "AdminLoginRecorder", "Admin2FAHandler",
		"TOTPStatus", "TOTPSetupResult", "TOTPEnableResult", "ChallengeClaims", "AuthLoginResult",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_2fa_handler.go"), []string{
		"NewAdmin2FAHandler", "Get2FAStatus", "Setup2FA", "Enable2FA", "Disable2FA",
		"RegenerateRecoveryCodes", "Verify2FA", "ResetTargetAdmin2FA",
	})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_user_2fa_handler.go"), []string{
		"UserTOTPService", "AdminUser2FAHandler",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_user_2fa_handler.go"), []string{
		"NewAdminUser2FAHandler", "ResetUser2FA",
	})
	assertDirectoryGoFileBudget(t, transportRoot, 5)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_2fa.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_user_2fa.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_login_log.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy adminauth handler must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy adminauth handler: %v", err)
		}
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, "internal", "router", "adminauth_adapter.go")); err == nil {
		t.Fatal("adminauth composition adapters belong in internal/bootstrap/adminauth")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy adminauth router adapter: %v", err)
	}
	assertDirectoryGoFileBudget(t, filepath.Join(repositoryRoot, "internal", "bootstrap", "adminauth"), 4)
}

func TestAdminIdentityPersistenceLivesInVerticalModule(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "identity", "admin")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("admin identity module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "domain", "admin.go"), []string{"Admin"})
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "contract", "store.go"), []string{"Store"})
	assertFileDeclaresTypes(t, filepath.Join(moduleRoot, "infrastructure", "gormstore", "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(moduleRoot, "infrastructure", "gormstore", "store.go"), []string{"New"})
	assertFileDeclaresFunctions(t, filepath.Join(moduleRoot, "application", "bootstrap.go"), []string{"InitDefaultAdmin"})
	assertDirectoryGoFileBudget(t, filepath.Join(moduleRoot, "application"), 2)
	assertDirectoryGoFileBudget(t, filepath.Join(moduleRoot, "domain"), 1)
	assertDirectoryGoFileBudget(t, filepath.Join(moduleRoot, "contract"), 1)
	assertDirectoryGoFileBudget(t, filepath.Join(moduleRoot, "infrastructure", "gormstore"), 2)
}
