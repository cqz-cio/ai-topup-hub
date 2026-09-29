package presenter

import (
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	"github.com/dujiao-next/internal/shared/money"
)

// UserProfileResp 用户资料响应
type UserProfileResp struct {
	ID                 uint         `json:"id"`
	Email              string       `json:"email"`
	Nickname           string       `json:"nickname"`
	EmailVerifiedAt    *time.Time   `json:"email_verified_at"`
	Locale             string       `json:"locale"`
	MemberLevelID      uint         `json:"member_level_id"`
	TotalRecharged     money.Amount `json:"total_recharged"`
	TotalSpent         money.Amount `json:"total_spent"`
	EmailChangeMode    string       `json:"email_change_mode,omitempty"`
	PasswordChangeMode string       `json:"password_change_mode,omitempty"`
}

// NewUserProfileResp 从 userdomain.User 构造用户资料响应
func NewUserProfileResp(user *userdomain.User, emailMode, passwordMode string) UserProfileResp {
	if user == nil {
		return UserProfileResp{}
	}
	return UserProfileResp{
		ID:                 user.ID,
		Email:              user.Email,
		Nickname:           user.DisplayName,
		EmailVerifiedAt:    user.EmailVerifiedAt,
		Locale:             user.Locale,
		MemberLevelID:      user.MemberLevelID,
		TotalRecharged:     user.TotalRecharged,
		TotalSpent:         user.TotalSpent,
		EmailChangeMode:    emailMode,
		PasswordChangeMode: passwordMode,
	}
	// 排除：PasswordHash、PasswordSetupRequired、Status、TokenVersion、TokenInvalidBefore、
	// LastLoginAt、CreatedAt、UpdatedAt、DeletedAt
}

// TelegramBindingResp Telegram 绑定状态响应
type TelegramBindingResp struct {
	Bound          bool       `json:"bound"`
	Provider       string     `json:"provider,omitempty"`
	ProviderUserID string     `json:"provider_user_id,omitempty"`
	Username       string     `json:"username,omitempty"`
	AvatarURL      string     `json:"avatar_url,omitempty"`
	AuthAt         *time.Time `json:"auth_at,omitempty"`
	CanUnbind      bool       `json:"can_unbind"`
}

// GoogleBindingResp Google 绑定状态响应。
type GoogleBindingResp struct {
	Bound          bool       `json:"bound"`
	Provider       string     `json:"provider,omitempty"`
	ProviderUserID string     `json:"provider_user_id,omitempty"`
	Username       string     `json:"username,omitempty"`
	Email          string     `json:"email,omitempty"`
	DisplayName    string     `json:"display_name,omitempty"`
	AvatarURL      string     `json:"avatar_url,omitempty"`
	AuthAt         *time.Time `json:"auth_at,omitempty"`
	CanUnbind      bool       `json:"can_unbind"`
}

// NewGoogleBindingResp constructs the browser-safe Google binding response.
func NewGoogleBindingResp(identity *externalidentitydomain.Identity, email, displayName string, canUnbind bool) GoogleBindingResp {
	if identity == nil {
		return GoogleBindingResp{Bound: false, CanUnbind: false}
	}
	return GoogleBindingResp{
		Bound:          true,
		Provider:       identity.Provider,
		ProviderUserID: identity.ProviderUserID,
		Username:       identity.Username,
		Email:          email,
		DisplayName:    displayName,
		AvatarURL:      identity.AvatarURL,
		AuthAt:         identity.AuthAt,
		CanUnbind:      canUnbind,
	}
}

// NewTelegramBindingResp 从外部身份领域实体构造响应。
func NewTelegramBindingResp(identity *externalidentitydomain.Identity, canUnbind ...bool) TelegramBindingResp {
	resolvedCanUnbind := len(canUnbind) > 0 && canUnbind[0]
	if identity == nil {
		return TelegramBindingResp{Bound: false, CanUnbind: false}
	}
	return TelegramBindingResp{
		Bound:          true,
		Provider:       identity.Provider,
		ProviderUserID: identity.ProviderUserID,
		Username:       identity.Username,
		AvatarURL:      identity.AvatarURL,
		AuthAt:         identity.AuthAt,
		CanUnbind:      resolvedCanUnbind,
	}
	// 排除：ID、UserID、CreatedAt、UpdatedAt
}

// UserAuthBriefResp 登录/注册返回的精简用户信息
type UserAuthBriefResp struct {
	ID              uint       `json:"id"`
	Email           string     `json:"email"`
	Nickname        string     `json:"nickname"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
}

// NewUserAuthBriefResp 从 userdomain.User 构造登录/注册精简响应
func NewUserAuthBriefResp(user *userdomain.User) UserAuthBriefResp {
	return UserAuthBriefResp{
		ID:              user.ID,
		Email:           user.Email,
		Nickname:        user.DisplayName,
		EmailVerifiedAt: user.EmailVerifiedAt,
	}
}
