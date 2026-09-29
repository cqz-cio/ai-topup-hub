package contract

import (
	"errors"

	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
)

var (
	ErrConfigInvalid          = settingsmessaging.ErrNotificationConfigInvalid
	ErrSendFailed             = errors.New("notification send failed")
	ErrEventInvalid           = errors.New("notification event invalid")
	ErrInvalidEmail           = errors.New("invalid email")
	ErrEmailServiceDisabled   = errors.New("email service disabled")
	ErrEmailNotConfigured     = errors.New("email service not configured")
	ErrEmailRecipientRejected = errors.New("email recipient rejected")
)
