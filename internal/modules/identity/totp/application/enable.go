package totpapplication

import (
	"errors"
	"fmt"
	"time"

	"github.com/dujiao-next/internal/crypto"
)

var (
	ErrSubjectNotFound     = errors.New("totp subject not found")
	ErrAlreadyEnabled      = errors.New("totp already enabled")
	ErrNotEnabled          = errors.New("totp not enabled")
	ErrPendingExpired      = errors.New("totp pending secret expired")
	ErrCodeInvalid         = errors.New("totp code invalid")
	ErrRecoveryCodeInvalid = errors.New("recovery code invalid or used")
	ErrTooManyAttempts     = errors.New("too many failed attempts")
)

// EnableInput contains account-independent data needed to activate TOTP.
type EnableInput struct {
	AccountID         uint
	EncryptionKey     []byte
	Code              string
	RecoveryCodeCount int
	Now               func() time.Time

	PendingSecret    string
	PendingExpiresAt *time.Time
	CheckFailures    func(uint) error
	VerifyCode       func(secret, code string) bool
	BumpFailure      func(uint)
}

// EnableResult is the prepared state persisted after the first valid code.
type EnableResult struct {
	EncryptedSecret   string
	RecoveryCodes     []string
	RecoveryCodesJSON string
	EnabledAt         time.Time
}

// EnableSubject is the persisted TOTP state loaded for an account.
type EnableSubject struct {
	Exists           bool
	EnabledAt        *time.Time
	PendingSecret    string
	PendingExpiresAt *time.Time
}

// EnableStore supplies the account-specific persistence and failure policy.
type EnableStore interface {
	LoadEnableSubject(accountID uint) (EnableSubject, error)
	SaveEnabled(accountID uint, result *EnableResult) error
	ClearEnableFailures(accountID uint)
	CheckEnableFailures(accountID uint) error
	VerifyEnableCode(secret, code string) bool
	BumpEnableFailure(accountID uint)
}

// Enable runs the shared first-code activation workflow.
func Enable(store EnableStore, input EnableInput) (*EnableResult, error) {
	subject, err := store.LoadEnableSubject(input.AccountID)
	if err != nil {
		return nil, err
	}
	if !subject.Exists {
		return nil, ErrSubjectNotFound
	}
	if subject.EnabledAt != nil {
		return nil, ErrAlreadyEnabled
	}

	input.PendingSecret = subject.PendingSecret
	input.PendingExpiresAt = subject.PendingExpiresAt
	input.CheckFailures = store.CheckEnableFailures
	input.VerifyCode = store.VerifyEnableCode
	input.BumpFailure = store.BumpEnableFailure

	result, err := PrepareEnable(input)
	if err != nil {
		return nil, err
	}
	if err := store.SaveEnabled(input.AccountID, result); err != nil {
		return nil, err
	}
	store.ClearEnableFailures(input.AccountID)
	return result, nil
}

// PrepareEnable validates and prepares activation without persistence.
func PrepareEnable(input EnableInput) (*EnableResult, error) {
	now := input.Now
	if now == nil {
		now = time.Now
	}
	if input.PendingSecret == "" || input.PendingExpiresAt == nil || now().After(*input.PendingExpiresAt) {
		return nil, ErrPendingExpired
	}
	if input.CheckFailures != nil {
		if err := input.CheckFailures(input.AccountID); err != nil {
			return nil, err
		}
	}
	secret, err := crypto.Decrypt(input.EncryptionKey, input.PendingSecret)
	if err != nil {
		return nil, fmt.Errorf("decrypt pending: %w", err)
	}
	if input.VerifyCode == nil || !input.VerifyCode(secret, input.Code) {
		if input.BumpFailure != nil {
			input.BumpFailure(input.AccountID)
		}
		return nil, ErrCodeInvalid
	}
	encryptedSecret, err := crypto.Encrypt(input.EncryptionKey, secret)
	if err != nil {
		return nil, fmt.Errorf("re-encrypt secret: %w", err)
	}
	recoveryCodes, recoveryCodesJSON, err := GenerateRecoveryCodes(input.RecoveryCodeCount)
	if err != nil {
		return nil, err
	}
	return &EnableResult{
		EncryptedSecret:   encryptedSecret,
		RecoveryCodes:     recoveryCodes,
		RecoveryCodesJSON: recoveryCodesJSON,
		EnabledAt:         now(),
	}, nil
}
