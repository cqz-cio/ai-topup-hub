package settingsbootstrap

import (
	"github.com/dujiao-next/internal/config"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

type googleAuthRuntime interface {
	SetConfig(config.GoogleAuthConfig)
}

type settingsGoogleAuthAdapter struct {
	settings   *settingsapp.Service
	cfg        *config.Config
	googleAuth googleAuthRuntime
}

func (a settingsGoogleAuthAdapter) GetGoogleAuthSetting() (settingssecurity.GoogleAuthSetting, error) {
	return a.settings.GetGoogleAuthSetting(a.cfg.GoogleAuth)
}

func (a settingsGoogleAuthAdapter) PatchGoogleAuthSetting(patch settingssecurity.GoogleAuthSettingPatch) (settingssecurity.GoogleAuthSetting, error) {
	return a.settings.PatchGoogleAuthSetting(a.cfg.GoogleAuth, patch)
}

func (a settingsGoogleAuthAdapter) ApplyRuntime(setting settingssecurity.GoogleAuthSetting) {
	runtimeCfg := settingssecurity.GoogleAuthSettingToConfig(setting)
	if a.googleAuth != nil {
		a.googleAuth.SetConfig(runtimeCfg)
	}
}
