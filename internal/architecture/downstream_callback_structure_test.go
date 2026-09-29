package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDownstreamCallbackLivesInCompleteVertical(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "downstreamcallback")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	gormStoreRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	callbackClientRoot := filepath.Join(moduleRoot, "infrastructure", "callbackclient")
	orderReaderRoot := filepath.Join(moduleRoot, "infrastructure", "orderreader")
	credentialReaderRoot := filepath.Join(moduleRoot, "infrastructure", "credentialreader")
	queueAdapterRoot := filepath.Join(moduleRoot, "infrastructure", "queueadapter")

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Options", "Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"Repository", "OrderReader", "CredentialReader", "CallbackQueue", "Deliverer",
	})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "order_ref.go"), []string{"OrderRef"})
	assertFileDeclaresTypes(t, filepath.Join(gormStoreRoot, "store.go"), []string{"Store"})
	assertFileDeclaresTypes(t, filepath.Join(callbackClientRoot, "client.go"), []string{"Client"})
	assertFileDeclaresTypes(t, filepath.Join(orderReaderRoot, "reader.go"), []string{"Reader"})
	assertFileDeclaresTypes(t, filepath.Join(credentialReaderRoot, "reader.go"), []string{"Reader"})
	assertFileDeclaresTypes(t, filepath.Join(queueAdapterRoot, "adapter.go"), []string{"Adapter"})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("downstream callback module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 2)
	assertDirectoryGoFileBudget(t, contractRoot, 4)
	assertDirectoryGoFileBudget(t, domainRoot, 2)
	assertDirectoryGoFileBudget(t, gormStoreRoot, 2)
	assertDirectoryGoFileBudget(t, callbackClientRoot, 3)
	assertDirectoryGoFileBudget(t, orderReaderRoot, 2)
	assertDirectoryGoFileBudget(t, credentialReaderRoot, 2)
	assertDirectoryGoFileBudget(t, queueAdapterRoot, 2)

	for _, forbiddenImport := range []string{
		"net/http",
		"github.com/hibiken/asynq",
		"github.com/dujiao-next/internal/models",
		"github.com/dujiao-next/internal/queue",
		"github.com/dujiao-next/internal/upstream",
	} {
		assertProductionImportsAbsent(t, applicationRoot, forbiddenImport)
	}
	assertProductionImportsAbsent(t, contractRoot, "github.com/dujiao-next/internal/models")
	assertProductionImportsAbsent(t, domainRoot, "github.com/dujiao-next/internal/models")

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "models", "downstream_order_ref.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "downstream_order_ref_repository.go"),
		filepath.Join(repositoryRoot, "internal", "service", "downstream_callback_service.go"),
		filepath.Join(repositoryRoot, "internal", "modules", "downstreamcallback", "service.go"),
		filepath.Join(repositoryRoot, "internal", "integration", "downstreamcallback"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy downstream callback path must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy downstream callback path: %v", err)
		}
	}
}
