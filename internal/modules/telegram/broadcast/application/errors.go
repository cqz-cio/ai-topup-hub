package broadcastapp

import "errors"

var (
	ErrInvalid          = errors.New("telegram broadcast invalid")
	ErrNotFound         = errors.New("telegram broadcast not found")
	ErrNoRecipients     = errors.New("telegram broadcast no recipients")
	ErrTokenUnavailable = errors.New("telegram bot token unavailable")
)
