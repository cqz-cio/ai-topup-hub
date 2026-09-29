package application

import "errors"

var (
	ErrNotFound        = errors.New("admin not found")
	ErrCannotResetSelf = errors.New("cannot reset self via super admin endpoint")
)
