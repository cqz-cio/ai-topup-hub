package contract

import "errors"

var (
	ErrRefNotFound = errors.New("downstream order ref not found")
	ErrInvalidRef  = errors.New("invalid downstream order ref")
)
