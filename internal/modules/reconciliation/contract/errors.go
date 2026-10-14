package contract

import "errors"

var (
	ErrQueueUnavailable  = errors.New("reconciliation queue unavailable")
	ErrIncompleteResults = errors.New("reconciliation contains unchecked orders")
	ErrJobNotFound       = errors.New("reconciliation job not found")
	ErrItemNotFound      = errors.New("reconciliation item not found")
	ErrJobRunning        = errors.New("reconciliation job is already running")
)
