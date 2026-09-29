package application

import (
	"context"
	"strings"

	"github.com/dujiao-next/internal/config"
	notifycontract "github.com/dujiao-next/internal/modules/telegram/notify/contract"
)

// Service 解析通知配置并编排 Telegram 消息发送。
type Service struct {
	settings   notifycontract.SettingReader
	defaultCfg config.TelegramAuthConfig
	sender     notifycontract.Sender
}

// NewService 创建 Telegram 通知应用服务。
func NewService(settings notifycontract.SettingReader, defaultCfg config.TelegramAuthConfig, sender notifycontract.Sender) *Service {
	if sender == nil {
		panic("telegram notify service: sender is nil")
	}
	return &Service{settings: settings, defaultCfg: defaultCfg, sender: sender}
}

// SendMessage 使用当前配置的 Bot Token 发送消息。
func (s *Service) SendMessage(ctx context.Context, chatID, message string) error {
	token, err := s.resolveBotToken()
	if err != nil {
		return err
	}
	if token == "" {
		return notifycontract.ErrNotifyConfigInvalid
	}
	return s.sender.SendWithBotToken(ctx, token, notifycontract.SendOptions{
		ChatID:                chatID,
		Message:               message,
		DisableWebPagePreview: true,
	})
}

// SendWithBotToken 使用显式 Bot Token 发送消息。
func (s *Service) SendWithBotToken(ctx context.Context, botToken string, options notifycontract.SendOptions) error {
	return s.sender.SendWithBotToken(ctx, botToken, options)
}

func (s *Service) resolveBotToken() (string, error) {
	if s.settings == nil {
		return strings.TrimSpace(s.defaultCfg.BotToken), nil
	}
	setting, err := s.settings.GetTelegramAuthSetting(s.defaultCfg)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(setting.BotToken), nil
}
