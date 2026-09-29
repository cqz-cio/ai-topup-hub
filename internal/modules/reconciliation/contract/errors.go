package contract

import "errors"

var (
	ErrJobNotFound  = errors.New("reconciliation job not found")
	ErrItemNotFound = errors.New("reconciliation item not found")
	ErrJobRunning   = errors.New("reconciliation job is already running")
)
