package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGiftCardImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "giftcard")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	redeemTransactionRoot := filepath.Join(
		repositoryRoot,
		"internal", "workflows", "giftcardredeem", "infrastructure", "gormuow",
	)
	settingsCurrencyRoot := filepath.Join(moduleRoot, "infrastructure", "settingscurrency")
	integrationTestRoot := filepath.Join(moduleRoot, "integrationtest")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")
	presenterRoot := filepath.Join(moduleRoot, "transport", "presenter")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "card.go"), []string{"GiftCard"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "batch.go"), []string{"GiftCardBatch"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "repository.go"), []string{
		"ListFilter", "Repository", "WalletCreditInput", "RedeemTransaction",
		"RedeemTransactionRunner", "UserDirectory", "CurrencyProvider",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "generate.go"), []string{"Generate"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "manage.go"), []string{
		"List", "Update", "Delete", "BatchUpdateStatus", "ResolveRedeemedUsers",
	})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "redeem.go"), []string{"RedeemGiftCard"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "export.go"), []string{"Export"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresTypes(t, filepath.Join(redeemTransactionRoot, "runner.go"), []string{"Runner"})
	assertFileDeclaresTypes(t, filepath.Join(settingsCurrencyRoot, "provider.go"), []string{"Provider"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{
		"RegisterAdminRoutes", "RegisterUserRoutes", "RegisterChannelRoutes",
	})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("giftcard module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, domainRoot, 3)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, applicationRoot, 9)
	assertDirectoryGoFileBudget(t, storeRoot, 3)
	assertDirectoryGoFileBudget(t, redeemTransactionRoot, 2)
	assertDirectoryGoFileBudget(t, settingsCurrencyRoot, 2)
	assertDirectoryGoFileBudget(t, integrationTestRoot, 3)
	assertDirectoryGoFileBudget(t, transportRoot, 6)
	assertDirectoryGoFileBudget(t, presenterRoot, 2)
}

func TestGiftCardLegacyRepositoryFileStaysRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	paths := []string{
		filepath.Join(repositoryRoot, "internal", "repository", "gift_card_repository.go"),
		filepath.Join(repositoryRoot, "internal", "models", "gift_card.go"),
		filepath.Join(repositoryRoot, "internal", "models", "gift_card_batch.go"),
		filepath.Join(repositoryRoot, "internal", "service", "gift_card_service.go"),
		filepath.Join(repositoryRoot, "internal", "service", "gift_card_store_adapter.go"),
		filepath.Join(repositoryRoot, "internal", "dto", "gift_card.go"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "giftcard"),
		filepath.Join(repositoryRoot, "internal", "modules", "giftcard", "store"),
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("legacy gift card path must stay removed: %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", path, err)
		}
	}
}
