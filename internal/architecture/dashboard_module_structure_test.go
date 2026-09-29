package architecture

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestDashboardImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "dashboard")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{
		"NewService", "GetOverview", "GetTrends", "GetRankings",
		"LoadDashboardAlertSetting", "GetInventoryAlertItems", "GetPaymentOrderAlertCounts",
	})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"Repository", "SettingReader", "OverviewRow", "PaymentOrderAlertCountsRow",
		"OrderTrendRow", "PaymentTrendRow", "ProfitOverviewRow", "ProfitTrendRow",
		"StockStatsRow", "InventoryAlertRow", "ProductRankingRow", "ChannelRankingRow",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "types.go"), []string{
		"OverviewResponse", "KPI", "Funnel", "AlertItem", "TrendResponse",
		"TrendPoint", "RankingsResponse", "ProductRanking", "ChannelRanking",
	})

	storeFunctions := map[string][]string{
		"store.go":     {"New", "paidOrderStatuses", "onlinePaymentBase"},
		"overview.go":  {"GetOverview", "GetPaymentOrderAlertCounts", "GetTotalUserBalance"},
		"trend.go":     {"GetOrderTrends", "GetPaymentTrends"},
		"inventory.go": {"GetStockStats", "GetInventoryAlertItems"},
		"product.go":   {"GetTopProducts"},
		"profit.go":    {"GetProfitOverview", "GetProfitTrends"},
		"channel.go":   {"GetTopChannels"},
	}
	for file, functions := range storeFunctions {
		assertFileDeclaresFunctions(t, filepath.Join(storeRoot, file), functions)
	}

	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("dashboard module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 4)
	assertDirectoryGoFileBudget(t, contractRoot, 2)
	assertDirectoryGoFileBudget(t, storeRoot, 16)
	assertDirectoryGoFileBudget(t, transportRoot, 6)
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/models")
	assertProductionImportsAbsent(t, transportRoot, moduleImportPath+"/internal/models")
}

func TestDashboardLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	patterns := []string{
		filepath.Join(repositoryRoot, "internal", "repository", "dashboard_repository*.go"),
		filepath.Join(repositoryRoot, "internal", "service", "dashboard_service*.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_dashboard.go"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(matches) != 0 {
			t.Errorf("legacy dashboard files must stay removed: %v", matches)
		}
	}
}

func assertFileDeclaresFunctions(t *testing.T, path string, required []string) {
	t.Helper()
	parsed := parseProductionGoFile(t, path)
	declared := declaredFunctionNames(parsed)
	set := make(map[string]struct{}, len(declared))
	for _, name := range declared {
		set[name] = struct{}{}
	}
	for _, name := range required {
		if _, ok := set[name]; !ok {
			t.Errorf("%s must declare function %s; got %v", path, name, declared)
		}
	}
}

func assertFileDeclaresTypes(t *testing.T, path string, required []string) {
	t.Helper()
	parsed := parseProductionGoFile(t, path)
	declared := declaredTypeNames(parsed)
	set := make(map[string]struct{}, len(declared))
	for _, name := range declared {
		set[name] = struct{}{}
	}
	for _, name := range required {
		if _, ok := set[name]; !ok {
			t.Errorf("%s must declare type %s; got %v", path, name, declared)
		}
	}
}

func assertDirectoryGoFileBudget(t *testing.T, directory string, maximum int) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read directory %s: %v", directory, err)
	}
	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}
		files = append(files, entry.Name())
	}
	sort.Strings(files)
	if len(files) > maximum {
		t.Errorf("%s exceeds Go file budget %d: %d files (%v)", directory, maximum, len(files), files)
	}
}
