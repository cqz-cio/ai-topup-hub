package application

import (
	"context"
	"errors"
	"testing"

	"github.com/dujiao-next/internal/config"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	notifycontract "github.com/dujiao-next/internal/modules/telegram/notify/contract"
)

type settingReaderStub struct {
	setting settingssecurity.TelegramAuthSetting
	err     error
	seen    config.TelegramAuthConfig
}

func (s *settingReaderStub) GetTelegramAuthSetting(defaultCfg config.TelegramAuthConfig) (settingssecurity.TelegramAuthSetting, error) {
	s.seen = defaultCfg
	return s.setting, s.err
}

type senderStub struct {
	token   string
	options notifycontract.SendOptions
	err     error
	calls   int
}

func (s *senderStub) SendWithBotToken(_ context.Context, token string, options notifycontract.SendOptions) error {
	s.calls++
	s.token = token
	s.options = options
	return s.err
}

func TestServiceSendMessageUsesDynamicBotToken(t *testing.T) {
	defaults := config.TelegramAuthConfig{BotToken: "default-token"}
	settings := &settingReaderStub{
		setting: settingssecurity.TelegramAuthSetting{BotToken: "  dynamic-token  "},
	}
	sender := &senderStub{}
	service := NewService(settings, defaults, sender)

	if err := service.SendMessage(context.Background(), "10001", "hello"); err != nil {
		t.Fatalf("send message failed: %v", err)
	}
	if settings.seen != defaults {
		t.Fatalf("unexpected default config: %#v", settings.seen)
	}
	if sender.calls != 1 || sender.token != "dynamic-token" {
		t.Fatalf("unexpected sender call: calls=%d token=%q", sender.calls, sender.token)
	}
	if sender.options.ChatID != "10001" || sender.options.Message != "hello" {
		t.Fatalf("unexpected send options: %#v", sender.options)
	}
	if !sender.options.DisableWebPagePreview {
		t.Fatal("configured notification must disable web page previews")
	}
}

func TestServiceSendMessageFallsBackToDefaultWithoutSettings(t *testing.T) {
	sender := &senderStub{}
	service := NewService(nil, config.TelegramAuthConfig{BotToken: "  default-token  "}, sender)

	if err := service.SendMessage(context.Background(), "10001", "hello"); err != nil {
		t.Fatalf("send message failed: %v", err)
	}
	if sender.calls != 1 || sender.token != "default-token" {
		t.Fatalf("unexpected sender call: calls=%d token=%q", sender.calls, sender.token)
	}
}

func TestServiceSendMessageRejectsMissingBotToken(t *testing.T) {
	sender := &senderStub{}
	service := NewService(&settingReaderStub{}, config.TelegramAuthConfig{}, sender)

	err := service.SendMessage(context.Background(), "10001", "hello")
	if !errors.Is(err, notifycontract.ErrNotifyConfigInvalid) {
		t.Fatalf("expected config invalid error, got %v", err)
	}
	if sender.calls != 0 {
		t.Fatalf("sender must not be called without a bot token, got %d calls", sender.calls)
	}
}

func TestServiceSendMessagePropagatesSettingReadFailure(t *testing.T) {
	wantErr := errors.New("settings unavailable")
	sender := &senderStub{}
	service := NewService(&settingReaderStub{err: wantErr}, config.TelegramAuthConfig{BotToken: "default-token"}, sender)

	err := service.SendMessage(context.Background(), "10001", "hello")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected setting error, got %v", err)
	}
	if sender.calls != 0 {
		t.Fatalf("sender must not be called after a setting error, got %d calls", sender.calls)
	}
}
