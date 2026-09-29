package architecture

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestHTTPServerSeparatesRoutesFromMiddleware(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	httpServerRoot := filepath.Join(repositoryRoot, "internal", "app", "httpserver")
	middlewareRoot := filepath.Join(httpServerRoot, "middleware")

	adapters, err := filepath.Glob(filepath.Join(httpServerRoot, "*_adapter.go"))
	if err != nil {
		t.Fatalf("list router adapters: %v", err)
	}
	if len(adapters) != 0 {
		t.Fatalf("composition adapters belong in internal/bootstrap, not internal/router: %v", adapters)
	}

	entries, err := os.ReadDir(httpServerRoot)
	if err != nil {
		t.Fatalf("read HTTP server directory: %v", err)
	}
	productionFiles := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		productionFiles = append(productionFiles, entry.Name())
	}
	sort.Strings(productionFiles)
	if len(productionFiles) > 6 {
		t.Fatalf("HTTP server route assembly file budget exceeded: got %d files: %v", len(productionFiles), productionFiles)
	}
	assertDirectoryGoFileBudget(t, httpServerRoot, 10)
	assertDirectoryGoFileBudget(t, middlewareRoot, 16)
}

func TestBootstrapPackagesStayFocused(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	bootstrapRoot := filepath.Join(repositoryRoot, "internal", "bootstrap")
	entries, err := os.ReadDir(bootstrapRoot)
	if err != nil {
		t.Fatalf("read bootstrap directory: %v", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t.Run(entry.Name(), func(t *testing.T) {
			budget := 4
			if entry.Name() == "settingshttp" {
				// Dedicated adapters keep independently hot-reloadable security
				// settings (SMTP, captcha, Telegram, Google) out of the router;
				// the Google adapter has a focused concurrency regression test.
				budget = 6
			}
			assertDirectoryGoFileBudget(t, filepath.Join(bootstrapRoot, entry.Name()), budget)
		})
	}
}
