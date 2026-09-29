package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFulfillmentHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "fulfillment")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("fulfillment module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "fulfillment.go"), []string{"Fulfillment"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "store.go"), []string{"Store"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service", "Options"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"New"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "store.go"), []string{"New"})
	assertDirectoryGoFileBudget(t, domainRoot, 2)
	assertDirectoryGoFileBudget(t, contractRoot, 2)
	assertDirectoryGoFileBudget(t, applicationRoot, 3)
	assertDirectoryGoFileBudget(t, storeRoot, 2)
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/service")
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/repository")
	assertProductionImportsAbsent(t, applicationRoot, "gorm.io/gorm")

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "ManualCreator", "AdminOrderReader", "CreateManualInput",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"NewAdminHandler", "AdminCreateFulfillment", "AdminDownloadFulfillment",
	})
	assertDirectoryGoFileBudget(t, transportRoot, 3)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "fulfillment_admin.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "cache.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy fulfillment/cache handler must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy handler: %v", err)
		}
	}
}
