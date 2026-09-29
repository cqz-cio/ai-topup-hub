package passwordpolicy

import (
	"errors"
	"testing"
)

func TestValidateReturnsLocalizedViolationDetails(t *testing.T) {
	err := Validate(Policy{MinLength: 8}, "short")
	if !errors.Is(err, ErrWeak) {
		t.Fatalf("expected weak-password sentinel, got %v", err)
	}
	detailed, ok := err.(interface {
		Key() string
		Args() []interface{}
	})
	if !ok {
		t.Fatalf("violation does not expose localization details: %T", err)
	}
	if detailed.Key() != "error.password_min_length" || len(detailed.Args()) != 1 || detailed.Args()[0] != 8 {
		t.Fatalf("unexpected violation details: key=%q args=%#v", detailed.Key(), detailed.Args())
	}
}

func TestValidateEnforcesEveryCharacterClass(t *testing.T) {
	policy := Policy{MinLength: 8, RequireUpper: true, RequireLower: true, RequireNumber: true, RequireSpecial: true}
	if err := Validate(policy, "Strong1!"); err != nil {
		t.Fatalf("valid password rejected: %v", err)
	}
	for name, password := range map[string]string{
		"upper":   "missing1!",
		"lower":   "MISSING1!",
		"number":  "Missing!!",
		"special": "Missing12",
	} {
		t.Run(name, func(t *testing.T) {
			if err := Validate(policy, password); !errors.Is(err, ErrWeak) {
				t.Fatalf("expected weak-password error, got %v", err)
			}
		})
	}
}
