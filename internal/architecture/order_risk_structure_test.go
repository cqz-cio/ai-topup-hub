package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOrderRiskUsesCompleteVerticalLayout(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "orderrisk")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	limiterRoot := filepath.Join(moduleRoot, "infrastructure", "redislimiter")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("order risk module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 2)
	assertDirectoryGoFileBudget(t, contractRoot, 4)
	assertDirectoryGoFileBudget(t, domainRoot, 1)
	assertDirectoryGoFileBudget(t, limiterRoot, 1)

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Options", "Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "types.go"), []string{"CheckInput"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"SettingReader", "PendingOrderGate", "RateLimiter", "Controller",
	})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "errors.go"), []string{"RateLimitedError"})
	assertFileDeclaresFunctions(t, filepath.Join(contractRoot, "errors.go"), []string{"GetRetryAfter"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "lock_key.go"), []string{"LockKey"})
	assertFileDeclaresTypes(t, filepath.Join(limiterRoot, "limiter.go"), []string{"Limiter"})
	assertFileDeclaresFunctions(t, filepath.Join(limiterRoot, "limiter.go"), []string{"New"})

	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/cache")
	assertProductionImportsAbsent(t, applicationRoot, "github.com/redis/go-redis")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/cache")
	assertProductionImportsAbsent(t, contractRoot, "github.com/redis/go-redis")

	for _, relativePath := range []string{
		"internal/modules/orderrisk/service.go",
		"internal/modules/orderrisk/service_test.go",
		"internal/service/order_risk_control_service.go",
		"internal/service/order_risk_control_setting.go",
	} {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("legacy order risk path must stay removed: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", relativePath, err)
		}
	}
}
