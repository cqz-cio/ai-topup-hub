package contract

import "errors"

var (
	ErrInvalid      = errors.New("promotion invalid")
	ErrNotFound     = errors.New("promotion not found")
	ErrUpdateFailed = errors.New("promotion update failed")
	ErrDeleteFailed = errors.New("promotion delete failed")
)
