package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestComplianceHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "compliance")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	integrationTestRoot := filepath.Join(moduleRoot, "integrationtest")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "types.go"), []string{"AcknowledgeCommand"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{"SettingsStore"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "status.go"), []string{"Status"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "AdminService",
	})
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("compliance module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 3)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, domainRoot, 2)
	assertDirectoryGoFileBudget(t, integrationTestRoot, 3)
	assertDirectoryGoFileBudget(t, transportRoot, 3)
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/transport")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/transport")
	assertProductionImportsAbsent(t, domainRoot, moduleImportPath+"/internal/transport")

	legacy := filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_compliance.go")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatalf("legacy compliance handler must stay removed: %s", legacy)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy compliance handler: %v", err)
	}
	legacyService := filepath.Join(repositoryRoot, "internal", "service", "compliance_service.go")
	if _, err := os.Stat(legacyService); err == nil {
		t.Fatalf("legacy compliance service must stay removed: %s", legacyService)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy compliance service: %v", err)
	}
}
