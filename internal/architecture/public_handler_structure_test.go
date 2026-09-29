package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyPublicHandlerPackageIsRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	handlerDirectory := filepath.Join(repositoryRoot, "internal", "http", "handlers", "public")
	files, err := filepath.Glob(filepath.Join(handlerDirectory, "*.go"))
	if err != nil {
		t.Fatalf("list legacy public handler package: %v", err)
	}
	if len(files) != 0 {
		t.Fatalf("legacy public handler Go files must stay removed: %v", files)
	}
}

func TestPublicConfigHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	transportRoot := filepath.Join(
		repositoryRoot,
		"internal", "modules", "settings", "transport", "http", "public",
	)

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterPublicRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "handler.go"), []string{
		"Handler", "Settings", "PaymentChannels", "ConfigCache",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "handler.go"), []string{
		"NewHandler", "GetConfig",
	})
	assertDirectoryGoFileBudget(t, transportRoot, 3)

	legacy := filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "public_config.go")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatalf("legacy public config handler must stay removed: %s", legacy)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy public config handler: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repositoryRoot, "internal", "router", "publicconfig_adapter.go")); err == nil {
		t.Fatal("public config composition adapters belong in internal/bootstrap/publicconfig")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy public config router adapter: %v", err)
	}
	assertDirectoryGoFileBudget(t, filepath.Join(repositoryRoot, "internal", "bootstrap", "publicconfig"), 4)
}
