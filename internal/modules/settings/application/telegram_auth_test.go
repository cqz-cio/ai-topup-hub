package settingsapp

import (
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

func TestNormalizeTelegramAuthSetting(t *testing.T) {
	setting := settingssecurity.NormalizeTelegramAuthSetting(settingssecurity.TelegramAuthSetting{
		BotUsername:        " @demo_bot ",
		MiniAppURL:         " https://example.com/mini-app ",
		LoginExpireSeconds: 0,
		ReplayTTLSeconds:   10,
	})

	if setting.BotUsername != "demo_bot" {
		t.Fatalf("expected normalized username demo_bot, got %q", setting.BotUsername)
	}
	if setting.LoginExpireSeconds != 300 {
		t.Fatalf("expected default login expire 300, got %d", setting.LoginExpireSeconds)
	}
	if setting.MiniAppURL != "https://example.com/mini-app" {
		t.Fatalf("expected normalized mini app url, got %q", setting.MiniAppURL)
	}
	if setting.ReplayTTLSeconds != 60 {
		t.Fatalf("expected minimum replay ttl 60, got %d", setting.ReplayTTLSeconds)
	}
}

func TestPatchTelegramAuthSettingKeepsTokenWhenEmpty(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)

	defaultCfg := config.TelegramAuthConfig{
		Enabled:            true,
		BotUsername:        "demo_bot",
		BotToken:           "secret-token",
		LoginExpireSeconds: 300,
		ReplayTTLSeconds:   300,
	}

	updated, err := svc.PatchTelegramAuthSetting(defaultCfg, settingssecurity.TelegramAuthSettingPatch{
		BotUsername:        ptrString("@new_bot"),
		BotToken:           ptrString(""),
		MiniAppURL:         ptrString(" https://example.com/mini-app "),
		LoginExpireSeconds: ptrInt(600),
		ReplayTTLSeconds:   ptrInt(900),
	})
	if err != nil {
		t.Fatalf("patch telegram auth setting failed: %v", err)
	}
	if updated.BotToken != "secret-token" {
		t.Fatalf("expected keep token secret-token, got %q", updated.BotToken)
	}
	if updated.BotUsername != "new_bot" {
		t.Fatalf("expected normalized username new_bot, got %q", updated.BotUsername)
	}
	if updated.MiniAppURL != "https://example.com/mini-app" {
		t.Fatalf("expected normalized mini app url, got %q", updated.MiniAppURL)
	}

	saved, ok := repo.store[constants.SettingKeyTelegramAuthConfig]
	if !ok {
		t.Fatalf("telegram auth setting was not saved")
	}
	if saved["bot_token"] != "secret-token" {
		t.Fatalf("expected saved token keep old value, got %v", saved["bot_token"])
	}
	if saved["mini_app_url"] != "https://example.com/mini-app" {
		t.Fatalf("expected saved mini app url, got %v", saved["mini_app_url"])
	}
}

func TestValidateTelegramAuthSetting(t *testing.T) {
	valid := settingssecurity.NormalizeTelegramAuthSetting(settingssecurity.TelegramAuthSetting{
		Enabled:            true,
		BotUsername:        "demo_bot",
		BotToken:           "secret",
		LoginExpireSeconds: 300,
		ReplayTTLSeconds:   300,
	})
	if err := settingssecurity.ValidateTelegramAuthSetting(valid); err != nil {
		t.Fatalf("expected valid telegram auth config, got error: %v", err)
	}

	invalid := valid
	invalid.BotToken = ""
	if err := settingssecurity.ValidateTelegramAuthSetting(invalid); err == nil {
		t.Fatal("expected validation error when enabled and token missing")
	}
}

func ptrInt(value int) *int {
	return &value
}

func TestTelegramLoginModeDetection(t *testing.T) {
	cases := []struct {
		name    string
		setting settingssecurity.TelegramAuthSetting
		want    settingssecurity.TelegramLoginMode
	}{
		{"disabled", settingssecurity.TelegramAuthSetting{Enabled: false, BotToken: "1:abc", ClientSecret: "s", OIDCRedirectURI: "https://x/cb"}, settingssecurity.TelegramLoginModeDisabled},
		{"widget", settingssecurity.TelegramAuthSetting{Enabled: true, BotUsername: "bot", BotToken: "1:abc"}, settingssecurity.TelegramLoginModeWidget},
		{"oidc", settingssecurity.TelegramAuthSetting{Enabled: true, BotUsername: "bot", BotToken: "123:abc", ClientSecret: "s", OIDCRedirectURI: "https://x/cb"}, settingssecurity.TelegramLoginModeOIDC},
		{"oidc-missing-redirect-falls-back-widget", settingssecurity.TelegramAuthSetting{Enabled: true, BotUsername: "bot", BotToken: "1:abc", ClientSecret: "s"}, settingssecurity.TelegramLoginModeWidget},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := settingssecurity.ResolveTelegramLoginMode(settingssecurity.NormalizeTelegramAuthSetting(tc.setting)); got != tc.want {
				t.Fatalf("mode = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTelegramAuthSettingMaskIncludesOIDC(t *testing.T) {
	masked := settingssecurity.MaskTelegramAuthSettingForAdmin(settingssecurity.NormalizeTelegramAuthSetting(settingssecurity.TelegramAuthSetting{
		Enabled: true, BotUsername: "bot", BotToken: "123:abc", ClientSecret: "secret", OIDCRedirectURI: "https://x/auth/telegram/callback",
	}))
	if masked["client_secret"] != "" {
		t.Fatalf("client_secret should be masked, got %v", masked["client_secret"])
	}
	if masked["has_client_secret"] != true {
		t.Fatalf("has_client_secret should be true")
	}
	if masked["oidc_redirect_uri"] != "https://x/auth/telegram/callback" {
		t.Fatalf("oidc_redirect_uri mismatch: %v", masked["oidc_redirect_uri"])
	}
	if masked["mode"] != string(settingssecurity.TelegramLoginModeOIDC) {
		t.Fatalf("mode = %v, want oidc", masked["mode"])
	}
}

func TestValidateTelegramAuthSettingRequiresRedirectWhenSecretSet(t *testing.T) {
	err := settingssecurity.ValidateTelegramAuthSetting(settingssecurity.TelegramAuthSetting{Enabled: true, BotUsername: "bot", BotToken: "123:abc", ClientSecret: "secret"})
	if err == nil {
		t.Fatalf("expected error when client_secret set without oidc_redirect_uri")
	}
}

func TestTelegramAuthSettingPatchClientSecretEmptyKeepsExisting(t *testing.T) {
	cur := settingssecurity.NormalizeTelegramAuthSetting(settingssecurity.TelegramAuthSetting{Enabled: true, BotUsername: "bot", BotToken: "123:abc", ClientSecret: "old", OIDCRedirectURI: "https://x/auth/telegram/callback"})
	empty := ""
	next := settingssecurity.ApplyTelegramAuthSettingPatch(cur, settingssecurity.TelegramAuthSettingPatch{ClientSecret: &empty})
	if next.ClientSecret != "old" {
		t.Fatalf("empty client_secret patch should keep existing, got %q", next.ClientSecret)
	}
	val := "new"
	next2 := settingssecurity.ApplyTelegramAuthSettingPatch(cur, settingssecurity.TelegramAuthSettingPatch{ClientSecret: &val})
	if next2.ClientSecret != "new" {
		t.Fatalf("client_secret should update to new, got %q", next2.ClientSecret)
	}
}

func TestTelegramAuthSettingPatchClearingRedirectAlsoClearsClientSecret(t *testing.T) {
	cur := settingssecurity.NormalizeTelegramAuthSetting(settingssecurity.TelegramAuthSetting{Enabled: true, BotUsername: "bot", BotToken: "123:abc", ClientSecret: "secret", OIDCRedirectURI: "https://x/auth/telegram/callback"})
	empty := ""
	next := settingssecurity.ApplyTelegramAuthSettingPatch(cur, settingssecurity.TelegramAuthSettingPatch{OIDCRedirectURI: &empty})
	if next.OIDCRedirectURI != "" || next.ClientSecret != "" {
		t.Fatalf("clearing oidc_redirect_uri should also clear client_secret, got redirect=%q secret=%q", next.OIDCRedirectURI, next.ClientSecret)
	}
	if err := settingssecurity.ValidateTelegramAuthSetting(next); err != nil {
		t.Fatalf("setting should be valid (widget mode) after clearing, got %v", err)
	}
	// 仅更新（非清空）回调地址时 client_secret 保留
	other := "https://y/auth/telegram/callback"
	next2 := settingssecurity.ApplyTelegramAuthSettingPatch(cur, settingssecurity.TelegramAuthSettingPatch{OIDCRedirectURI: &other})
	if next2.ClientSecret != "secret" {
		t.Fatalf("updating (not clearing) redirect should keep client_secret, got %q", next2.ClientSecret)
	}
}
