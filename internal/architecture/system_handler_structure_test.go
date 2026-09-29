package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSystemHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	transportRoot := filepath.Join(repositoryRoot, "internal", "platform", "http", "system")

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "ReleaseChecker",
	})
	assertDirectoryGoFileBudget(t, transportRoot, 3)

	legacy := filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_system.go")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatalf("legacy system handler must stay removed: %s", legacy)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy system handler: %v", err)
	}
}
