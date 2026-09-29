package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestChannelClientHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "channelclient")
	applicationRoot := filepath.Join(moduleRoot, "application")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "types.go"), []string{"ClientDetail", "ActiveEndpoint"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"AdminHandler", "AdminService",
	})
	assertDirectoryGoFileBudget(t, moduleRoot, 0)
	assertDirectoryGoFileBudget(t, applicationRoot, 4)
	assertDirectoryGoFileBudget(t, transportRoot, 3)

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_channel_client.go"),
		filepath.Join(repositoryRoot, "internal", "service", "channel_client_service.go"),
		filepath.Join(repositoryRoot, "internal", "models", "channel_client.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "channel_client_repository.go"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "channelclient"),
		filepath.Join(repositoryRoot, "internal", "wiring", "channelclient"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy channel client path must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy channel client path: %v", err)
		}
	}
}
