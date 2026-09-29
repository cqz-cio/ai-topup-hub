// Package passwordpolicy owns the shared password-strength contract used by
// administrator and user identity applications.
package passwordpolicy

import (
	"errors"
	"unicode"
)

var ErrWeak = errors.New("weak password")

type Policy struct {
	MinLength      int
	RequireUpper   bool
	RequireLower   bool
	RequireNumber  bool
	RequireSpecial bool
}

type violation struct {
	key  string
	args []interface{}
}

func (e violation) Error() string        { return e.key }
func (e violation) Is(target error) bool { return target == ErrWeak }
func (e violation) Key() string          { return e.key }
func (e violation) Args() []interface{}  { return e.args }

// Validate checks a password against the configured strength policy.
func Validate(policy Policy, password string) error {
	if policy.MinLength <= 0 &&
		!policy.RequireUpper &&
		!policy.RequireLower &&
		!policy.RequireNumber &&
		!policy.RequireSpecial {
		return nil
	}
	if policy.MinLength > 0 && len([]rune(password)) < policy.MinLength {
		return violation{key: "error.password_min_length", args: []interface{}{policy.MinLength}}
	}

	var hasUpper, hasLower, hasNumber, hasSpecial bool
	for _, character := range password {
		switch {
		case unicode.IsUpper(character):
			hasUpper = true
		case unicode.IsLower(character):
			hasLower = true
		case unicode.IsDigit(character):
			hasNumber = true
		default:
			hasSpecial = true
		}
	}
	if policy.RequireUpper && !hasUpper {
		return violation{key: "error.password_require_upper"}
	}
	if policy.RequireLower && !hasLower {
		return violation{key: "error.password_require_lower"}
	}
	if policy.RequireNumber && !hasNumber {
		return violation{key: "error.password_require_number"}
	}
	if policy.RequireSpecial && !hasSpecial {
		return violation{key: "error.password_require_special"}
	}
	return nil
}
