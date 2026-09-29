package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReconciliationUsesCompleteVerticalLayout(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "reconciliation")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	procurementRoot := filepath.Join(moduleRoot, "infrastructure", "procurementreader")
	upstreamRoot := filepath.Join(moduleRoot, "infrastructure", "upstreamreader")
	queueRoot := filepath.Join(moduleRoot, "infrastructure", "queueadapter")
	notificationRoot := filepath.Join(moduleRoot, "infrastructure", "notificationadapter")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("reconciliation module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 4)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, domainRoot, 4)
	assertDirectoryGoFileBudget(t, storeRoot, 2)
	assertDirectoryGoFileBudget(t, procurementRoot, 1)
	assertDirectoryGoFileBudget(t, upstreamRoot, 1)
	assertDirectoryGoFileBudget(t, queueRoot, 1)
	assertDirectoryGoFileBudget(t, notificationRoot, 1)
	assertDirectoryGoFileBudget(t, transportRoot, 3)

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service", "Options"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService", "CreateAndEnqueue", "Execute"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "query.go"), []string{"GetJob", "ListJobs", "GetJobItems", "ResolveItem"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"JobRepository", "ItemRepository", "ProcurementReader", "UpstreamOrderProvider", "UpstreamOrderReader", "Enqueuer", "MismatchNotifier", "UseCase",
	})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "types.go"), []string{
		"RunInput", "JobListFilter", "ProcurementOrder", "UpstreamOrder",
	})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "job.go"), []string{"Job"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "item.go"), []string{"Item"})
	assertFileDeclaresFunctions(t, filepath.Join(domainRoot, "status.go"), []string{"IsStatusConsistent"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "job_store.go"), []string{"JobStore"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "item_store.go"), []string{"ItemStore"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{"AdminHandler", "Service"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})

	for _, forbiddenImport := range []string{
		moduleImportPath + "/internal/models",
		moduleImportPath + "/internal/queue",
		moduleImportPath + "/internal/upstream",
		moduleImportPath + "/internal/modules/notification",
		"github.com/gin-gonic/gin",
		"gorm.io/gorm",
	} {
		assertProductionImportsAbsent(t, applicationRoot, forbiddenImport)
	}
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/queue")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/upstream")
	assertProductionImportsAbsent(t, domainRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, domainRoot, "gorm.io/gorm")
	assertProductionImportsAbsent(t, transportRoot, moduleImportPath+"/internal/models")

	for _, relativePath := range []string{
		"internal/models/reconciliation.go",
		"internal/modules/reconciliation/service.go",
		"internal/modules/reconciliation/execute.go",
		"internal/modules/reconciliation/query.go",
		"internal/modules/reconciliation/service_test.go",
		"internal/modules/reconciliation/store",
		"internal/transport/http/reconciliation",
		"internal/service/reconciliation_service.go",
		"internal/repository/reconciliation_repository.go",
		"internal/http/handlers/admin/admin_reconciliation.go",
	} {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("legacy reconciliation path must stay removed: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", relativePath, err)
		}
	}
}
