package architecture

import (
	"path/filepath"
	"testing"
)

func TestResellerModuleOwnsCompleteVerticalSlice(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "reseller")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("reseller module root must remain structural only, got production=%d total=%d", production, total)
	}

	domainRoot := filepath.Join(moduleRoot, "domain")
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "profile.go"), []string{"Profile"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "site.go"), []string{"Domain", "SiteConfig"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "accounting.go"), []string{"LedgerEntry", "WithdrawRequest", "BalanceAccount"})
	assertDirectoryGoFileBudget(t, domainRoot, 7)

	applicationRoot := filepath.Join(moduleRoot, "application")
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "management.go"), []string{"ManagementService"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "accounting_query.go"), []string{"AccountingQueryService"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "accounting_withdraw.go"), []string{"AccountingWithdrawService"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "accounting_profit.go"), []string{"AccountingLedgerService"})
	assertDirectoryGoFileBudget(t, applicationRoot, 17)

	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "store.go"), []string{"New", "Migrate"})
	assertDirectoryGoFileBudget(t, storeRoot, 18)

	transportRoot := filepath.Join(moduleRoot, "transport", "http")
	assertDirectoryGoFileBudget(t, filepath.Join(transportRoot, "admin"), 10)
	assertDirectoryGoFileBudget(t, filepath.Join(transportRoot, "user"), 8)
	assertDirectoryGoFileBudget(t, filepath.Join(transportRoot, "presenter"), 4)
	assertDirectoryGoFileBudget(t, filepath.Join(transportRoot, "shared"), 1)
}
