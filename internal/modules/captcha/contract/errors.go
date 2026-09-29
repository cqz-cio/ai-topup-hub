package contract

import (
	"errors"

	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

var (
	ErrConfigInvalid = settingssecurity.ErrCaptchaConfigInvalid
	ErrRequired      = errors.New("captcha required")
	ErrInvalid       = errors.New("captcha invalid")
	ErrVerifyFailed  = errors.New("captcha verify failed")
)
