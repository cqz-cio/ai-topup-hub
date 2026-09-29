package architecture

import (
	"path/filepath"
	"testing"
)

func TestNotificationImplementationLivesInBoundedContextDirectories(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "notification")
	domainRoot := filepath.Join(moduleRoot, "domain")
	contractRoot := filepath.Join(moduleRoot, "contract")
	applicationRoot := filepath.Join(moduleRoot, "application")
	formatRoot := filepath.Join(applicationRoot, "format")
	storeRoot := filepath.Join(moduleRoot, "infrastructure", "gormstore")
	transportRoot := filepath.Join(moduleRoot, "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(domainRoot, "log.go"), []string{"NotificationLog"})
	assertFileDeclaresTypes(t, filepath.Join(contractRoot, "ports.go"), []string{
		"SettingsReader", "EmailSender", "DispatchQueue", "DashboardAlertReader", "TelegramSender", "FeishuSender",
		"OrderStatusEmailInput", "LogRepository", "LogListFilter", "EnqueueInput", "NotificationEnqueuer", "TestSendInput", "TestSender",
	})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresTypes(t, filepath.Join(applicationRoot, "log_service.go"), []string{"LogRecordInput", "LogService"})
	assertFileDeclaresTypes(t, filepath.Join(formatRoot, "order_format.go"), []string{"OrderItemCounts"})
	assertFileDeclaresTypes(t, filepath.Join(storeRoot, "log_store.go"), []string{"LogStore"})
	assertFileDeclaresTypes(t, filepath.Join(transportRoot, "admin_handler.go"), []string{
		"SettingsService", "LogService", "Sender", "AdminHandler",
	})
	assertFileDeclaresFunctions(t, filepath.Join(transportRoot, "routes.go"), []string{"RegisterAdminRoutes"})

	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("notification module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, domainRoot, 1)
	assertDirectoryGoFileBudget(t, contractRoot, 2)
	assertDirectoryGoFileBudget(t, applicationRoot, 6)
	assertDirectoryGoFileBudget(t, formatRoot, 6)
	assertDirectoryGoFileBudget(t, storeRoot, 3)
	assertDirectoryGoFileBudget(t, transportRoot, 3)
}

func TestNotificationLegacyFlatFilesStayRemoved(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	patterns := []string{
		filepath.Join(repositoryRoot, "internal", "service", "notification_service*.go"),
		filepath.Join(repositoryRoot, "internal", "service", "notification_log*.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "notification_log*.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_notification*.go"),
		filepath.Join(repositoryRoot, "internal", "models", "notification_log.go"),
		filepath.Join(repositoryRoot, "internal", "modules", "notification", "*.go"),
		filepath.Join(repositoryRoot, "internal", "modules", "notification", "store"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "notification"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatalf("glob %s: %v", pattern, err)
		}
		if len(matches) != 0 {
			t.Errorf("legacy notification files must stay removed: %v", matches)
		}
	}
}
