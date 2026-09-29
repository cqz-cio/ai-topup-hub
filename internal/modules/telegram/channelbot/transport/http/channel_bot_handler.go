package telegramhttp

import (
	"net/http"
	"time"

	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/platform/http/channelresponse"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// BotSettings 是渠道 Telegram Bot 配置/心跳端口。
type BotSettings interface {
	GetTelegramBotConfig() (settingsmessaging.TelegramBotConfigSetting, error)
	GetTelegramBotRuntimeStatus() (settingsmessaging.TelegramBotRuntimeStatusSetting, error)
	UpdateTelegramBotRuntimeStatus(status settingsmessaging.TelegramBotRuntimeStatusSetting) error
}

// ChannelBotTokenProvider 按渠道客户端 ID 解密 bot token。
type ChannelBotTokenProvider interface {
	DecryptBotTokenByClientID(clientID uint) (string, error)
}

// ChannelBotHandler 处理渠道 Telegram Bot 配置与心跳。
type ChannelBotHandler struct {
	settings BotSettings
	tokens   ChannelBotTokenProvider
}

func NewChannelBotHandler(settingsSvc BotSettings, tokens ChannelBotTokenProvider) *ChannelBotHandler {
	if settingsSvc == nil {
		panic("telegram channel bot handler: settings is nil")
	}
	if tokens == nil {
		panic("telegram channel bot handler: tokens is nil")
	}
	return &ChannelBotHandler{settings: settingsSvc, tokens: tokens}
}

type reportHeartbeatRequest struct {
	BotVersion       string   `json:"bot_version"`
	WebhookStatus    string   `json:"webhook_status"`
	MachineCode      string   `json:"machine_code"`
	LicenseStatus    string   `json:"license_status"`
	LicenseExpiresAt string   `json:"license_expires_at"`
	Warnings         []string `json:"warnings"`
}

// GetBotConfig GET /api/v1/channel/telegram/config
func (h *ChannelBotHandler) GetBotConfig(c *gin.Context) {
	config, err := h.settings.GetTelegramBotConfig()
	if err != nil {
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal_error", "error.internal_error", err)
		return
	}

	var botToken string
	if clientID, exists := c.Get("channel_client_id"); exists {
		if id, ok := clientID.(uint); ok {
			if token, tokenErr := h.tokens.DecryptBotTokenByClientID(id); tokenErr == nil {
				botToken = token
			}
		}
	}

	runtimeStatus, err := h.settings.GetTelegramBotRuntimeStatus()
	if err != nil {
		ginutil.RequestLog(c).Warnw("channel_get_runtime_status_failed", "error", err)
		runtimeStatus = settingsmessaging.DefaultTelegramBotRuntimeStatus()
	}

	channelresponse.Success(c, gin.H{
		"config":         settingsmessaging.SerializeTelegramBotConfigForChannel(config, botToken),
		"config_version": runtimeStatus.ConfigVersion,
	})
}

// ReportHeartbeat POST /api/v1/channel/telegram/heartbeat
func (h *ChannelBotHandler) ReportHeartbeat(c *gin.Context) {
	var req reportHeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		channelresponse.BindError(c, err)
		return
	}

	current, err := h.settings.GetTelegramBotRuntimeStatus()
	if err != nil {
		ginutil.RequestLog(c).Warnw("channel_heartbeat_get_status_failed", "error", err)
		current = settingsmessaging.DefaultTelegramBotRuntimeStatus()
	}

	now := time.Now().UTC().Format(time.RFC3339)
	updated := settingsmessaging.TelegramBotRuntimeStatusSetting{
		Connected:        true,
		LastSeenAt:       now,
		BotVersion:       req.BotVersion,
		WebhookStatus:    req.WebhookStatus,
		MachineCode:      req.MachineCode,
		LicenseStatus:    req.LicenseStatus,
		LicenseExpiresAt: req.LicenseExpiresAt,
		Warnings:         append([]string(nil), req.Warnings...),
		ConfigVersion:    current.ConfigVersion,
		LastConfigSyncAt: current.LastConfigSyncAt,
	}

	if err := h.settings.UpdateTelegramBotRuntimeStatus(updated); err != nil {
		ginutil.RequestLog(c).Errorw("channel_heartbeat_update_failed", "error", err)
		channelresponse.Error(c, http.StatusInternalServerError, response.CodeInternal, "internal_error", "error.internal_error", err)
		return
	}

	channelresponse.Success(c, gin.H{"config_version": updated.ConfigVersion})
}
