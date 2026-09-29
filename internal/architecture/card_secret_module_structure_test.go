package architecture

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCardSecretUsesCompleteVerticalLayout(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "cardsecret")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")
	integrationRoot := filepath.Join(moduleRoot, "integrationtest")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("card secret module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 6)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, domainRoot, 3)
	assertDirectoryGoFileBudget(t, storeRoot, 3)
	assertDirectoryGoFileBudget(t, transportRoot, 3)
	assertDirectoryGoFileBudget(t, integrationRoot, 2)

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service", "ServiceOptions"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"Repository", "BatchRepository", "UnitOfWork", "ProductRepository", "ProductSKURepository",
	})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "secret.go"), []string{"Secret"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "batch.go"), []string{"Batch"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "batch_store.go"), []string{"BatchStore"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{"AdminHandler", "Service"})

	expected := map[string][]string{
		"service.go": {"NewService", "resolveCardSecretSKU", "normalizeCardSecretIDs"},
		"import.go": {
			"CreateCardSecretBatch", "ImportCardSecretCSV", "shouldDeduplicateCardSecrets",
			"normalizeSecrets", "parseCSVSecrets", "generateBatchNo",
		},
		"manage.go": {
			"ListCardSecrets", "buildRepositoryFilter", "hasListFilter",
			"BatchUpdateCardSecretStatus", "BatchDeleteCardSecrets", "UpdateCardSecret",
		},
		"export.go": {
			"ExportCardSecrets", "ExportAvailableCardSecrets", "normalizeCardSecretExportFormat",
			"buildCardSecretExportContent", "validateAutoCardSecretExportScope",
			"resolveExportTargetCardSecretIDs", "resolveBatchTargetCardSecretIDs",
		},
		"stats.go": {"GetStats", "ListBatches"},
	}
	for file, want := range expected {
		parsed := parseProductionGoFile(t, filepath.Join(applicationRoot, file))
		got := declaredFunctionNames(parsed)
		sort.Strings(want)
		sort.Strings(got)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s function ownership mismatch\nwant: %v\ngot:  %v", file, want, got)
		}
	}

	for _, forbiddenImport := range []string{
		"github.com/dujiao-next/internal/models",
		"github.com/dujiao-next/internal/repository",
		"gorm.io/gorm",
		"github.com/gin-gonic/gin",
	} {
		assertProductionImportsAbsent(t, applicationRoot, forbiddenImport)
	}
	assertProductionImportsAbsent(t, contractRoot, "github.com/dujiao-next/internal/models")
	assertProductionImportsAbsent(t, contractRoot, "gorm.io/gorm")
	assertProductionImportsAbsent(t, domainRoot, "gorm.io/gorm")

	legacyPaths := []string{
		"internal/models/card_secret.go",
		"internal/models/card_secret_batch.go",
		"internal/repository/card_secret_compat.go",
		"internal/repository/card_secret_repository.go",
		"internal/repository/card_secret_batch_repository.go",
		"internal/service/card_secret_core.go",
		"internal/service/card_secret_import.go",
		"internal/service/card_secret_manage.go",
		"internal/service/card_secret_export.go",
		"internal/service/card_secret_stats.go",
		"internal/service/card_secret_service_test.go",
		"internal/modules/cardsecret/service.go",
		"internal/modules/cardsecret/import.go",
		"internal/modules/cardsecret/manage.go",
		"internal/modules/cardsecret/export.go",
		"internal/modules/cardsecret/stats.go",
		"internal/modules/cardsecret/store",
		"internal/integration/cardsecret",
		"internal/transport/http/cardsecret",
		"internal/http/handlers/admin/card_secret_admin.go",
	}
	for _, relativePath := range legacyPaths {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err == nil {
			t.Errorf("legacy card secret path must not return: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", relativePath, err)
		}
	}
}
