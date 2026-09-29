package settingssecurity

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/dujiao-next/internal/config"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

var ErrTelegramAuthConfigInvalid = errors.New("telegram auth config invalid")

// TelegramLoginMode Telegram 网页登录模式。
type TelegramLoginMode string

const (
	TelegramLoginModeDisabled TelegramLoginMode = ""
	TelegramLoginModeWidget   TelegramLoginMode = "widget"
	TelegramLoginModeOIDC     TelegramLoginMode = "oidc"
)

// TelegramAuthSetting Telegram 登录配置实体。
type TelegramAuthSetting struct {
	Enabled            bool   `json:"enabled"`
	BotUsername        string `json:"bot_username"`
	BotToken           string `json:"bot_token"`
	ClientSecret       string `json:"client_secret"`
	OIDCRedirectURI    string `json:"oidc_redirect_uri"`
	MiniAppURL         string `json:"mini_app_url"`
	LoginExpireSeconds int    `json:"login_expire_seconds"`
	ReplayTTLSeconds   int    `json:"replay_ttl_seconds"`
}

// TelegramAuthSettingPatch Telegram 登录配置补丁。
type TelegramAuthSettingPatch struct {
	Enabled            *bool   `json:"enabled"`
	BotUsername        *string `json:"bot_username"`
	BotToken           *string `json:"bot_token"`
	ClientSecret       *string `json:"client_secret"`
	OIDCRedirectURI    *string `json:"oidc_redirect_uri"`
	MiniAppURL         *string `json:"mini_app_url"`
	LoginExpireSeconds *int    `json:"login_expire_seconds"`
	ReplayTTLSeconds   *int    `json:"replay_ttl_seconds"`
}

// DefaultTelegramAuthSetting 根据运行时配置生成默认设置。
func DefaultTelegramAuthSetting(cfg config.TelegramAuthConfig) TelegramAuthSetting {
	return NormalizeTelegramAuthSetting(TelegramAuthSetting{
		Enabled:            cfg.Enabled,
		BotUsername:        strings.TrimSpace(cfg.BotUsername),
		BotToken:           strings.TrimSpace(cfg.BotToken),
		ClientSecret:       strings.TrimSpace(cfg.ClientSecret),
		OIDCRedirectURI:    strings.TrimSpace(cfg.OIDCRedirectURI),
		MiniAppURL:         strings.TrimSpace(cfg.MiniAppURL),
		LoginExpireSeconds: cfg.LoginExpireSeconds,
		ReplayTTLSeconds:   cfg.ReplayTTLSeconds,
	})
}

// NormalizeTelegramAuthSetting 归一化 Telegram 配置。
func NormalizeTelegramAuthSetting(setting TelegramAuthSetting) TelegramAuthSetting {
	setting.BotUsername = strings.TrimPrefix(strings.TrimSpace(setting.BotUsername), "@")
	setting.BotToken = strings.TrimSpace(setting.BotToken)
	setting.ClientSecret = strings.TrimSpace(setting.ClientSecret)
	setting.OIDCRedirectURI = strings.TrimRight(strings.TrimSpace(setting.OIDCRedirectURI), "/")
	setting.MiniAppURL = strings.TrimSpace(setting.MiniAppURL)

	if setting.LoginExpireSeconds <= 0 {
		setting.LoginExpireSeconds = 300
	}
	if setting.LoginExpireSeconds < 30 {
		setting.LoginExpireSeconds = 30
	}
	if setting.LoginExpireSeconds > 86400 {
		setting.LoginExpireSeconds = 86400
	}

	if setting.ReplayTTLSeconds <= 0 {
		setting.ReplayTTLSeconds = setting.LoginExpireSeconds
	}
	if setting.ReplayTTLSeconds < 60 {
		setting.ReplayTTLSeconds = 60
	}
	if setting.ReplayTTLSeconds > 86400 {
		setting.ReplayTTLSeconds = 86400
	}
	return setting
}

// ValidateTelegramAuthSetting 校验 Telegram 配置合法性。
func ValidateTelegramAuthSetting(setting TelegramAuthSetting) error {
	normalized := NormalizeTelegramAuthSetting(setting)

	if normalized.LoginExpireSeconds < 30 || normalized.LoginExpireSeconds > 86400 {
		return fmt.Errorf("%w: 登录有效期需在 30-86400 秒之间", ErrTelegramAuthConfigInvalid)
	}
	if normalized.ReplayTTLSeconds < 60 || normalized.ReplayTTLSeconds > 86400 {
		return fmt.Errorf("%w: 重放保护时长需在 60-86400 秒之间", ErrTelegramAuthConfigInvalid)
	}
	if !normalized.Enabled {
		return nil
	}
	if normalized.BotUsername == "" {
		return fmt.Errorf("%w: Bot 用户名不能为空", ErrTelegramAuthConfigInvalid)
	}
	if strings.ContainsAny(normalized.BotUsername, " \t\r\n") {
		return fmt.Errorf("%w: Bot 用户名格式无效", ErrTelegramAuthConfigInvalid)
	}
	if normalized.BotToken == "" {
		return fmt.Errorf("%w: Bot Token 不能为空", ErrTelegramAuthConfigInvalid)
	}
	if normalized.ClientSecret != "" {
		if normalized.OIDCRedirectURI == "" {
			return fmt.Errorf("%w: 配置 client_secret 时必须同时填写 OIDC 回调地址", ErrTelegramAuthConfigInvalid)
		}
		if u, perr := url.Parse(normalized.OIDCRedirectURI); perr != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: OIDC 回调地址必须是合法的 http(s) URL", ErrTelegramAuthConfigInvalid)
		}
		if TelegramBotIDFromToken(normalized.BotToken) == "" {
			return fmt.Errorf("%w: Bot Token 格式无效，无法解析出 OIDC client_id", ErrTelegramAuthConfigInvalid)
		}
	}
	return nil
}

// ResolveTelegramLoginMode 判定网页登录模式（入参需先 Normalize）。
func ResolveTelegramLoginMode(s TelegramAuthSetting) TelegramLoginMode {
	if !s.Enabled {
		return TelegramLoginModeDisabled
	}
	if s.ClientSecret != "" && s.OIDCRedirectURI != "" && TelegramBotIDFromToken(s.BotToken) != "" {
		return TelegramLoginModeOIDC
	}
	if s.BotToken != "" {
		return TelegramLoginModeWidget
	}
	return TelegramLoginModeDisabled
}

// TelegramBotIDFromToken 从 "123456789:ABC..." 取数字前缀作为 OIDC client_id。
func TelegramBotIDFromToken(token string) string {
	token = strings.TrimSpace(token)
	idx := strings.IndexByte(token, ':')
	if idx <= 0 {
		return ""
	}
	id := token[:idx]
	for _, r := range id {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return id
}

// ApplyTelegramAuthSettingPatch 把补丁应用到当前配置（不做校验）。
func ApplyTelegramAuthSettingPatch(current TelegramAuthSetting, patch TelegramAuthSettingPatch) TelegramAuthSetting {
	next := current
	if patch.Enabled != nil {
		next.Enabled = *patch.Enabled
	}
	if patch.BotUsername != nil {
		next.BotUsername = strings.TrimSpace(*patch.BotUsername)
	}
	if patch.BotToken != nil {
		if v := strings.TrimSpace(*patch.BotToken); v != "" {
			next.BotToken = v
		}
	}
	if patch.ClientSecret != nil {
		if v := strings.TrimSpace(*patch.ClientSecret); v != "" {
			next.ClientSecret = v
		}
	}
	if patch.OIDCRedirectURI != nil {
		next.OIDCRedirectURI = strings.TrimSpace(*patch.OIDCRedirectURI)
		// 显式清空回调地址 = 退回旧版 widget 模式，连带清掉 client_secret，
		// 否则 ValidateTelegramAuthSetting 会因「设了 client_secret 却没有回调地址」报错且无从修复。
		if next.OIDCRedirectURI == "" {
			next.ClientSecret = ""
		}
	}
	if patch.MiniAppURL != nil {
		next.MiniAppURL = strings.TrimSpace(*patch.MiniAppURL)
	}
	if patch.LoginExpireSeconds != nil {
		next.LoginExpireSeconds = *patch.LoginExpireSeconds
	}
	if patch.ReplayTTLSeconds != nil {
		next.ReplayTTLSeconds = *patch.ReplayTTLSeconds
	}
	return next
}

// TelegramAuthSettingToConfig 转换为运行时配置。
func TelegramAuthSettingToConfig(setting TelegramAuthSetting) config.TelegramAuthConfig {
	normalized := NormalizeTelegramAuthSetting(setting)
	return config.TelegramAuthConfig{
		Enabled:            normalized.Enabled,
		BotUsername:        normalized.BotUsername,
		BotToken:           normalized.BotToken,
		ClientSecret:       normalized.ClientSecret,
		OIDCRedirectURI:    normalized.OIDCRedirectURI,
		MiniAppURL:         normalized.MiniAppURL,
		LoginExpireSeconds: normalized.LoginExpireSeconds,
		ReplayTTLSeconds:   normalized.ReplayTTLSeconds,
	}
}

// EncodeTelegramAuthSetting 转换为 settings 存储结构。
func EncodeTelegramAuthSetting(setting TelegramAuthSetting) jsonmap.JSON {
	normalized := NormalizeTelegramAuthSetting(setting)
	return jsonmap.JSON{
		"enabled":              normalized.Enabled,
		"bot_username":         normalized.BotUsername,
		"bot_token":            normalized.BotToken,
		"client_secret":        normalized.ClientSecret,
		"oidc_redirect_uri":    normalized.OIDCRedirectURI,
		"mini_app_url":         normalized.MiniAppURL,
		"login_expire_seconds": normalized.LoginExpireSeconds,
		"replay_ttl_seconds":   normalized.ReplayTTLSeconds,
	}
}

// MaskTelegramAuthSettingForAdmin 返回脱敏配置。
func MaskTelegramAuthSettingForAdmin(setting TelegramAuthSetting) jsonmap.JSON {
	normalized := NormalizeTelegramAuthSetting(setting)
	return jsonmap.JSON{
		"enabled":              normalized.Enabled,
		"bot_username":         normalized.BotUsername,
		"bot_token":            "",
		"has_bot_token":        normalized.BotToken != "",
		"client_secret":        "",
		"has_client_secret":    normalized.ClientSecret != "",
		"oidc_redirect_uri":    normalized.OIDCRedirectURI,
		"mode":                 string(ResolveTelegramLoginMode(normalized)),
		"mini_app_url":         normalized.MiniAppURL,
		"login_expire_seconds": normalized.LoginExpireSeconds,
		"replay_ttl_seconds":   normalized.ReplayTTLSeconds,
	}
}

// DecodeTelegramAuthSetting 从持久化 JSON 解码，并对缺失字段使用 fallback。
func DecodeTelegramAuthSetting(raw jsonmap.JSON, fallback TelegramAuthSetting) TelegramAuthSetting {
	next := fallback
	if raw == nil {
		return next
	}

	if value, exists := raw["enabled"]; exists {
		next.Enabled = settingsvalue.ParseBool(value)
	}
	if value, exists := raw["bot_username"]; exists {
		if text, ok := value.(string); ok {
			next.BotUsername = text
		}
	}
	if value, exists := raw["bot_token"]; exists {
		if text, ok := value.(string); ok {
			next.BotToken = text
		}
	}
	if value, exists := raw["client_secret"]; exists {
		if text, ok := value.(string); ok {
			next.ClientSecret = text
		}
	}
	if value, exists := raw["oidc_redirect_uri"]; exists {
		if text, ok := value.(string); ok {
			next.OIDCRedirectURI = text
		}
	}
	if value, exists := raw["mini_app_url"]; exists {
		if text, ok := value.(string); ok {
			next.MiniAppURL = text
		}
	}
	if value, exists := raw["login_expire_seconds"]; exists {
		if parsed, err := settingsvalue.ParseInt(value); err == nil {
			next.LoginExpireSeconds = parsed
		}
	}
	if value, exists := raw["replay_ttl_seconds"]; exists {
		if parsed, err := settingsvalue.ParseInt(value); err == nil {
			next.ReplayTTLSeconds = parsed
		}
	}
	return next
}

// NormalizeTelegramAuthSettingJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeTelegramAuthSettingJSON(raw jsonmap.JSON) jsonmap.JSON {
	return EncodeTelegramAuthSetting(DecodeTelegramAuthSetting(raw, DefaultTelegramAuthSetting(config.TelegramAuthConfig{})))
}
