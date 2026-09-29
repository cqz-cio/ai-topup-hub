package totpapplication

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const recoveryCodeBcryptCost = 10

// RecoveryCode is the persisted, hashed representation of one recovery code.
type RecoveryCode struct {
	Hash   string     `json:"hash"`
	UsedAt *time.Time `json:"used_at,omitempty"`
}

// GenerateRecoveryCodes creates plaintext recovery codes and their hashed JSON form.
func GenerateRecoveryCodes(count int) (plaintext []string, codesJSON string, err error) {
	plaintext = make([]string, 0, count)
	entries := make([]RecoveryCode, 0, count)
	for i := 0; i < count; i++ {
		raw := make([]byte, 5)
		if _, err := rand.Read(raw); err != nil {
			return nil, "", err
		}
		hexValue := hex.EncodeToString(raw)
		formatted := hexValue[:4] + "-" + hexValue[4:]
		hash, err := bcrypt.GenerateFromPassword([]byte(formatted), recoveryCodeBcryptCost)
		if err != nil {
			return nil, "", err
		}
		plaintext = append(plaintext, formatted)
		entries = append(entries, RecoveryCode{Hash: string(hash)})
	}
	encoded, err := json.Marshal(entries)
	if err != nil {
		return nil, "", err
	}
	return plaintext, string(encoded), nil
}

// DecodeRecoveryCodes parses the persisted recovery-code JSON.
func DecodeRecoveryCodes(value string) ([]RecoveryCode, error) {
	if value == "" {
		return []RecoveryCode{}, nil
	}
	var entries []RecoveryCode
	if err := json.Unmarshal([]byte(value), &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// MatchAndConsumeRecoveryCode marks the first matching unused code as consumed.
func MatchAndConsumeRecoveryCode(entriesJSON, code string, now time.Time) (string, error) {
	code = strings.TrimSpace(strings.ToLower(code))
	if code == "" {
		return "", ErrRecoveryCodeInvalid
	}
	entries, err := DecodeRecoveryCodes(entriesJSON)
	if err != nil {
		return "", fmt.Errorf("decode recovery: %w", err)
	}
	matched := -1
	for index, entry := range entries {
		if entry.UsedAt != nil {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(entry.Hash), []byte(code)) == nil {
			matched = index
			break
		}
	}
	if matched < 0 {
		return "", ErrRecoveryCodeInvalid
	}
	consumedAt := now
	entries[matched].UsedAt = &consumedAt
	encoded, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}
