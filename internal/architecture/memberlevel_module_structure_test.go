package architecture

import (
	"path/filepath"
	"testing"
)

func TestMemberLevelImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "memberlevel")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")
	presenterRoot := filepath.Join(moduleRoot, "transport", "presenter")
	integrationTestRoot := filepath.Join(moduleRoot, "integrationtest")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "level.go"), []string{"MemberLevel"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "price.go"), []string{"MemberLevelPrice"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{
		"ListFilter", "LevelRepository", "PriceRepository", "UserRepository",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{
		"NewService", "CreateLevel", "UpdateLevel", "DeleteLevel", "ResolveMemberPrice",
		"CheckAndUpgrade", "OnRechargeCompleted", "OnOrderPaid", "AssignDefaultLevel",
		"SetUserLevel", "BackfillDefaultLevel",
	})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "level_store.go"), []string{"LevelStore"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "price_store.go"), []string{"PriceStore"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "user_store.go"), []string{"UserStore"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterAdminRoutes", "RegisterPublicRoutes", "RegisterChannelRoutes",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "list_handler.go"), []string{
		"NewPublicHandler", "NewChannelHandler",
	})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("memberlevel module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, domainRoot, 3)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, applicationRoot, 2)
	assertDirectoryGoFileBudget(t, storeRoot, 5)
	assertDirectoryGoFileBudget(t, transportRoot, 5)
	assertDirectoryGoFileBudget(t, presenterRoot, 2)
	assertDirectoryGoFileBudget(t, integrationTestRoot, 2)
}

func TestMemberLevelLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "memberlevel")
	patterns := []string{
		filepath.Join(repositoryRoot, "internal", "models", "member_level*.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "member_level*.go"),
		filepath.Join(repositoryRoot, "internal", "service", "member_level_service*.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_member_level.go"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "memberlevel"),
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
			t.Errorf("legacy member-level files must stay removed: %v", matches)
		}
	}
}
