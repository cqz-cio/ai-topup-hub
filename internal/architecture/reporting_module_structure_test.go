package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReportingUsesCompleteVerticalLayout(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "reporting")
	applicationRoot := filepath.Join(moduleRoot, "application")
	domainRoot := filepath.Join(moduleRoot, "domain")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("reporting module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 2)
	assertDirectoryGoFileBudget(t, domainRoot, 2)
	assertDirectoryGoFileBudget(t, transportRoot, 2)

	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "resolver.go"), []string{"Resolve"})
	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "query.go"), []string{"Query", "Window"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "query.go"), []string{"ParseQuery"})
	assertProductionImportsAbsent(t, applicationRoot, "github.com/gin-gonic/gin")
	assertProductionImportsAbsent(t, domainRoot, "github.com/gin-gonic/gin")
	assertProductionImportsAbsent(t, domainRoot, moduleImportPath+"/internal/platform")

	for _, relativePath := range []string{
		"internal/modules/reporting/window.go",
		"internal/modules/reporting/window_test.go",
		"internal/http/handlers/shared/reporting_query.go",
		"internal/http/handlers/shared/reporting_query_test.go",
	} {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("legacy reporting path must stay removed: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", relativePath, err)
		}
	}
}
