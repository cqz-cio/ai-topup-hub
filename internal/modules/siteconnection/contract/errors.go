package contract

import "errors"

var (
	ErrNotFound = errors.New("site connection not found")
	ErrInvalid  = errors.New("site connection is invalid")
)
