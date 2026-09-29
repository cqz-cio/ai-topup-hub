package architecture

import (
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const moduleImportPath = "github.com/dujiao-next"

type importViolation struct {
	file       string
	importPath string
	reason     string
}

func TestDependencyRules(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	targets := []string{
		filepath.Join(repositoryRoot, "internal", "modules"),
		filepath.Join(repositoryRoot, "internal", "workflows"),
		filepath.Join(repositoryRoot, "internal", "platform"),
		filepath.Join(repositoryRoot, "internal", "shared"),
	}

	var violations []importViolation
	for _, target := range targets {
		found, err := inspectImports(repositoryRoot, target)
		if err != nil {
			t.Fatalf("inspect imports below %s: %v", target, err)
		}
		violations = append(violations, found...)
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].file == violations[j].file {
			return violations[i].importPath < violations[j].importPath
		}
		return violations[i].file < violations[j].file
	})

	for _, violation := range violations {
		t.Errorf("%s imports %q: %s", violation.file, violation.importPath, violation.reason)
	}
}

func TestValidateImportRules(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		importPath    string
		wantViolation bool
	}{
		{
			name:          "content application cannot import gorm",
			file:          "internal/modules/content/application/post_service.go",
			importPath:    "gorm.io/gorm",
			wantViolation: true,
		},
		{
			name:          "content application cannot import gin",
			file:          "internal/modules/content/application/post_service.go",
			importPath:    "github.com/gin-gonic/gin",
			wantViolation: true,
		},
		{
			name:          "content application cannot import legacy repository",
			file:          "internal/modules/content/application/post_service.go",
			importPath:    moduleImportPath + "/internal/repository",
			wantViolation: true,
		},
		{
			name:          "gorm store can import gorm",
			file:          "internal/modules/content/infrastructure/gormstore/post_store.go",
			importPath:    "gorm.io/gorm",
			wantViolation: false,
		},
		{
			name:          "gorm store cannot import legacy service",
			file:          "internal/modules/content/infrastructure/gormstore/post_store.go",
			importPath:    moduleImportPath + "/internal/service",
			wantViolation: true,
		},
		{
			name:          "content application cannot import shared models after complete migration",
			file:          "internal/modules/content/application/post_service.go",
			importPath:    moduleImportPath + "/internal/models",
			wantViolation: true,
		},
		{
			name:          "content transport can import gin",
			file:          "internal/modules/content/transport/http/public_handler.go",
			importPath:    "github.com/gin-gonic/gin",
			wantViolation: false,
		},
		{
			name:          "content transport cannot import repository",
			file:          "internal/modules/content/transport/http/public_handler.go",
			importPath:    moduleImportPath + "/internal/repository",
			wantViolation: true,
		},
		{
			name:          "content transport cannot import service",
			file:          "internal/modules/content/transport/http/public_handler.go",
			importPath:    moduleImportPath + "/internal/service",
			wantViolation: true,
		},
		{
			name:          "content transport cannot import provider container",
			file:          "internal/modules/content/transport/http/public_handler.go",
			importPath:    moduleImportPath + "/internal/provider",
			wantViolation: true,
		},
		{
			name:          "content transport cannot import app container",
			file:          "internal/modules/content/transport/http/public_handler.go",
			importPath:    moduleImportPath + "/internal/app/container",
			wantViolation: true,
		},
		{
			name:          "dashboard application cannot import legacy service",
			file:          "internal/modules/dashboard/application/service.go",
			importPath:    moduleImportPath + "/internal/service",
			wantViolation: true,
		},
		{
			name:          "dashboard gorm store can import gorm",
			file:          "internal/modules/dashboard/infrastructure/gormstore/store.go",
			importPath:    "gorm.io/gorm",
			wantViolation: false,
		},
		{
			name:          "nested catalog product gorm store can import gorm",
			file:          "internal/modules/catalog/product/store/gormstore/product_store.go",
			importPath:    "gorm.io/gorm",
			wantViolation: false,
		},
		{
			name:          "module infrastructure gorm store can import gorm",
			file:          "internal/modules/order/infrastructure/gormstore/order_store.go",
			importPath:    "gorm.io/gorm",
			wantViolation: false,
		},
		{
			name:          "module HTTP transport can import gin",
			file:          "internal/modules/order/transport/http/create_handler.go",
			importPath:    "github.com/gin-gonic/gin",
			wantViolation: false,
		},
		{
			name:          "module domain cannot import application",
			file:          "internal/modules/order/domain/order.go",
			importPath:    moduleImportPath + "/internal/modules/order/application",
			wantViolation: true,
		},
		{
			name:          "module application cannot import infrastructure",
			file:          "internal/modules/order/application/create.go",
			importPath:    moduleImportPath + "/internal/modules/order/infrastructure/gormstore",
			wantViolation: true,
		},
		{
			name:          "module transport cannot import infrastructure",
			file:          "internal/modules/order/transport/http/create_handler.go",
			importPath:    moduleImportPath + "/internal/modules/order/infrastructure/gormstore",
			wantViolation: true,
		},
		{
			name:          "platform cannot import a business module",
			file:          "internal/platform/database/gormdb/database.go",
			importPath:    moduleImportPath + "/internal/modules/order/domain",
			wantViolation: true,
		},
		{
			name:          "shared code cannot import platform",
			file:          "internal/shared/money/money.go",
			importPath:    moduleImportPath + "/internal/platform/database",
			wantViolation: true,
		},
		{
			name:          "workflow application cannot import gorm",
			file:          "internal/workflows/checkout/application/checkout.go",
			importPath:    "gorm.io/gorm",
			wantViolation: true,
		},
		{
			name:          "workflow gorm unit of work can import gorm",
			file:          "internal/workflows/checkout/infrastructure/gormuow/unit_of_work.go",
			importPath:    "gorm.io/gorm",
			wantViolation: false,
		},
		{
			name:          "dashboard transport cannot import legacy repository",
			file:          "internal/modules/dashboard/transport/http/admin_handler.go",
			importPath:    moduleImportPath + "/internal/repository",
			wantViolation: true,
		},
		{
			name:          "content black box integration test can import gorm",
			file:          "internal/modules/content/integrationtest/http/admin_integration_test.go",
			importPath:    "gorm.io/gorm",
			wantViolation: false,
		},
		{
			name:          "module integration test package can import gorm",
			file:          "internal/modules/identity/userauth/integrationtest/auth_test.go",
			importPath:    "gorm.io/gorm",
			wantViolation: false,
		},
		{
			name:          "module integration test can assemble legacy repository during migration",
			file:          "internal/modules/catalog/product/integrationtest/application/product_test_helpers_test.go",
			importPath:    moduleImportPath + "/internal/repository",
			wantViolation: false,
		},
		{
			name:          "application white box test can assemble infrastructure",
			file:          "internal/modules/order/application/order_service_status_test.go",
			importPath:    moduleImportPath + "/internal/modules/order/infrastructure/gormstore",
			wantViolation: false,
		},
		{
			name:          "application white box test can import gorm",
			file:          "internal/modules/order/application/order_service_status_test.go",
			importPath:    "gorm.io/gorm",
			wantViolation: false,
		},
		{
			name:          "module integration test can assemble infrastructure adapters",
			file:          "internal/modules/catalog/product/integrationtest/transport/admin_http_test.go",
			importPath:    moduleImportPath + "/internal/modules/catalog/product/store/gormstore",
			wantViolation: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reason := validateImport(test.file, test.importPath)
			if test.wantViolation && reason == "" {
				t.Fatalf("expected %s importing %q to violate a dependency rule", test.file, test.importPath)
			}
			if !test.wantViolation && reason != "" {
				t.Fatalf("expected %s importing %q to be allowed, got: %s", test.file, test.importPath, reason)
			}
		})
	}
}

func TestInspectImportsRejectsForbiddenDependency(t *testing.T) {
	repositoryRoot := t.TempDir()
	contentRoot := filepath.Join(repositoryRoot, "internal", "modules", "content", "application")
	if err := os.MkdirAll(contentRoot, 0o755); err != nil {
		t.Fatalf("create synthetic content module: %v", err)
	}

	source := []byte("package application\n\nimport _ \"gorm.io/gorm\"\n")
	if err := os.WriteFile(filepath.Join(contentRoot, "post_service.go"), source, 0o600); err != nil {
		t.Fatalf("write synthetic content source: %v", err)
	}

	violations, err := inspectImports(repositoryRoot, contentRoot)
	if err != nil {
		t.Fatalf("inspect synthetic content module: %v", err)
	}
	if len(violations) != 1 {
		t.Fatalf("expected one dependency violation, got %d: %#v", len(violations), violations)
	}
	if violations[0].importPath != "gorm.io/gorm" {
		t.Fatalf("expected GORM violation, got %#v", violations[0])
	}
}

func findRepositoryRoot(t *testing.T) string {
	t.Helper()

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve architecture test filename")
	}

	return filepath.Clean(filepath.Join(filepath.Dir(filename), "..", ".."))
}

func inspectImports(repositoryRoot, target string) ([]importViolation, error) {
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var violations []importViolation
	err := filepath.WalkDir(target, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" {
			return nil
		}

		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		relativePath, err := filepath.Rel(repositoryRoot, path)
		if err != nil {
			return fmt.Errorf("resolve relative path for %s: %w", path, err)
		}
		relativePath = filepath.ToSlash(relativePath)

		for _, imported := range parsed.Imports {
			importPath, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return fmt.Errorf("unquote import %s in %s: %w", imported.Path.Value, path, err)
			}
			if reason := validateImport(relativePath, importPath); reason != "" {
				violations = append(violations, importViolation{
					file:       relativePath,
					importPath: importPath,
					reason:     reason,
				})
			}
		}

		return nil
	})

	return violations, err
}

func validateImport(file, importPath string) string {
	file = filepath.ToSlash(file)

	if pathWithin(file, "internal/modules") {
		integrationTest := isModuleIntegrationTest(file)
		whiteBoxTest := strings.HasSuffix(file, "_test.go")
		if pathWithin(file, "internal/modules/content") && importMatches(importPath, moduleImportPath+"/internal/models") {
			return "the completed Content vertical must own its domain models"
		}
		if forbiddenLegacyImport(importPath) && !integrationTest {
			return "domain modules must not depend on legacy service, repository, HTTP, router, or provider packages"
		}
		if importMatches(importPath, "github.com/gin-gonic/gin") && !isHTTPTransport(file) && !isModuleIntegrationTest(file) {
			return "HTTP transport belongs outside domain modules"
		}
		if strings.HasPrefix(importPath, "gorm.io/") && !isGormStore(file) && !integrationTest && !whiteBoxTest {
			return "only a module's store/gormstore adapter may import GORM"
		}
		if !integrationTest && isLayer(file, "domain") && importsAnyLayer(importPath, "application", "infrastructure", "store", "transport") {
			return "domain code must not depend on application, infrastructure, store, or transport packages"
		}
		if !integrationTest && isLayer(file, "application") {
			if importMatches(importPath, "github.com/gin-gonic/gin") || strings.HasPrefix(importPath, "github.com/hibiken/asynq") {
				return "application code must not depend on HTTP or job transport libraries"
			}
			if (!whiteBoxTest && importsAnyLayer(importPath, "infrastructure", "store")) || importsAnyLayer(importPath, "transport") || importMatches(importPath, moduleImportPath+"/internal/bootstrap") {
				return "application code must depend on ports, not infrastructure, transport, or bootstrap packages"
			}
		}
		if !integrationTest && isLayer(file, "transport") && importsAnyLayer(importPath, "infrastructure", "store") {
			return "transport code must depend on application contracts, not concrete stores or infrastructure"
		}
		if !integrationTest && isLayer(file, "infrastructure") && (importsAnyLayer(importPath, "transport") || importMatches(importPath, moduleImportPath+"/internal/bootstrap")) {
			return "infrastructure code must not depend on transport or bootstrap packages"
		}
	}

	if pathWithin(file, "internal/workflows") {
		if forbiddenLegacyImport(importPath) {
			return "workflows must not depend on legacy service, repository, HTTP, router, or provider packages"
		}
		if strings.HasPrefix(importPath, "gorm.io/") && !isWorkflowGormUnitOfWork(file) {
			return "only a workflow infrastructure/gormuow adapter may import GORM"
		}
		if isLayer(file, "application") && (importMatches(importPath, "github.com/gin-gonic/gin") || strings.HasPrefix(importPath, "github.com/hibiken/asynq") || importsAnyLayer(importPath, "infrastructure", "transport")) {
			return "workflow application code must depend on contracts and ports, not transport or infrastructure"
		}
	}

	if pathWithin(file, "internal/platform") {
		for _, forbidden := range []string{
			moduleImportPath + "/internal/modules",
			moduleImportPath + "/internal/workflows",
			moduleImportPath + "/internal/bootstrap",
		} {
			if importMatches(importPath, forbidden) {
				return "platform code must not depend on business modules, workflows, or bootstrap"
			}
		}
	}

	if pathWithin(file, "internal/shared") {
		for _, forbidden := range []string{
			moduleImportPath + "/internal/modules",
			moduleImportPath + "/internal/workflows",
			moduleImportPath + "/internal/platform",
			moduleImportPath + "/internal/bootstrap",
		} {
			if importMatches(importPath, forbidden) {
				return "shared code must remain independent from modules, workflows, platform, and bootstrap"
			}
		}
		if strings.HasPrefix(importPath, "gorm.io/") || importMatches(importPath, "github.com/gin-gonic/gin") || strings.HasPrefix(importPath, "github.com/hibiken/asynq") {
			return "shared code must not depend on persistence or transport frameworks"
		}
	}

	if pathWithin(file, "internal/transport/http") {
		for _, forbidden := range []string{
			moduleImportPath + "/internal/repository",
			moduleImportPath + "/internal/service",
			moduleImportPath + "/internal/provider",
		} {
			if importMatches(importPath, forbidden) {
				return "HTTP transport must depend on narrow use-case interfaces, not legacy implementations or the provider container"
			}
		}
		if strings.HasPrefix(importPath, "gorm.io/") && !strings.HasSuffix(file, "_integration_test.go") {
			return "HTTP transport must not depend on GORM"
		}
	}

	return ""
}

func forbiddenLegacyImport(importPath string) bool {
	for _, forbidden := range []string{
		moduleImportPath + "/internal/service",
		moduleImportPath + "/internal/repository",
		moduleImportPath + "/internal/http",
		moduleImportPath + "/internal/router",
		moduleImportPath + "/internal/provider",
		moduleImportPath + "/internal/app/container",
	} {
		if importMatches(importPath, forbidden) {
			return true
		}
	}
	return false
}

func importMatches(importPath, forbidden string) bool {
	return importPath == forbidden || strings.HasPrefix(importPath, forbidden+"/")
}

func pathWithin(file, directory string) bool {
	return file == directory || strings.HasPrefix(file, directory+"/")
}

func isGormStore(file string) bool {
	parts := strings.Split(filepath.ToSlash(file), "/")
	if len(parts) < 6 || parts[0] != "internal" || parts[1] != "modules" {
		return false
	}
	for index := 2; index+1 < len(parts); index++ {
		if (parts[index] == "store" || parts[index] == "infrastructure") && parts[index+1] == "gormstore" {
			return true
		}
	}
	return false
}

func isModuleIntegrationTest(file string) bool {
	file = filepath.ToSlash(file)
	return strings.HasSuffix(file, "_test.go") && isLayer(file, "integrationtest")
}

func isHTTPTransport(file string) bool {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for index := 0; index+1 < len(parts); index++ {
		if parts[index] == "transport" && parts[index+1] == "http" {
			return true
		}
	}
	return false
}

func isWorkflowGormUnitOfWork(file string) bool {
	parts := strings.Split(filepath.ToSlash(file), "/")
	for index := 0; index+1 < len(parts); index++ {
		if parts[index] == "infrastructure" && parts[index+1] == "gormuow" {
			return true
		}
	}
	return false
}

func isLayer(path, layer string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	for _, part := range parts {
		if part == layer {
			return true
		}
	}
	return false
}

func importsAnyLayer(importPath string, layers ...string) bool {
	parts := strings.Split(strings.Trim(importPath, "/"), "/")
	for _, part := range parts {
		for _, layer := range layers {
			if part == layer {
				return true
			}
		}
	}
	return false
}
