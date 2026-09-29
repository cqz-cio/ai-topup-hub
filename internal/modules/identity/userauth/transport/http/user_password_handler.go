package userauthhttp

import (
	"errors"

	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

var (
	ErrWeakPassword    = errors.New("weak password")
	ErrInvalidPassword = errors.New("invalid password")
)

// WeakPasswordError 携带可本地化的弱密码策略详情。
type WeakPasswordError struct {
	key  string
	args []interface{}
}

func NewWeakPasswordError(key string, args ...interface{}) error {
	if key == "" {
		return ErrWeakPassword
	}
	return WeakPasswordError{key: key, args: args}
}

func (e WeakPasswordError) Error() string { return e.key }

func (e WeakPasswordError) Is(target error) bool { return target == ErrWeakPassword }

func (e WeakPasswordError) Key() string { return e.key }

func (e WeakPasswordError) Args() []interface{} { return e.args }

// UserPasswordService 是密码重置/改密端点所需的最小端口。
type UserPasswordService interface {
	GetEmailVerificationEnabled(defaultValue bool) (bool, error)
	ResetPassword(email, code, newPassword string) error
	ChangePassword(userID uint, oldPassword, newPassword string) error
}

// UserPasswordHandler 处理忘记密码与登录态改密 HTTP 请求。
type UserPasswordHandler struct {
	service UserPasswordService
}

func NewUserPasswordHandler(service UserPasswordService) *UserPasswordHandler {
	if service == nil {
		panic("user password handler: service is nil")
	}
	return &UserPasswordHandler{service: service}
}

// UserResetPasswordRequest 重置密码请求。
type UserResetPasswordRequest struct {
	Email       string `json:"email" binding:"required"`
	Code        string `json:"code" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// UserForgotPassword 忘记密码重置。
func (h *UserPasswordHandler) UserForgotPassword(c *gin.Context) {
	emailVerificationEnabled, err := h.service.GetEmailVerificationEnabled(true)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.reset_failed", err)
		return
	}
	if !emailVerificationEnabled {
		ginutil.RespondError(c, response.CodeForbidden, "error.password_reset_disabled", nil)
		return
	}

	var req UserResetPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.service.ResetPassword(req.Email, req.Code, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, ErrInvalidEmail):
			ginutil.RespondError(c, response.CodeBadRequest, "error.email_invalid", nil)
		case errors.Is(err, ErrUserNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		case errors.Is(err, ErrVerifyCodeInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_code_invalid", nil)
		case errors.Is(err, ErrVerifyCodeExpired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_code_expired", nil)
		case errors.Is(err, ErrVerifyCodeAttemptsExceeded):
			ginutil.RespondError(c, response.CodeBadRequest, "error.verify_code_attempts_exceeded", nil)
		case errors.Is(err, ErrWeakPassword):
			respondWeakPassword(c, err)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.reset_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"reset": true})
}

// ChangeUserPasswordRequest 用户改密请求。
type ChangeUserPasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangeUserPassword 用户登录态修改密码。
func (h *UserPasswordHandler) ChangeUserPassword(c *gin.Context) {
	id, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req ChangeUserPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.service.ChangePassword(id, req.OldPassword, req.NewPassword); err != nil {
		switch {
		case errors.Is(err, ErrInvalidPassword):
			ginutil.RespondError(c, response.CodeBadRequest, "error.password_old_invalid", nil)
		case errors.Is(err, ErrWeakPassword):
			respondWeakPassword(c, err)
		case errors.Is(err, ErrUserNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.save_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"updated": true})
}

func respondWeakPassword(c *gin.Context, err error) {
	locale := i18n.ResolveLocale(c)
	if perr, ok := err.(interface {
		Key() string
		Args() []interface{}
	}); ok {
		msg := i18n.Sprintf(locale, perr.Key(), perr.Args()...)
		ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, msg, nil)
		return
	}
	ginutil.RespondError(c, response.CodeBadRequest, "error.password_weak", nil)
}
