package contract

import (
	"github.com/dujiao-next/internal/config"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

// SettingReader 是验证码运行时读取动态配置所需的最小端口。
type SettingReader interface {
	GetCaptchaSetting(defaultCfg config.CaptchaConfig) (settingssecurity.CaptchaSetting, error)
}

// TurnstileVerifier 是应用层校验 Turnstile 令牌所需的端口。
type TurnstileVerifier interface {
	Verify(cfg settingssecurity.CaptchaTurnstileSetting, token, clientIP string) error
}
