package settingshttp

import (
	"errors"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/cache"
	captcha "github.com/dujiao-next/internal/modules/captcha/contract"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// CaptchaAdminService 是后台验证码设置端口。
type CaptchaAdminService interface {
	GetCaptchaSetting() (settingssecurity.CaptchaSetting, error)
	PatchCaptchaSetting(patch settingssecurity.CaptchaSettingPatch) (settingssecurity.CaptchaSetting, error)
	ApplyRuntime(setting settingssecurity.CaptchaSetting)
}

// CaptchaHandler 处理后台验证码设置请求。
type CaptchaHandler struct {
	captcha CaptchaAdminService
}

func NewCaptchaHandler(captcha CaptchaAdminService) *CaptchaHandler {
	if captcha == nil {
		panic("settings captcha handler: captcha is nil")
	}
	return &CaptchaHandler{captcha: captcha}
}

// GetCaptcha 获取验证码配置（脱敏）。
func (h *CaptchaHandler) GetCaptcha(c *gin.Context) {
	setting, err := h.captcha.GetCaptchaSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, settingssecurity.MaskCaptchaSettingForAdmin(setting))
}

// UpdateCaptcha 更新验证码配置。
func (h *CaptchaHandler) UpdateCaptcha(c *gin.Context) {
	var req settingssecurity.CaptchaSettingPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	setting, err := h.captcha.PatchCaptchaSetting(req)
	if err != nil {
		switch {
		case errors.Is(err, captcha.ErrConfigInvalid):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		}
		return
	}

	h.captcha.ApplyRuntime(setting)
	_ = cache.DelAllPublicConfig(c.Request.Context())
	response.Success(c, settingssecurity.MaskCaptchaSettingForAdmin(setting))
}
