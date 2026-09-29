package contract

import "errors"

var (
	ErrGatewayConfigInvalid      = errors.New("payment provider config invalid")
	ErrGatewayRequestFailed      = errors.New("payment provider request failed")
	ErrGatewayResponseInvalid    = errors.New("payment provider response invalid")
	ErrGatewaySignatureInvalid   = errors.New("payment provider signature invalid")
	ErrGatewayAuthFailed         = errors.New("payment provider auth failed")
	ErrGatewayUnsupportedChannel = errors.New("payment channel type not supported by provider")
	ErrGatewayProviderNotFound   = errors.New("payment provider not found in registry")
)
