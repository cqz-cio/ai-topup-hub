package settingsbootstrap

import (
	"github.com/dujiao-next/internal/config"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

type settingsTelegramAuthAdapter struct {
	settings     *settingsapp.Service
	cfg          *config.Config
	telegramAuth *telegramauthapp.Service
}

func (a settingsTelegramAuthAdapter) GetTelegramAuthSetting() (settingssecurity.TelegramAuthSetting, error) {
	return a.settings.GetTelegramAuthSetting(a.cfg.TelegramAuth)
}

func (a settingsTelegramAuthAdapter) PatchTelegramAuthSetting(patch settingssecurity.TelegramAuthSettingPatch) (settingssecurity.TelegramAuthSetting, error) {
	return a.settings.PatchTelegramAuthSetting(a.cfg.TelegramAuth, patch)
}

func (a settingsTelegramAuthAdapter) ApplyRuntime(setting settingssecurity.TelegramAuthSetting) {
	a.cfg.TelegramAuth = settingssecurity.TelegramAuthSettingToConfig(setting)
	if a.telegramAuth != nil {
		a.telegramAuth.SetConfig(a.cfg.TelegramAuth)
	}
}
