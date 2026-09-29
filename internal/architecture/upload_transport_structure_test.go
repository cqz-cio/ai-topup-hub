package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUploadAdminHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "upload")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "localstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service", "Policy"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{"Result", "StoreInput", "Store"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "errors.go"), []string{"ValidationError"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "store.go"), []string{"Store"})
	assertFileDeclaresFunctions(t, filepath.Join(storeRoot, "store.go"), []string{"New"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"FileUploader", "MediaRecorder", "AdminHandler",
	})
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("upload module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 3)
	assertDirectoryGoFileBudget(t, contractRoot, 3)
	assertDirectoryGoFileBudget(t, storeRoot, 2)
	assertDirectoryGoFileBudget(t, transportRoot, 4)
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/config")
	assertProductionImportsAbsent(t, applicationRoot, moduleImportPath+"/internal/transport")
	assertProductionImportsAbsent(t, contractRoot, moduleImportPath+"/internal/transport")

	for _, relativePath := range []string{
		"internal/http/handlers/admin/admin_upload.go",
		"internal/http/handlers/admin/admin_upload_test.go",
		"internal/service/upload_service.go",
		"internal/wiring/upload/wiring.go",
	} {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err == nil {
			t.Errorf("legacy upload handler must stay removed: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("inspect %s: %v", relativePath, err)
		}
	}
}
