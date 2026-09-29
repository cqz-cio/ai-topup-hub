package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAdProxyHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "adproxy")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	gatewayRoot := filepath.Join(moduleRoot, "infrastructure", "adgateway")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "render.go"), []string{
		"RenderSlot", "RenderItem", "RenderResponse",
	})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "gateway.go"), []string{"Gateway"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(gatewayRoot, "client.go"), []string{"Client"})
	assertFileDeclaresFunctions(t, filepath.Join(gatewayRoot, "client.go"), []string{"New", "NewClient"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "AdminService",
	})
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("adproxy module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 2)
	assertDirectoryGoFileBudget(t, contractRoot, 2)
	assertDirectoryGoFileBudget(t, domainRoot, 2)
	assertDirectoryGoFileBudget(t, gatewayRoot, 5)
	assertDirectoryGoFileBudget(t, transportRoot, 3)
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/adgateway")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/adgateway")
	assertProductionImportsAbsent(t, domainRoot, moduleImportPath+"/internal/adgateway")
	assertProductionImportsAbsent(t, transportRoot, moduleImportPath+"/internal/adgateway")

	legacy := filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_ad_proxy.go")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatalf("legacy ad proxy handler must stay removed: %s", legacy)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy ad proxy handler: %v", err)
	}
	legacyService := filepath.Join(repositoryRoot, "internal", "service", "ad_proxy_service.go")
	if _, err := os.Stat(legacyService); err == nil {
		t.Fatalf("legacy ad proxy service must stay removed: %s", legacyService)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy ad proxy service: %v", err)
	}
}
