package architecture

import (
	"path/filepath"
	"testing"
)

func TestApiCredentialImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "apicredential")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	integrationTestRoot := filepath.Join(moduleRoot, "integrationtest")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "credential.go"), []string{"ApiCredential"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{
		"ListFilter", "Repository",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{
		"NewService", "Apply", "Approve", "Reject", "SetActive", "SetActiveByUserID",
		"Regenerate", "RegenerateByUserID", "GetByUserID", "GetByID", "List", "Delete",
	})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterAdminRoutes", "RegisterUserRoutes",
	})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("apicredential module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, domainRoot, 2)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, applicationRoot, 2)
	assertDirectoryGoFileBudget(t, storeRoot, 3)
	assertDirectoryGoFileBudget(t, integrationTestRoot, 2)
	assertDirectoryGoFileBudget(t, transportRoot, 3)
}

func TestApiCredentialLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "apicredential")
	patterns := []string{
		filepath.Join(repositoryRoot, "internal", "models", "api_credential.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "api_credential*.go"),
		filepath.Join(repositoryRoot, "internal", "service", "api_credential_service*.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_api_credential.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "api_credential.go"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "apicredential"),
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
			t.Errorf("legacy API credential files must stay removed: %v", matches)
		}
	}
}
