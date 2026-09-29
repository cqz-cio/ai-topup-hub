package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPublicCaptchaHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "captcha")
	applicationRoot := filepath.Join(moduleRoot, "application")
	contractRoot := filepath.Join(moduleRoot, "contract")
	turnstileRoot := filepath.Join(moduleRoot, "infrastructure", "turnstile")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresFunctions(t, filepath.Join(applicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{"SettingReader", "TurnstileVerifier"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "types.go"), []string{"VerifyPayload", "ImageChallenge"})
	assertFileDeclaresTypes(t, filepath.Join(turnstileRoot, "client.go"), []string{"Client"})
	assertFileDeclaresFunctions(t, filepath.Join(turnstileRoot, "client.go"), []string{"New"})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterPublicRoutes"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "public_handler.go"), []string{
		"PublicHandler", "ImageChallengeGenerator",
	})
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("captcha module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, applicationRoot, 2)
	assertDirectoryGoFileBudget(t, contractRoot, 4)
	assertDirectoryGoFileBudget(t, turnstileRoot, 3)
	assertDirectoryGoFileBudget(t, transportRoot, 6)
	assertProductionImportsAbsent(t, applicationRoot, "net/http")
	assertProductionImportsAbsent(t, applicationRoot, "net/url")
	assertProductionImportsAbsent(t, contractRoot, "net/http")

	legacy := filepath.Join(repositoryRoot, "internal", "http", "handlers", "public", "captcha.go")
	if _, err := os.Stat(legacy); err == nil {
		t.Fatalf("legacy public captcha handler must stay removed: %s", legacy)
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat legacy captcha handler: %v", err)
	}
	for _, relativePath := range []string{
		"internal/service/captcha_service.go",
		"internal/service/captcha_setting.go",
	} {
		path := filepath.Join(repositoryRoot, filepath.FromSlash(relativePath))
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("legacy captcha service file must stay removed: %s", relativePath)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat %s: %v", relativePath, err)
		}
	}
}
