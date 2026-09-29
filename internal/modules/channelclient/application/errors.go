package channelclientapp

import "errors"

var (
	ErrNotFound         = errors.New("channel client not found")
	ErrDisabled         = errors.New("channel client disabled")
	ErrSignatureInvalid = errors.New("channel signature invalid")
	ErrTimestampExpired = errors.New("channel timestamp expired")
)
