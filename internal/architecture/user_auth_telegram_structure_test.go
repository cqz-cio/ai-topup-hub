package architecture

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUserAuthTelegramServiceIsSplitByFlow(t *testing.T) {
	repositoryRoot := findRepositoryRoot(t)
	legacyPath := filepath.Join(repositoryRoot, "internal", "service", "user_auth_service_oauth.go")
	if _, err := os.Stat(legacyPath); err == nil {
		t.Fatalf("user_auth_service_oauth.go must be replaced by Telegram-flow-focused service files")
	} else if !os.IsNotExist(err) {
		t.Fatalf("stat user_auth_service_oauth.go: %v", err)
	}

	expectedOwner := map[string]string{
		"LoginWithTelegram":                     "telegram_login.go",
		"LoginWithTelegramMiniApp":              "telegram_login.go",
		"LoginVerifiedTelegram":                 "telegram_login.go",
		"StartTelegramOIDC":                     "telegram_oidc.go",
		"LoginWithTelegramOIDC":                 "telegram_oidc.go",
		"BindTelegramOIDC":                      "telegram_oidc.go",
		"BindTelegram":                          "telegram_binding.go",
		"BindTelegramMiniApp":                   "telegram_binding.go",
		"bindVerifiedTelegram":                  "telegram_binding.go",
		"UnbindTelegram":                        "telegram_binding.go",
		"GetTelegramBinding":                    "telegram_binding.go",
		"ResolveTelegramChannelIdentity":        "telegram_channel.go",
		"ProvisionTelegramChannelIdentity":      "telegram_channel.go",
		"BindTelegramChannelByEmailCode":        "telegram_channel.go",
		"resolveTelegramChannelIdentity":        "telegram_channel.go",
		"provisionTelegramChannelIdentity":      "telegram_channel.go",
		"bindTelegramIdentityToUser":            "telegram_channel.go",
		"normalizeTelegramChannelIdentityInput": "telegram_channel.go",
		"getActiveUserByID":                     "telegram_identity.go",
		"findOrCreateTelegramUser":              "telegram_identity.go",
		"getTelegramIdentityByVerifiedID":       "telegram_identity.go",
		"canonicalizeTelegramProviderUserID":    "telegram_identity.go",
		"telegramProviderUserIDMatchesVerified": "telegram_identity.go",
		"applyTelegramIdentity":                 "telegram_identity.go",
	}
	expectedTypeOwner := map[string]string{
		"LoginWithTelegramInput":              "telegram_login.go",
		"LoginWithTelegramMiniAppInput":       "telegram_login.go",
		"StartTelegramOIDCInput":              "telegram_oidc.go",
		"LoginWithTelegramOIDCInput":          "telegram_oidc.go",
		"BindTelegramOIDCInput":               "telegram_oidc.go",
		"BindTelegramInput":                   "telegram_binding.go",
		"BindTelegramMiniAppInput":            "telegram_binding.go",
		"TelegramChannelIdentityInput":        "telegram_channel.go",
		"BindTelegramChannelByEmailCodeInput": "telegram_channel.go",
	}

	files := []string{
		"telegram_login.go",
		"telegram_oidc.go",
		"telegram_binding.go",
		"telegram_channel.go",
		"telegram_identity.go",
	}
	actualOwners := make(map[string][]string, len(expectedOwner))
	actualTypeOwners := make(map[string][]string, len(expectedTypeOwner))
	for _, file := range files {
		parsed := parseProductionGoFile(t, filepath.Join(repositoryRoot, "internal", "modules", "identity", "userauth", "application", file))
		for _, function := range declaredFunctionNames(parsed) {
			if _, tracked := expectedOwner[function]; tracked {
				actualOwners[function] = append(actualOwners[function], file)
			}
		}
		for _, typeName := range declaredTypeNames(parsed) {
			if _, tracked := expectedTypeOwner[typeName]; tracked {
				actualTypeOwners[typeName] = append(actualTypeOwners[typeName], file)
			}
		}
	}

	for function, wantFile := range expectedOwner {
		gotFiles := actualOwners[function]
		if len(gotFiles) != 1 || gotFiles[0] != wantFile {
			t.Errorf("%s ownership mismatch: want [%s], got %v", function, wantFile, gotFiles)
		}
	}
	for typeName, wantFile := range expectedTypeOwner {
		gotFiles := actualTypeOwners[typeName]
		if len(gotFiles) != 1 || gotFiles[0] != wantFile {
			t.Errorf("%s ownership mismatch: want [%s], got %v", typeName, wantFile, gotFiles)
		}
	}
}
