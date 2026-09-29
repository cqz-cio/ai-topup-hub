package architecture

import (
	"path/filepath"
	"testing"
)

func TestSiteConnectionOwnsCompleteVerticalSlice(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "siteconnection")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "connection.go"), []string{"Connection"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{"ListFilter", "Repository"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "types.go"), []string{
		"CreateInput", "UpdateInput", "PingResult",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "store.go"), []string{"New"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "AdminService", "MarkupReapplier",
	})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("siteconnection module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, domainRoot, 1)
	assertDirectoryGoFileBudget(t, contractRoot, 2)
	assertDirectoryGoFileBudget(t, applicationRoot, 2)
	assertDirectoryGoFileBudget(t, storeRoot, 2)
	assertDirectoryGoFileBudget(t, transportRoot, 3)
}
