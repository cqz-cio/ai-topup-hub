package application

import "errors"

var (
	ErrGoogleAuthDisabled      = errors.New("google auth disabled")
	ErrGoogleAuthConfigInvalid = errors.New("google auth config invalid")
	ErrGoogleCredentialInvalid = errors.New("google credential invalid")
	ErrGoogleCredentialExpired = errors.New("google credential expired")
	ErrGoogleEmailUnverified   = errors.New("google email unverified")
	ErrGoogleJWKSUnavailable   = errors.New("google jwks unavailable")
)
