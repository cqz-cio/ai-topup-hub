package application

import (
	"context"
	"time"

	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
)

// BindTelegramInput 绑定 Telegram 输入
type BindTelegramInput struct {
	UserID  uint
	Payload telegramauthapp.LoginPayload
	Context context.Context
}

// BindTelegramMiniAppInput Telegram Mini App 绑定输入
type BindTelegramMiniAppInput struct {
	UserID   uint
	InitData string
	Context  context.Context
}

// TelegramBinding is the account-safe Telegram binding view.
type TelegramBinding struct {
	Identity  *externalidentitydomain.Identity
	CanUnbind bool
}

// BindTelegram 绑定 Telegram
func (s *Service) BindTelegram(input BindTelegramInput) (*externalidentitydomain.Identity, error) {
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
	verified, err := s.telegramAuthService.VerifyLogin(ctx, input.Payload)
	if err != nil {
		return nil, err
	}
	return s.bindVerifiedTelegram(input.UserID, verified)
}

// BindTelegramMiniApp 绑定当前用户的 Telegram Mini App 身份
func (s *Service) BindTelegramMiniApp(input BindTelegramMiniAppInput) (*externalidentitydomain.Identity, error) {
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
	verified, err := s.telegramAuthService.VerifyMiniAppInitData(ctx, input.InitData)
	if err != nil {
		return nil, err
	}
	return s.bindVerifiedTelegram(input.UserID, verified)
}

func (s *Service) bindVerifiedTelegram(userID uint, verified *telegramauthapp.IdentityVerified) (*externalidentitydomain.Identity, error) {
	if _, err := s.getActiveUserByID(userID); err != nil {
		return nil, err
	}

	occupied, err := s.getTelegramIdentityByVerifiedID(verified)
	if err != nil {
		return nil, err
	}
	if occupied != nil && occupied.UserID != userID {
		return nil, ErrUserOAuthIdentityExists
	}

	current, err := s.userOAuthIdentityRepo.GetByUserProvider(userID, verified.Provider)
	if err != nil {
		return nil, err
	}
	if current != nil && !telegramProviderUserIDMatchesVerified(current.ProviderUserID, verified) {
		return nil, ErrUserOAuthAlreadyBound
	}
	if current == nil {
		current = &externalidentitydomain.Identity{
			UserID:         userID,
			Provider:       verified.Provider,
			ProviderUserID: verified.ProviderUserID,
			Username:       verified.Username,
			AvatarURL:      verified.AvatarURL,
			AuthAt:         &verified.AuthAt,
			CreatedAt:      time.Now(),
			UpdatedAt:      time.Now(),
		}
		if err := s.userOAuthIdentityRepo.Create(current); err != nil {
			occupied, occupiedErr := s.getTelegramIdentityByVerifiedID(verified)
			if occupiedErr == nil && occupied != nil && occupied.UserID != userID {
				return nil, ErrUserOAuthIdentityExists
			}
			latest, latestErr := s.userOAuthIdentityRepo.GetByUserProvider(userID, verified.Provider)
			if latestErr == nil && latest != nil {
				if !telegramProviderUserIDMatchesVerified(latest.ProviderUserID, verified) {
					return nil, ErrUserOAuthAlreadyBound
				}
				return latest, nil
			}
			return nil, err
		}
		return current, nil
	}

	identityChanged, err := s.canonicalizeTelegramProviderUserID(verified, current)
	if err != nil {
		return nil, err
	}
	if applyTelegramIdentity(verified, current) || identityChanged {
		current.UpdatedAt = time.Now()
		if err := s.userOAuthIdentityRepo.Update(current); err != nil {
			return nil, err
		}
	}
	return current, nil
}

// UnbindTelegram 解绑 Telegram
func (s *Service) UnbindTelegram(userID uint) error {
	if err := s.unbindExternalIdentity(userID, constants.UserOAuthProviderTelegram); err != nil {
		if err == errExternalIdentityUnbindLocked {
			return ErrTelegramUnbindRequiresEmail
		}
		return err
	}
	return nil
}

// GetTelegramBinding 获取 Telegram 绑定
func (s *Service) GetTelegramBinding(userID uint) (*TelegramBinding, error) {
	if userID == 0 {
		return nil, ErrNotFound
	}
	if s.userOAuthIdentityRepo == nil {
		return nil, telegramauthapp.ErrTelegramAuthConfigInvalid
	}
	user, err := s.getActiveUserByID(userID)
	if err != nil {
		return nil, err
	}
	identity, err := s.userOAuthIdentityRepo.GetByUserProvider(userID, constants.UserOAuthProviderTelegram)
	if err != nil {
		return nil, err
	}
	result := &TelegramBinding{Identity: identity}
	if identity == nil {
		return result, nil
	}
	result.CanUnbind, err = s.canUnbindExternalIdentity(user, identity)
	if err != nil {
		return nil, err
	}
	return result, nil
}
