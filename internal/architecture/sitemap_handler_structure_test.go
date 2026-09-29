package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSitemapHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "sitemap")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	cacheRoot := filepath.Join(moduleRoot, "infrastructure", "cacheadapter")
	catalogRoot := filepath.Join(moduleRoot, "infrastructure", "catalogreader")
	brandRoot := filepath.Join(moduleRoot, "infrastructure", "settingsbrand")
	integrationTestRoot := filepath.Join(moduleRoot, "integrationtest")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"Category", "Product", "PublishedPost", "CatalogReader", "PublishedPostReader", "PublishedPostReaderFunc", "Cache",
	})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "url.go"), []string{"URL"})
	assertFileDeclaresTypes(t, filepath.Join(cacheRoot, "cache.go"), []string{"Cache"})
	assertFileDeclaresTypes(t, filepath.Join(catalogRoot, "reader.go"), []string{"Reader"})
	assertFileDeclaresTypes(t, filepath.Join(brandRoot, "reader.go"), []string{"Reader"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "handler.go"), []string{
		"Handler", "Generator", "SiteBrandReader",
	})
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("sitemap module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 2)
	assertDirectoryGoFileBudget(t, contractRoot, 2)
	assertDirectoryGoFileBudget(t, domainRoot, 2)
	assertDirectoryGoFileBudget(t, cacheRoot, 2)
	assertDirectoryGoFileBudget(t, catalogRoot, 2)
	assertDirectoryGoFileBudget(t, brandRoot, 2)
	assertDirectoryGoFileBudget(t, integrationTestRoot, 2)
	assertDirectoryGoFileBudget(t, transportRoot, 4)
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/cache")
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/modules/catalog")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/modules/catalog")
	assertProductionImportsAbsent(t, domainRoot, moduleImportPath+"/internal/modules/catalog")

	legacy := filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "sitemap.go")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatalf("legacy public sitemap handler must stay removed: %s", legacy)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy sitemap handler: %v", err)
	}
	legacyService := filepath.Join(repositoryRoot, "internal", "service", "sitemap_service.go")
	if _, err := os.Stat(legacyService); err == nil {
		t.Fatalf("legacy sitemap service must stay removed: %s", legacyService)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy sitemap service: %v", err)
	}
}
