package challenge

import "time"

const (
	PurposeTwoFactor = "2fa_challenge"
	TTL              = 5 * time.Minute
	MaxFailures      = 5
)

func FailureKey(jti string) string {
	return "2fa:challenge:" + jti + ":fails"
}

func RevocationKey(jti string) string {
	return "2fa:challenge:" + jti + ":revoked"
}
