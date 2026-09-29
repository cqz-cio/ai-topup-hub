package application

import "errors"

var (
	ErrNotFound           = errors.New("admin not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInvalidPassword    = errors.New("invalid password")
)
