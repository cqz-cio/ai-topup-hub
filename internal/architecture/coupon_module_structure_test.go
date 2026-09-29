package architecture

import (
	"path/filepath"
	"testing"
)

func TestCouponImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "coupon")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	integrationTestRoot := filepath.Join(moduleRoot, "integrationtest")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "coupon.go"), []string{"Coupon"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "usage.go"), []string{"CouponUsage"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{
		"ListFilter", "UsageListFilter", "EligibilityItem", "Repository", "UsageRepository",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "admin_service.go"), []string{
		"AdminService", "CreateCouponInput", "UpdateCouponInput",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "usage_store.go"), []string{"UsageStore"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("coupon module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, domainRoot, 4)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, applicationRoot, 3)
	assertDirectoryGoFileBudget(t, storeRoot, 5)
	assertDirectoryGoFileBudget(t, integrationTestRoot, 3)
	assertDirectoryGoFileBudget(t, transportRoot, 3)
}

func TestCouponLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "coupon")
	patterns := []string{
		filepath.Join(repositoryRoot, "internal", "models", "coupon*.go"),
		filepath.Join(repositoryRoot, "internal", "service", "coupon*.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "coupon*.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_coupon*.go"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "coupon"),
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
			t.Errorf("legacy coupon files must stay removed: %v", matches)
		}
	}
}
