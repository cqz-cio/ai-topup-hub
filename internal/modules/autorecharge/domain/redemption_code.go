package domain

import (
	"crypto/rand"
	"errors"
	"io"
	"strings"
)

const (
	// Exactly 32 symbols: omit I/O/0/1 to reduce transcription mistakes.
	redemptionAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	redemptionCodeSize = 25
)

var ErrInvalidRedemptionCode = errors.New("invalid_redemption_code")

// GenerateRedemptionCode returns five groups of five random symbols. This
// creates a code only; issuing and storing its redemption entitlement is separate.
func GenerateRedemptionCode() (string, error) {
	return generateRedemptionCode(rand.Reader)
}

func generateRedemptionCode(source io.Reader) (string, error) {
	var raw [redemptionCodeSize]byte
	for {
		if _, err := io.ReadFull(source, raw[:]); err != nil {
			return "", err
		}
		letters, digits := false, false
		for i, value := range raw {
			// 256 is divisible by 32, so this mapping has no modulo bias.
			raw[i] = redemptionAlphabet[int(value)%len(redemptionAlphabet)]
			letters = letters || raw[i] >= 'A'
			digits = digits || raw[i] <= '9'
		}
		if letters && digits {
			return formatRedemptionCode(string(raw[:])), nil
		}
		// Resample the entire code if it does not contain both character types.
	}
}

// NormalizeRedemptionCode accepts ASCII letters in either case, with either
// all four separators in the correct positions or no separators. It validates
// syntax only; entitlement, expiry and redemption state require a database check.
func NormalizeRedemptionCode(input string) (string, error) {
	input = strings.TrimSpace(input)
	if len(input) != redemptionCodeSize && len(input) != redemptionCodeSize+4 {
		return "", ErrInvalidRedemptionCode
	}
	var raw strings.Builder
	letters, digits := false, false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if len(input) == redemptionCodeSize+4 && i%6 == 5 {
			if ch != '-' {
				return "", ErrInvalidRedemptionCode
			}
			continue
		}
		if ch >= 'a' && ch <= 'z' {
			ch -= 'a' - 'A'
		}
		if !strings.ContainsRune(redemptionAlphabet, rune(ch)) {
			return "", ErrInvalidRedemptionCode
		}
		letters = letters || ch >= 'A'
		digits = digits || ch <= '9'
		raw.WriteByte(ch)
	}
	if !letters || !digits {
		return "", ErrInvalidRedemptionCode
	}
	return formatRedemptionCode(raw.String()), nil
}

func formatRedemptionCode(raw string) string {
	return raw[:5] + "-" + raw[5:10] + "-" + raw[10:15] + "-" + raw[15:20] + "-" + raw[20:25]
}
