package constants

// AdminLoginEventType admin 登录与 2FA 相关事件类型
const (
	AdminLoginEventLoginPassword       = "login_password"        // 第一步密码登录尝试
	AdminLoginEventLogin2FAVerify      = "login_2fa_verify"      // 第二步 TOTP 验证尝试
	AdminLoginEventLoginRecoveryCode   = "login_recovery_code"   // 用恢复码完成登录
	AdminLoginEvent2FASetup            = "2fa_setup"             // 调用 setup（生成 pending secret）
	AdminLoginEvent2FAEnabled          = "2fa_enabled"           // 完成绑定
	AdminLoginEvent2FADisabled         = "2fa_disabled"          // 自助关闭
	AdminLoginEventRecoveryRegenerated = "recovery_regenerated"  // 重新生成恢复码
	AdminLoginEvent2FAResetByAdmin     = "2fa_reset_by_admin"    // 超管重置他人
	AdminLoginEventPasswordResetByCLI  = "password_reset_by_cli" // 运维通过 admin-tool 重置密码
)

// AdminLoginStatus 登录结果
const (
	AdminLoginStatusSuccess = "success"
	AdminLoginStatusFailed  = "failed"
)

// AdminLoginFailReason 失败原因
const (
	AdminLoginFailInvalidCredentials  = "invalid_credentials"
	AdminLoginFailInvalidTOTPCode     = "invalid_totp_code"
	AdminLoginFailInvalidRecoveryCode = "invalid_recovery_code"
	AdminLoginFailChallengeExpired    = "challenge_expired"
	AdminLoginFailChallengeRevoked    = "challenge_revoked"
	AdminLoginFailTooManyAttempts     = "too_many_attempts"
	AdminLoginFailPendingExpired      = "pending_expired"
	AdminLoginFailCaptchaRequired     = "captcha_required"
	AdminLoginFailCaptchaInvalid      = "captcha_invalid"
	AdminLoginFailRateLimited         = "rate_limited"
	AdminLoginFailInternal            = "internal_error"
	AdminLoginFailAlreadyEnabled      = "already_enabled"
)
