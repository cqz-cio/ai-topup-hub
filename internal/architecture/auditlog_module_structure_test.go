package architecture

import (
	"path/filepath"
	"testing"
)

func TestAuditLogImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "auditlog")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "admin_login.go"), []string{"AdminLoginLog"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "user_login.go"), []string{"UserLoginLog"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "authz.go"), []string{"AuthzAuditLog"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{
		"UserLoginFilter", "AuthzFilter", "AdminLoginFilter",
		"UserLoginRepository", "AuthzRepository", "AdminLoginRepository",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "user_login_service.go"), []string{
		"UserLoginRecord", "UserLoginService",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "authz_service.go"), []string{
		"AuthzRecord", "AuthzService",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "admin_login_service.go"), []string{
		"AdminLoginRecord", "AdminLoginService",
	})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "user_login_store.go"), []string{"UserLoginStore"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "authz_store.go"), []string{"AuthzStore"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "admin_login_store.go"), []string{"AdminLoginStore"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterAdminRoutes", "RegisterUserRoutes",
	})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("auditlog module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, domainRoot, 3)
	assertDirectoryGoFileBudget(t, contractRoot, 1)
	assertDirectoryGoFileBudget(t, applicationRoot, 4)
	assertDirectoryGoFileBudget(t, storeRoot, 5)
	assertDirectoryGoFileBudget(t, transportRoot, 5)
}

func TestAuditLogLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	patterns := []string{
		filepath.Join(repositoryRoot, "internal", "service", "user_login_log_service.go"),
		filepath.Join(repositoryRoot, "internal", "service", "authz_audit_service.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "user_login_log_repository.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "authz_audit_log_repository.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "admin_login_log_repository*.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_authz_audit.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "user_login_log_admin.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "user_login_log.go"),
		filepath.Join(repositoryRoot, "internal", "models", "admin_login_log.go"),
		filepath.Join(repositoryRoot, "internal", "models", "user_login_log.go"),
		filepath.Join(repositoryRoot, "internal", "models", "authz_audit_log.go"),
		filepath.Join(repositoryRoot, "internal", "modules", "auditlog", "ports.go"),
		filepath.Join(repositoryRoot, "internal", "modules", "auditlog", "store"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "auditlog"),
		filepath.Join(repositoryRoot, "internal", "dto", "login_log.go"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(matches) != 0 {
			t.Errorf("legacy audit-log files must stay removed: %v", matches)
		}
	}
}
