package settingshttp

import (
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// TelegramBotAdminService 是后台 Telegram Bot 设置端口。
type TelegramBotAdminService interface {
	GetTelegramBotConfig() (settingsmessaging.TelegramBotConfigSetting, error)
	UpdateTelegramBotConfig(cfg settingsmessaging.TelegramBotConfigSetting) (settingsmessaging.TelegramBotConfigSetting, error)
	GetTelegramBotRuntimeStatus() (settingsmessaging.TelegramBotRuntimeStatusSetting, error)
}

// TelegramBotHandler 处理后台 Telegram Bot 设置请求。
type TelegramBotHandler struct {
	bot TelegramBotAdminService
}

func NewTelegramBotHandler(bot TelegramBotAdminService) *TelegramBotHandler {
	if bot == nil {
		panic("settings telegram-bot handler: bot is nil")
	}
	return &TelegramBotHandler{bot: bot}
}

// GetTelegramBotConfig 获取 Telegram Bot 配置。
func (h *TelegramBotHandler) GetTelegramBotConfig(c *gin.Context) {
	setting, err := h.bot.GetTelegramBotConfig()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, settingsmessaging.MaskTelegramBotConfigForAdmin(setting))
}

// UpdateTelegramBotConfig 更新 Telegram Bot 配置（整对象覆盖）。
func (h *TelegramBotHandler) UpdateTelegramBotConfig(c *gin.Context) {
	var req settingsmessaging.TelegramBotConfigSetting
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	setting, err := h.bot.UpdateTelegramBotConfig(req)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}

	response.Success(c, settingsmessaging.MaskTelegramBotConfigForAdmin(setting))
}

// GetTelegramBotRuntimeStatus 获取 Telegram Bot 运行时状态。
func (h *TelegramBotHandler) GetTelegramBotRuntimeStatus(c *gin.Context) {
	status, err := h.bot.GetTelegramBotRuntimeStatus()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, status)
}
