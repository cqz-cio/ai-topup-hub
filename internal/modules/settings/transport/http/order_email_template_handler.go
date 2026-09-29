package settingshttp

import (
	"errors"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// OrderEmailTemplateAdminService 是后台订单邮件模板设置端口。
type OrderEmailTemplateAdminService interface {
	GetOrderEmailTemplateSetting() (settingsmessaging.OrderEmailTemplateSetting, error)
	PatchOrderEmailTemplateSetting(patch settingsmessaging.OrderEmailTemplateSettingPatch) (settingsmessaging.OrderEmailTemplateSetting, error)
	ResetOrderEmailTemplateSetting() (settingsmessaging.OrderEmailTemplateSetting, error)
}

// OrderEmailTemplateHandler 处理后台订单邮件模板设置请求。
type OrderEmailTemplateHandler struct {
	templates OrderEmailTemplateAdminService
}

func NewOrderEmailTemplateHandler(templates OrderEmailTemplateAdminService) *OrderEmailTemplateHandler {
	if templates == nil {
		panic("settings order-email-template handler: templates is nil")
	}
	return &OrderEmailTemplateHandler{templates: templates}
}

// GetOrderEmailTemplate 获取订单邮件模板配置。
func (h *OrderEmailTemplateHandler) GetOrderEmailTemplate(c *gin.Context) {
	setting, err := h.templates.GetOrderEmailTemplateSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, settingsmessaging.MaskOrderEmailTemplateSettingForAdmin(setting))
}

// UpdateOrderEmailTemplate 更新订单邮件模板配置。
func (h *OrderEmailTemplateHandler) UpdateOrderEmailTemplate(c *gin.Context) {
	var req settingsmessaging.OrderEmailTemplateSettingPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	setting, err := h.templates.PatchOrderEmailTemplateSetting(req)
	if err != nil {
		switch {
		case errors.Is(err, settingsmessaging.ErrOrderEmailTemplateConfigInvalid):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		}
		return
	}

	response.Success(c, settingsmessaging.MaskOrderEmailTemplateSettingForAdmin(setting))
}

// ResetOrderEmailTemplate 重置订单邮件模板为默认。
func (h *OrderEmailTemplateHandler) ResetOrderEmailTemplate(c *gin.Context) {
	setting, err := h.templates.ResetOrderEmailTemplateSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		return
	}
	response.Success(c, settingsmessaging.MaskOrderEmailTemplateSettingForAdmin(setting))
}
