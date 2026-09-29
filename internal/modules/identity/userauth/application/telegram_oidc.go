package application

import (
	"context"

	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
)

// StartTelegramOIDCInput 启动 Telegram OIDC 流程输入
type StartTelegramOIDCInput struct {
	Intent  string // "login" | "bind"
	UserID  uint
	Context context.Context
}

// LoginWithTelegramOIDCInput Telegram OIDC 登录输入
type LoginWithTelegramOIDCInput struct {
	Code    string
	State   string
	Context context.Context
}

// BindTelegramOIDCInput Telegram OIDC 绑定输入
type BindTelegramOIDCInput struct {
	UserID  uint
	Code    string
	State   string
	Context context.Context
}

// StartTelegramOIDC 生成 Telegram OIDC 授权 URL
func (s *Service) StartTelegramOIDC(input StartTelegramOIDCInput) (string, error) {
	if s.telegramAuthService == nil {
		return "", telegramauthapp.ErrTelegramAuthConfigInvalid
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	intent := input.Intent
	if intent != telegramauthapp.IntentLogin && intent != telegramauthapp.IntentBind {
		return "", telegramauthapp.ErrTelegramAuthPayloadInvalid
	}
	return s.telegramAuthService.StartOIDCLogin(ctx, intent, input.UserID)
}

// LoginWithTelegramOIDC 通过 Telegram OIDC 回调登录
func (s *Service) LoginWithTelegramOIDC(input LoginWithTelegramOIDCInput) (*UserLoginResult, error) {
	if s.telegramAuthService == nil || s.userOAuthIdentityRepo == nil {
		return nil, telegramauthapp.ErrTelegramAuthConfigInvalid
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	verified, intent, _, err := s.telegramAuthService.CompleteOIDCLogin(ctx, input.Code, input.State)
	if err != nil {
		return nil, err
	}
	if intent != telegramauthapp.IntentLogin {
		return nil, telegramauthapp.ErrTelegramAuthPayloadInvalid
	}
	return s.LoginVerifiedTelegram(verified)
}

// BindTelegramOIDC 通过 Telegram OIDC 回调绑定当前用户
func (s *Service) BindTelegramOIDC(input BindTelegramOIDCInput) (*externalidentitydomain.Identity, error) {
	if input.UserID == 0 {
		return nil, ErrNotFound
	}
	if s.telegramAuthService == nil || s.userOAuthIdentityRepo == nil {
		return nil, telegramauthapp.ErrTelegramAuthConfigInvalid
	}
	ctx := input.Context
	if ctx == nil {
		ctx = context.Background()
	}
	verified, intent, userID, err := s.telegramAuthService.CompleteOIDCLogin(ctx, input.Code, input.State)
	if err != nil {
		return nil, err
	}
	if intent != telegramauthapp.IntentBind || userID != input.UserID {
		return nil, telegramauthapp.ErrTelegramAuthPayloadInvalid
	}
	return s.bindVerifiedTelegram(input.UserID, verified)
}
