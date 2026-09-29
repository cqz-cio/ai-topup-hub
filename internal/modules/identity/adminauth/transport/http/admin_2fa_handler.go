package adminauthhttp

import (
	"context"
	"errors"
	"time"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

const (
	challengeMaxFailures = 5
)

var (
	ErrNotFound            = errors.New("not found")
	ErrTOTPAlreadyEnabled  = errors.New("totp already enabled")
	ErrTOTPNotEnabled      = errors.New("totp not enabled")
	ErrTOTPPendingExpired  = errors.New("totp pending secret expired")
	ErrTOTPCodeInvalid     = errors.New("totp code invalid")
	ErrTOTPRecoveryInvalid = errors.New("recovery code invalid or used")
	ErrTOTPTooManyAttempts = errors.New("too many failed attempts")
	ErrTOTPCannotResetSelf = errors.New("cannot reset self via super admin endpoint")
)

// TOTPStatus 管理员 2FA 状态。
type TOTPStatus struct {
	Enabled                bool       `json:"enabled"`
	EnabledAt              *time.Time `json:"enabled_at,omitempty"`
	RecoveryCodesRemaining int        `json:"recovery_codes_remaining"`
	RecoveryCodesTotal     int        `json:"recovery_codes_total"`
}

// TOTPSetupResult /2fa/setup 响应。
type TOTPSetupResult struct {
	Secret     string    `json:"secret"`
	OtpauthURL string    `json:"otpauth_url"`
	ExpiresAt  time.Time `json:"expires_at"`
}

// TOTPEnableResult /2fa/enable 响应。
type TOTPEnableResult struct {
	EnabledAt     time.Time `json:"enabled_at"`
	RecoveryCodes []string  `json:"recovery_codes"`
}

// ChallengeClaims 是 transport 层挑战 token 视图。
type ChallengeClaims struct {
	AdminID uint
	JTI     string
}

// AuthLoginResult 是 transport 层管理员登录结果视图。
type AuthLoginResult struct {
	RequiresTOTP       bool
	Admin              *admindomain.Admin
	Token              string
	ExpiresAt          time.Time
	ChallengeToken     string
	ChallengeExpiresAt time.Time
}

// ChallengeStore 管理挑战失败计数与撤销。
type ChallengeStore interface {
	IsRevoked(ctx context.Context, jti string) bool
	BumpFails(ctx context.Context, jti string) int64
	Revoke(ctx context.Context, jti string)
}

// AdminLoginRecorder 记录管理员登录审计日志。
type AdminLoginRecorder interface {
	Record(adminID uint, username, eventType, status, failReason, clientIP, userAgent, requestID string, operatorID *uint)
}

// TOTPService 是管理员 2FA 绑定管理端口。
type TOTPService interface {
	GetStatus(adminID uint) (*TOTPStatus, error)
	Setup(adminID uint) (*TOTPSetupResult, error)
	Enable(adminID uint, code string) (*TOTPEnableResult, error)
	Disable(adminID uint, code string, isRecoveryCode bool) error
	RegenerateRecoveryCodes(adminID uint, code string) ([]string, error)
	VerifyChallengeCode(adminID uint, code string) error
	VerifyChallengeRecoveryCode(adminID uint, code string) error
	AdminReset(operatorID, targetID uint) error
}

// AuthService 是 2FA 登录完成端口。
type AuthService interface {
	ParseChallengeToken(tokenString string) (*ChallengeClaims, error)
	CompleteLoginAfter2FA(adminID uint) (*AuthLoginResult, error)
	GetAdminUsername(adminID uint) (string, error)
}

// Admin2FAHandler 处理管理员 2FA 管理与挑战验证 HTTP 请求。
type Admin2FAHandler struct {
	totp       TOTPService
	auth       AuthService
	challenges ChallengeStore
	recorder   AdminLoginRecorder
}

func NewAdmin2FAHandler(totp TOTPService, auth AuthService, challenges ChallengeStore, recorder AdminLoginRecorder) *Admin2FAHandler {
	if totp == nil {
		panic("admin 2fa handler: totp is nil")
	}
	if auth == nil {
		panic("admin 2fa handler: auth is nil")
	}
	return &Admin2FAHandler{totp: totp, auth: auth, challenges: challenges, recorder: recorder}
}

func (h *Admin2FAHandler) writeLoginLog(c *gin.Context, adminID uint, username, eventType, status, failReason string, operatorID *uint) {
	if h == nil || h.recorder == nil || c == nil {
		return
	}
	requestID := ""
	if rid, ok := c.Get("request_id"); ok {
		if value, ok := rid.(string); ok {
			requestID = value
		}
	}
	h.recorder.Record(adminID, username, eventType, status, failReason, c.ClientIP(), c.Request.UserAgent(), requestID, operatorID)
}

func adminUsername(c *gin.Context) string {
	return c.GetString("username")
}

// Get2FAStatus 当前管理员 2FA 状态。
func (h *Admin2FAHandler) Get2FAStatus(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	st, err := h.totp.GetStatus(adminID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		return
	}
	response.Success(c, st)
}

// Setup2FA 开始绑定，返回 secret + otpauth url。
func (h *Admin2FAHandler) Setup2FA(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	res, err := h.totp.Setup(adminID)
	if err != nil {
		switch {
		case errors.Is(err, ErrTOTPAlreadyEnabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_already_enabled", nil)
		case errors.Is(err, ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEvent2FASetup, constants.AdminLoginStatusSuccess, "", nil)
	response.Success(c, res)
}

// Enable2FARequest 启用请求。
type Enable2FARequest struct {
	Code string `json:"code" binding:"required"`
}

// Enable2FA 完成绑定。
func (h *Admin2FAHandler) Enable2FA(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var req Enable2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	res, err := h.totp.Enable(adminID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, ErrTOTPAlreadyEnabled):
			h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEvent2FAEnabled, constants.AdminLoginStatusFailed, constants.AdminLoginFailAlreadyEnabled, nil)
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_already_enabled", nil)
		case errors.Is(err, ErrTOTPPendingExpired):
			h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEvent2FAEnabled, constants.AdminLoginStatusFailed, constants.AdminLoginFailPendingExpired, nil)
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_pending_expired", nil)
		case errors.Is(err, ErrTOTPCodeInvalid):
			h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEvent2FAEnabled, constants.AdminLoginStatusFailed, constants.AdminLoginFailInvalidTOTPCode, nil)
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_invalid", nil)
		case errors.Is(err, ErrTOTPTooManyAttempts):
			h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEvent2FAEnabled, constants.AdminLoginStatusFailed, constants.AdminLoginFailTooManyAttempts, nil)
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_too_many_attempts", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEvent2FAEnabled, constants.AdminLoginStatusSuccess, "", nil)
	response.Success(c, res)
}

// Disable2FARequest 关闭请求。
type Disable2FARequest struct {
	Code         string `json:"code"`
	RecoveryCode string `json:"recovery_code"`
}

// Disable2FA 关闭 2FA。
func (h *Admin2FAHandler) Disable2FA(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var req Disable2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if req.Code == "" && req.RecoveryCode == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_required", nil)
		return
	}
	isRecovery := req.RecoveryCode != ""
	codeArg := req.Code
	if isRecovery {
		codeArg = req.RecoveryCode
	}
	if err := h.totp.Disable(adminID, codeArg, isRecovery); err != nil {
		switch {
		case errors.Is(err, ErrTOTPNotEnabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_not_enabled", nil)
		case errors.Is(err, ErrTOTPCodeInvalid):
			h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEvent2FADisabled, constants.AdminLoginStatusFailed, constants.AdminLoginFailInvalidTOTPCode, nil)
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_invalid", nil)
		case errors.Is(err, ErrTOTPRecoveryInvalid):
			h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEvent2FADisabled, constants.AdminLoginStatusFailed, constants.AdminLoginFailInvalidRecoveryCode, nil)
			ginutil.RespondError(c, response.CodeBadRequest, "error.recovery_code_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEvent2FADisabled, constants.AdminLoginStatusSuccess, "", nil)
	response.Success(c, nil)
}

// RegenerateRecoveryCodesRequest 重新生成恢复码请求。
type RegenerateRecoveryCodesRequest struct {
	Code string `json:"code" binding:"required"`
}

// RegenerateRecoveryCodes 重新生成恢复码。
func (h *Admin2FAHandler) RegenerateRecoveryCodes(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var req RegenerateRecoveryCodesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	codes, err := h.totp.RegenerateRecoveryCodes(adminID, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, ErrTOTPNotEnabled):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_not_enabled", nil)
		case errors.Is(err, ErrTOTPCodeInvalid):
			h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEventRecoveryRegenerated, constants.AdminLoginStatusFailed, constants.AdminLoginFailInvalidTOTPCode, nil)
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	h.writeLoginLog(c, adminID, adminUsername(c), constants.AdminLoginEventRecoveryRegenerated, constants.AdminLoginStatusSuccess, "", nil)
	response.Success(c, gin.H{"recovery_codes": codes})
}

// Verify2FARequest 第二步登录请求。
type Verify2FARequest struct {
	ChallengeToken string `json:"challenge_token" binding:"required"`
	Code           string `json:"code"`
	RecoveryCode   string `json:"recovery_code"`
}

// Verify2FA 完成两步登录。
func (h *Admin2FAHandler) Verify2FA(c *gin.Context) {
	var req Verify2FARequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if req.Code == "" && req.RecoveryCode == "" {
		ginutil.RespondError(c, response.CodeBadRequest, "error.totp_code_required", nil)
		return
	}
	claims, err := h.auth.ParseChallengeToken(req.ChallengeToken)
	if err != nil {
		ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_challenge_invalid", nil)
		return
	}
	ctx := context.Background()
	if h.challenges != nil && h.challenges.IsRevoked(ctx, claims.JTI) {
		ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_challenge_invalid", nil)
		return
	}
	verifyErr := h.verifyChallengeAttempt(claims.AdminID, req.Code, req.RecoveryCode)
	username, _ := h.auth.GetAdminUsername(claims.AdminID)
	if verifyErr != nil {
		failCnt := int64(0)
		if h.challenges != nil {
			failCnt = h.challenges.BumpFails(ctx, claims.JTI)
		}
		event := constants.AdminLoginEventLogin2FAVerify
		failReason := constants.AdminLoginFailInvalidTOTPCode
		if req.RecoveryCode != "" {
			event = constants.AdminLoginEventLoginRecoveryCode
			failReason = constants.AdminLoginFailInvalidRecoveryCode
		}
		h.writeLoginLog(c, claims.AdminID, username, event, constants.AdminLoginStatusFailed, failReason, nil)
		if failCnt >= challengeMaxFailures {
			if h.challenges != nil {
				h.challenges.Revoke(ctx, claims.JTI)
			}
			ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_too_many_attempts", nil)
			return
		}
		switch {
		case errors.Is(verifyErr, ErrTOTPCodeInvalid):
			ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_code_invalid", nil)
		case errors.Is(verifyErr, ErrTOTPRecoveryInvalid):
			ginutil.RespondError(c, response.CodeUnauthorized, "error.recovery_code_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeUnauthorized, "error.totp_challenge_invalid", nil)
		}
		return
	}
	if h.challenges != nil {
		h.challenges.Revoke(ctx, claims.JTI)
	}
	loginRes, err := h.auth.CompleteLoginAfter2FA(claims.AdminID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.login_failed", err)
		return
	}
	successEvent := constants.AdminLoginEventLogin2FAVerify
	if req.RecoveryCode != "" {
		successEvent = constants.AdminLoginEventLoginRecoveryCode
	}
	h.writeLoginLog(c, claims.AdminID, username, successEvent, constants.AdminLoginStatusSuccess, "", nil)
	response.Success(c, gin.H{
		"requires_totp": false,
		"token":         loginRes.Token,
		"user":          gin.H{"id": loginRes.Admin.ID, "username": loginRes.Admin.Username},
		"expires_at":    loginRes.ExpiresAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

func (h *Admin2FAHandler) verifyChallengeAttempt(adminID uint, code, recoveryCode string) error {
	if recoveryCode != "" {
		return h.totp.VerifyChallengeRecoveryCode(adminID, recoveryCode)
	}
	return h.totp.VerifyChallengeCode(adminID, code)
}

// ResetTargetAdmin2FA 超管重置某管理员 2FA。
func (h *Admin2FAHandler) ResetTargetAdmin2FA(c *gin.Context) {
	operatorID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	if !ginutil.IsSuperAdmin(c) {
		ginutil.RespondError(c, response.CodeForbidden, "error.forbidden", nil)
		return
	}
	targetID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.totp.AdminReset(operatorID, targetID); err != nil {
		switch {
		case errors.Is(err, ErrTOTPCannotResetSelf):
			ginutil.RespondError(c, response.CodeBadRequest, "error.totp_cannot_reset_self", nil)
		case errors.Is(err, ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal_error", err)
		}
		return
	}
	username, _ := h.auth.GetAdminUsername(targetID)
	op := operatorID
	h.writeLoginLog(c, targetID, username, constants.AdminLoginEvent2FAResetByAdmin, constants.AdminLoginStatusSuccess, "", &op)
	response.Success(c, nil)
}
