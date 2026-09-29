package jwttoken

import "github.com/golang-jwt/jwt/v5"

const (
	TypeAccess             = "access"
	TypeTwoFactorChallenge = "2fa_challenge"
)

// IsAccessType accepts an empty type for tokens issued before typ was added.
func IsAccessType(tokenType string) bool {
	return tokenType == "" || tokenType == TypeAccess
}

// NewHS256Parser rejects algorithm-confusion attempts before claims parsing.
func NewHS256Parser() *jwt.Parser {
	return jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
}
