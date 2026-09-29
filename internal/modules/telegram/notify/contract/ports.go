package contract

import (
	"context"

	"github.com/dujiao-next/internal/config"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

// SettingReader 提供 Telegram 动态配置。
type SettingReader interface {
	GetTelegramAuthSetting(defaultCfg config.TelegramAuthConfig) (settingssecurity.TelegramAuthSetting, error)
}

// Sender 是应用层发送显式 Bot Token 消息所需的端口。
type Sender interface {
	SendWithBotToken(ctx context.Context, botToken string, options SendOptions) error
}
