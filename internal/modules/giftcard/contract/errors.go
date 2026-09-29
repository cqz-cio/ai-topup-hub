package contract

import "errors"

var (
	ErrInvalid           = errors.New("gift card invalid")
	ErrNotFound          = errors.New("gift card not found")
	ErrExpired           = errors.New("gift card expired")
	ErrDisabled          = errors.New("gift card disabled")
	ErrRedeemed          = errors.New("gift card redeemed")
	ErrCreateFailed      = errors.New("gift card create failed")
	ErrFetchFailed       = errors.New("gift card fetch failed")
	ErrUpdateFailed      = errors.New("gift card update failed")
	ErrDeleteFailed      = errors.New("gift card delete failed")
	ErrBatchCreateFailed = errors.New("gift card batch create failed")
)
