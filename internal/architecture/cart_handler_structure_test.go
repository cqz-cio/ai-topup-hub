package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCartHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "cart")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	domainRoot := filepath.Join(moduleRoot, "domain")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")
	presenterRoot := filepath.Join(moduleRoot, "transport", "presenter")

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "types.go"), []string{"ItemDetail", "UpsertItemInput"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"StoredItem", "Repository", "ProductReader", "SKUReader", "CurrencyReader",
	})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "item.go"), []string{"Item"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "store.go"), []string{"New", "WithTx"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterUserRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "user_handler.go"), []string{
		"UserHandler", "Service",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "user_handler.go"), []string{
		"NewUserHandler", "GetCart", "UpsertCartItem", "DeleteCartItem",
	})
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("cart module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 3)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, domainRoot, 2)
	assertDirectoryGoFileBudget(t, storeRoot, 3)
	assertDirectoryGoFileBudget(t, transportRoot, 4)
	assertDirectoryGoFileBudget(t, presenterRoot, 2)
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, domainRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, transportRoot, moduleImportPath+"/internal/models")

	legacy := filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "cart.go")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatalf("legacy cart handler must stay removed: %s", legacy)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy cart handler: %v", err)
	}
	for _, relativePath := range []string{
		"internal/service/cart_service.go",
		"internal/wiring/cart/wiring.go",
		"internal/wiring/cart/factory.go",
	} {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("legacy cart file must stay removed: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", relativePath, err)
		}
	}
}
