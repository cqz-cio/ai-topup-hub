package settingshttp

import (
	"errors"

	"github.com/gin-gonic/gin"

	"github.com/dujiao-next/internal/cache"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
)

// GoogleAuthAdminService 是后台 Google Identity Services 登录设置端口。
type GoogleAuthAdminService interface {
	GetGoogleAuthSetting() (settingssecurity.GoogleAuthSetting, error)
	PatchGoogleAuthSetting(patch settingssecurity.GoogleAuthSettingPatch) (settingssecurity.GoogleAuthSetting, error)
	ApplyRuntime(setting settingssecurity.GoogleAuthSetting)
}

// GoogleAuthHandler 处理后台 Google Identity Services 登录设置请求。
type GoogleAuthHandler struct {
	googleAuth GoogleAuthAdminService
}

func NewGoogleAuthHandler(googleAuth GoogleAuthAdminService) *GoogleAuthHandler {
	if googleAuth == nil {
		panic("settings google auth handler: googleAuth is nil")
	}
	return &GoogleAuthHandler{googleAuth: googleAuth}
}

// GetGoogleAuth 获取 Google 登录配置。
func (h *GoogleAuthHandler) GetGoogleAuth(c *gin.Context) {
	setting, err := h.googleAuth.GetGoogleAuthSetting()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.settings_fetch_failed", err)
		return
	}
	response.Success(c, settingssecurity.MaskGoogleAuthSettingForAdmin(setting))
}

// UpdateGoogleAuth 更新 Google 登录配置并热更新登录服务。
func (h *GoogleAuthHandler) UpdateGoogleAuth(c *gin.Context) {
	var req settingssecurity.GoogleAuthSettingPatch
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	setting, err := h.googleAuth.PatchGoogleAuthSetting(req)
	if err != nil {
		switch {
		case errors.Is(err, settingssecurity.ErrGoogleAuthConfigInvalid):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, err.Error(), nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.settings_save_failed", err)
		}
		return
	}

	h.googleAuth.ApplyRuntime(setting)
	_ = cache.DelAllPublicConfig(c.Request.Context())
	response.Success(c, settingssecurity.MaskGoogleAuthSettingForAdmin(setting))
}
