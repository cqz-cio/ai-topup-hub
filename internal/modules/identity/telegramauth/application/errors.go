package telegramauthapp

import (
	"errors"

	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

var (
	ErrTelegramAuthDisabled         = errors.New("telegram auth disabled")
	ErrTelegramAuthConfigInvalid    = settingssecurity.ErrTelegramAuthConfigInvalid
	ErrTelegramAuthPayloadInvalid   = errors.New("telegram auth payload invalid")
	ErrTelegramAuthSignatureInvalid = errors.New("telegram auth signature invalid")
	ErrTelegramAuthExpired          = errors.New("telegram auth expired")
	ErrTelegramAuthReplay           = errors.New("telegram auth replay")
	ErrTelegramOIDCStateInvalid     = errors.New("telegram oidc state invalid")
	ErrTelegramOIDCTokenExchange    = errors.New("telegram oidc token exchange failed")
	ErrTelegramOIDCIDTokenInvalid   = errors.New("telegram oidc id token invalid")
)
