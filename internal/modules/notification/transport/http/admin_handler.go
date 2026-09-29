package notificationhttp

import (
	"context"
	"errors"
	"strings"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/contract"
	"github.com/dujiao-next/internal/modules/notification/domain"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type SettingsService interface {
	GetNotificationCenterSetting() (settingsmessaging.NotificationCenterSetting, error)
	PatchNotificationCenterSetting(settingsmessaging.NotificationCenterSettingPatch) (settingsmessaging.NotificationCenterSetting, error)
}

type LogService interface {
	ListForAdmin(contract.LogListFilter) ([]domain.NotificationLog, int64, error)
}

type Sender interface {
	SendTest(context.Context, contract.TestSendInput) error
}

type AdminHandler struct {
	settings SettingsService
	logs     LogService
	sender   Sender
}

func NewAdminHandler(settings SettingsService, logs LogService, sender Sender) *AdminHandler {
	if settings == nil || logs == nil || sender == nil {
		panic("notification admin handler: required dependency is nil")
	}
	return &AdminHandler{settings: settings, logs: logs, sender: sender}
}

// GetNotificationCenterSettings 获取通知中心配置
func (h *AdminHandler) GetNotificationCenterSettings(c *gin.Context) {
	setting, err := h.settings.GetNotificationCenterSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, settingsmessaging.MaskNotificationCenterSettingForAdmin(setting))
}

// UpdateNotificationCenterSettings 更新通知中心配置
func (h *AdminHandler) UpdateNotificationCenterSettings(c *gin.Context) {
	var req settingsmessaging.NotificationCenterSettingPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	setting, err := h.settings.PatchNotificationCenterSetting(req)
	if err != nil {
		switch {
		case errors.Is(err, contract.ErrConfigInvalid):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		}
		return
	}
	response.Success(c, settingsmessaging.MaskNotificationCenterSettingForAdmin(setting))
}

// NotificationCenterTestSendRequest 通知中心测试发送请求
type NotificationCenterTestSendRequest struct {
	Channel   string                 `json:"channel" binding:"required"`
	Target    string                 `json:"target" binding:"required"`
	Scene     string                 `json:"scene"`
	Locale    string                 `json:"locale"`
	Variables map[string]interface{} `json:"variables"`
}

// ListNotificationLogs 获取通知发送日志列表
func (h *AdminHandler) ListNotificationLogs(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	channel := strings.ToLower(strings.TrimSpace(c.Query("channel")))
	status := strings.ToLower(strings.TrimSpace(c.Query("status")))
	eventType := strings.ToLower(strings.TrimSpace(c.Query("event_type")))

	isTest, err := ginutil.ParseQueryBoolPtr(c, "is_test")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	items, total, err := h.logs.ListForAdmin(contract.LogListFilter{
		Page:        page,
		PageSize:    pageSize,
		Channel:     channel,
		Status:      status,
		EventType:   eventType,
		IsTest:      isTest,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}

	response.SuccessWithPage(c, items, response.BuildPagination(page, pageSize, total))
}

// TestNotificationCenterSettings 通知中心测试发送
func (h *AdminHandler) TestNotificationCenterSettings(c *gin.Context) {
	var req NotificationCenterTestSendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	channel := strings.ToLower(strings.TrimSpace(req.Channel))
	if channel != constants.NotificationChannelEmail &&
		channel != constants.NotificationChannelTelegram &&
		channel != constants.NotificationChannelFeishu {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	err := h.sender.SendTest(c.Request.Context(), contract.TestSendInput{
		Channel:   channel,
		Target:    strings.TrimSpace(req.Target),
		Scene:     strings.TrimSpace(req.Scene),
		Locale:    strings.TrimSpace(req.Locale),
		Variables: req.Variables,
	})
	if err != nil {
		switch {
		case errors.Is(err, contract.ErrConfigInvalid):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.notification_send_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"sent": true})
}
