package architecture

import (
	"path/filepath"
	"testing"
)

func TestPromotionImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "promotion")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "promotion.go"), []string{"Promotion"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{"ListFilter", "Repository"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "admin_service.go"), []string{
		"AdminService", "CreatePromotionInput", "UpdatePromotionInput",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{"AdminService", "AdminHandler"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("promotion module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, domainRoot, 2)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, applicationRoot, 3)
	assertDirectoryGoFileBudget(t, storeRoot, 3)
	assertDirectoryGoFileBudget(t, transportRoot, 3)
}

func TestPromotionLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "promotion")
	patterns := []string{
		filepath.Join(repositoryRoot, "internal", "models", "promotion.go"),
		filepath.Join(repositoryRoot, "internal", "service", "promotion*.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "promotion*.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_promotion*.go"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "promotion"),
		filepath.Join(moduleRoot, "admin_service.go"),
		filepath.Join(moduleRoot, "errors.go"),
		filepath.Join(moduleRoot, "ports.go"),
		filepath.Join(moduleRoot, "service.go"),
		filepath.Join(moduleRoot, "store"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(matches) != 0 {
			t.Errorf("legacy promotion files must stay removed: %v", matches)
		}
	}
}
