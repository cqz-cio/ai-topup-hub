package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTelegramBroadcastHTTPLivesInTransport(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	moduleRoot := filepath.Join(repositoryRoot, "internal", "modules", "telegram")
	broadcastRoot := filepath.Join(moduleRoot, "broadcast")
	broadcastTransportRoot := filepath.Join(broadcastRoot, "transport", "http")
	notifyRoot := filepath.Join(moduleRoot, "notify")
	notifyApplicationRoot := filepath.Join(notifyRoot, "application")
	notifyContractRoot := filepath.Join(notifyRoot, "contract")
	notifyBotAPIRoot := filepath.Join(notifyRoot, "infrastructure", "botapi")
	channelBotTransportRoot := filepath.Join(moduleRoot, "channelbot", "transport", "http")

	assertFileDeclaresTypes(t, filepath.Join(notifyApplicationRoot, "service.go"), []string{"Service"})
	assertFileDeclaresFunctions(t, filepath.Join(notifyApplicationRoot, "service.go"), []string{"NewService"})
	assertFileDeclaresTypes(t, filepath.Join(notifyContractRoot, "ports.go"), []string{"SettingReader", "Sender"})
	assertFileDeclaresTypes(t, filepath.Join(notifyContractRoot, "types.go"), []string{"SendOptions"})
	assertFileDeclaresTypes(t, filepath.Join(notifyBotAPIRoot, "client.go"), []string{"Client"})
	assertFileDeclaresFunctions(t, filepath.Join(notifyBotAPIRoot, "client.go"), []string{"New", "NewWithHTTPClient"})
	assertFileDeclaresFunctions(t, filepath.Join(channelBotTransportRoot, "routes.go"), []string{
		"RegisterChannelBotRoutes",
	})
	assertFileDeclaresFunctions(t, filepath.Join(broadcastTransportRoot, "routes.go"), []string{
		"RegisterAdminRoutes",
	})
	assertFileDeclaresTypes(t, filepath.Join(broadcastTransportRoot, "handler.go"), []string{
		"AdminHandler", "AdminService",
	})
	assertFileDeclaresTypes(t, filepath.Join(channelBotTransportRoot, "channel_bot_handler.go"), []string{
		"ChannelBotHandler", "BotSettings", "ChannelBotTokenProvider",
	})
	production, total := countDirectGoFiles(t, moduleRoot)
	if production != 0 || total != 0 {
		t.Fatalf("telegram module root must remain structural only, got production=%d total=%d", production, total)
	}
	assertDirectoryGoFileBudget(t, notifyApplicationRoot, 2)
	assertDirectoryGoFileBudget(t, notifyContractRoot, 4)
	assertDirectoryGoFileBudget(t, notifyBotAPIRoot, 3)
	assertDirectoryGoFileBudget(t, broadcastTransportRoot, 2)
	assertDirectoryGoFileBudget(t, channelBotTransportRoot, 3)
	assertProductionImportsAbsent(t, notifyApplicationRoot, "net/http")
	assertProductionImportsAbsent(t, notifyApplicationRoot, "os")

	for _, legacy := range []string{
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "admin", "admin_telegram_broadcast.go"),
		filepath.Join(repositoryRoot, "internal", "http", "handlers", "channel", "channel_telegram_bot.go"),
		filepath.Join(repositoryRoot, "internal", "service", "telegram_notify_service.go"),
		filepath.Join(repositoryRoot, "internal", "models", "telegram_broadcast.go"),
		filepath.Join(repositoryRoot, "internal", "repository", "telegram_broadcast_repository.go"),
		filepath.Join(repositoryRoot, "internal", "service", "telegram_broadcast_service.go"),
		filepath.Join(repositoryRoot, "internal", "transport", "http", "telegram", "admin_broadcast_handler.go"),
		filepath.Join(repositoryRoot, "internal", "wiring", "telegram", "broadcast.go"),
	} {
		if _, err := os.Stat(legacy); err == nil {
			t.Fatalf("legacy telegram handler must stay removed: %s", legacy)
		} else if !os.IsNotExist(err) {
			t.Fatalf("stat legacy telegram handler: %v", err)
		}
	}
}
