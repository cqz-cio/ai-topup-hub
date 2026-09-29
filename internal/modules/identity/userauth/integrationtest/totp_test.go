package integrationtest

import (
	"strings"
	"testing"
	"time"

	totpapplication "github.com/dujiao-next/internal/modules/identity/totp/application"
	usertotpapp "github.com/dujiao-next/internal/modules/identity/userauth/totp/application"

	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	"github.com/dujiao-next/internal/config"

	"github.com/glebarez/sqlite"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

func newUserTOTPTestService(t *testing.T) (*usertotpapp.Service, usercontract.Store, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{App: config.AppConfig{SecretKey: "test-secret-key-for-user-totp"}}
	userRepo := userstore.New(db)
	svc := usertotpapp.NewService(cfg, userRepo, nil)
	return svc, userRepo, db
}

func createUserTOTPTestUser(t *testing.T, repo usercontract.Store, email string) *userdomain.User {
	t.Helper()
	user := &userdomain.User{
		Email:        email,
		PasswordHash: "x",
		DisplayName:  email,
		Status:       "active",
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestUserTOTPSetupAndEnableFlow(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "alice@example.com")

	setupRes, err := svc.Setup(user.ID)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if setupRes.Secret == "" || setupRes.OtpauthURL == "" {
		t.Fatalf("expected non-empty secret and otpauth url")
	}
	if !strings.HasPrefix(setupRes.OtpauthURL, "otpauth://totp/") {
		t.Fatalf("otpauth url malformed: %s", setupRes.OtpauthURL)
	}

	code, err := totp.GenerateCode(setupRes.Secret, time.Now())
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	enableRes, err := svc.Enable(user.ID, code)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if len(enableRes.RecoveryCodes) != usertotpapp.RecoveryCodeCount {
		t.Fatalf("expected %d recovery codes, got %d", usertotpapp.RecoveryCodeCount, len(enableRes.RecoveryCodes))
	}
	for _, c := range enableRes.RecoveryCodes {
		if len(c) != 11 || !strings.Contains(c, "-") {
			t.Fatalf("recovery code format wrong: %q", c)
		}
	}
}

func TestUserTOTPEnableBumpsTokenVersion(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "tokenbump@example.com")

	setupRes, _ := svc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := svc.Enable(user.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}

	updated, _ := repo.GetByID(user.ID)
	if updated.TokenVersion != user.TokenVersion+1 {
		t.Fatalf("expected token_version to bump from %d to %d, got %d", user.TokenVersion, user.TokenVersion+1, updated.TokenVersion)
	}
	if updated.TokenInvalidBefore == nil {
		t.Fatalf("expected token_invalid_before set after enable")
	}
}

func TestUserTOTPEnableRejectsBadCode(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "bob@example.com")
	if _, err := svc.Setup(user.ID); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := svc.Enable(user.ID, "000000"); err != totpapplication.ErrCodeInvalid {
		t.Fatalf("expected totpapplication.ErrCodeInvalid, got %v", err)
	}
}

func TestUserTOTPEnableExpiredPending(t *testing.T) {
	svc, repo, db := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "carol@example.com")
	if _, err := svc.Setup(user.ID); err != nil {
		t.Fatalf("setup: %v", err)
	}
	past := time.Now().Add(-1 * time.Hour)
	if err := db.Model(&userdomain.User{}).Where("id = ?", user.ID).Update("totp_pending_expires_at", past).Error; err != nil {
		t.Fatalf("force expire: %v", err)
	}
	if _, err := svc.Enable(user.ID, "123456"); err != totpapplication.ErrPendingExpired {
		t.Fatalf("expected pending expired, got %v", err)
	}
}

func TestUserTOTPEnableRejectedWhenAlreadyEnabled(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "dup@example.com")
	setupRes, _ := svc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := svc.Enable(user.ID, code); err != nil {
		t.Fatalf("first enable: %v", err)
	}
	if _, err := svc.Enable(user.ID, code); err != totpapplication.ErrAlreadyEnabled {
		t.Fatalf("expected already enabled, got %v", err)
	}
	// Setup 也应被拒
	if _, err := svc.Setup(user.ID); err != totpapplication.ErrAlreadyEnabled {
		t.Fatalf("expected setup blocked when enabled, got %v", err)
	}
}

func TestUserTOTPVerifyChallengeCode(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "dave@example.com")
	setupRes, _ := svc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := svc.Enable(user.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}
	good, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if err := svc.VerifyChallengeCode(user.ID, good); err != nil {
		t.Fatalf("verify good: %v", err)
	}
	if err := svc.VerifyChallengeCode(user.ID, "000000"); err != totpapplication.ErrCodeInvalid {
		t.Fatalf("expected invalid for 000000, got %v", err)
	}
}

func TestUserTOTPVerifyChallengeRejectsWhenNotEnabled(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "noenable@example.com")
	if err := svc.VerifyChallengeCode(user.ID, "123456"); err != totpapplication.ErrNotEnabled {
		t.Fatalf("expected not enabled, got %v", err)
	}
	if err := svc.VerifyChallengeRecoveryCode(user.ID, "abcd-efgh"); err != totpapplication.ErrNotEnabled {
		t.Fatalf("expected not enabled for recovery, got %v", err)
	}
}

func TestUserTOTPRecoveryCodeOneShot(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "eve@example.com")
	setupRes, _ := svc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	enableRes, err := svc.Enable(user.ID, code)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	first := enableRes.RecoveryCodes[0]

	if err := svc.VerifyChallengeRecoveryCode(user.ID, first); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if err := svc.VerifyChallengeRecoveryCode(user.ID, first); err != totpapplication.ErrRecoveryCodeInvalid {
		t.Fatalf("expected totpapplication.ErrRecoveryCodeInvalid on reuse, got %v", err)
	}
	st, _ := svc.GetStatus(user.ID)
	if st.RecoveryCodesRemaining != usertotpapp.RecoveryCodeCount-1 {
		t.Fatalf("expected %d remaining, got %d", usertotpapp.RecoveryCodeCount-1, st.RecoveryCodesRemaining)
	}
	if st.RecoveryCodesTotal != usertotpapp.RecoveryCodeCount {
		t.Fatalf("expected total %d, got %d", usertotpapp.RecoveryCodeCount, st.RecoveryCodesTotal)
	}
}

func TestUserTOTPRecoveryCodeRejectsBogus(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "evil@example.com")
	setupRes, _ := svc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := svc.Enable(user.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := svc.VerifyChallengeRecoveryCode(user.ID, "0000-0000"); err != totpapplication.ErrRecoveryCodeInvalid {
		t.Fatalf("expected invalid, got %v", err)
	}
	if err := svc.VerifyChallengeRecoveryCode(user.ID, ""); err != totpapplication.ErrRecoveryCodeInvalid {
		t.Fatalf("expected invalid for empty, got %v", err)
	}
}

func TestUserTOTPRegenerateRecoveryCodes(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "frank@example.com")
	setupRes, _ := svc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	first, err := svc.Enable(user.ID, code)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}

	// 时间往后跳一个 TOTP 周期，避免使用同一动态码触发重放保护
	future := time.Now().Add(31 * time.Second)
	newCode, _ := totp.GenerateCode(setupRes.Secret, future)
	svc = usertotpapp.NewService(&config.Config{App: config.AppConfig{SecretKey: "test-secret-key-for-user-totp"}}, repo, nil, usertotpapp.WithClock(func() time.Time { return future }))
	regen, err := svc.RegenerateRecoveryCodes(user.ID, newCode)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if len(regen) != usertotpapp.RecoveryCodeCount {
		t.Fatalf("expected %d, got %d", usertotpapp.RecoveryCodeCount, len(regen))
	}
	for _, oldCode := range first.RecoveryCodes {
		if err := svc.VerifyChallengeRecoveryCode(user.ID, oldCode); err != totpapplication.ErrRecoveryCodeInvalid {
			t.Fatalf("expected old code invalid, got %v", err)
		}
	}
	if err := svc.VerifyChallengeRecoveryCode(user.ID, regen[0]); err != nil {
		t.Fatalf("expected new code valid: %v", err)
	}
}

func TestUserTOTPDisableWithCode(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "grace@example.com")
	setupRes, _ := svc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := svc.Enable(user.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}
	good, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if err := svc.Disable(user.ID, good, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	st, _ := svc.GetStatus(user.ID)
	if st.Enabled {
		t.Fatalf("expected disabled")
	}

	updated, _ := repo.GetByID(user.ID)
	if updated.TOTPSecret != "" || updated.RecoveryCodes != "" {
		t.Fatalf("expected fields cleared, got secret_len=%d recovery_len=%d", len(updated.TOTPSecret), len(updated.RecoveryCodes))
	}
	if updated.TokenInvalidBefore == nil {
		t.Fatalf("expected token_invalid_before bumped on disable")
	}
}

func TestUserTOTPDisableWithRecoveryCode(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "henry@example.com")
	setupRes, _ := svc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	enableRes, _ := svc.Enable(user.ID, code)
	if err := svc.Disable(user.ID, enableRes.RecoveryCodes[0], true); err != nil {
		t.Fatalf("disable with recovery: %v", err)
	}
	st, _ := svc.GetStatus(user.ID)
	if st.Enabled {
		t.Fatalf("expected disabled after recovery code")
	}
}

func TestUserTOTPDisableRejectsWhenNotEnabled(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "iris@example.com")
	if err := svc.Disable(user.ID, "123456", false); err != totpapplication.ErrNotEnabled {
		t.Fatalf("expected not enabled, got %v", err)
	}
}

func TestUserTOTPGetStatusForUnknownUser(t *testing.T) {
	svc, _, _ := newUserTOTPTestService(t)
	if _, err := svc.GetStatus(99999); err != usertotpapp.ErrNotFound {
		t.Fatalf("expected not found, got %v", err)
	}
}

func TestUserTOTPAdminResetClearsAndBumpsTokenVersion(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "reset@example.com")
	setupRes, _ := svc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := svc.Enable(user.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}
	enabledSnap, _ := repo.GetByID(user.ID)

	target, err := svc.AdminResetUser2FA(1, user.ID)
	if err != nil {
		t.Fatalf("admin reset: %v", err)
	}
	if target == nil || target.ID != user.ID {
		t.Fatalf("expected returned target user with ID=%d", user.ID)
	}
	if target.Email != "reset@example.com" {
		t.Fatalf("expected target email returned for audit, got %q", target.Email)
	}

	cleared, _ := repo.GetByID(user.ID)
	if cleared.TOTPEnabledAt != nil {
		t.Fatalf("expected totp_enabled_at cleared, got %v", cleared.TOTPEnabledAt)
	}
	if cleared.TOTPSecret != "" {
		t.Fatalf("expected totp_secret cleared, got %q", cleared.TOTPSecret)
	}
	if cleared.RecoveryCodes != "" {
		t.Fatalf("expected recovery_codes cleared, got %q", cleared.RecoveryCodes)
	}
	if cleared.TokenVersion != enabledSnap.TokenVersion+1 {
		t.Fatalf("expected token_version to bump from %d to %d, got %d",
			enabledSnap.TokenVersion, enabledSnap.TokenVersion+1, cleared.TokenVersion)
	}
	if cleared.TokenInvalidBefore == nil {
		t.Fatalf("expected token_invalid_before set after admin reset")
	}
}

func TestUserTOTPAdminResetRejectsWhenNotEnabled(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "noreset@example.com")
	if _, err := svc.AdminResetUser2FA(1, user.ID); err != totpapplication.ErrNotEnabled {
		t.Fatalf("expected totpapplication.ErrNotEnabled, got %v", err)
	}
}

func TestUserTOTPAdminResetRejectsUnknownUser(t *testing.T) {
	svc, _, _ := newUserTOTPTestService(t)
	if _, err := svc.AdminResetUser2FA(1, 99999); err != usertotpapp.ErrNotFound {
		t.Fatalf("expected usertotpapp.ErrNotFound, got %v", err)
	}
}

func TestUserTOTPAdminResetRequiresOperatorID(t *testing.T) {
	svc, repo, _ := newUserTOTPTestService(t)
	user := createUserTOTPTestUser(t, repo, "noopid@example.com")
	if _, err := svc.AdminResetUser2FA(0, user.ID); err == nil {
		t.Fatalf("expected error when operatorID=0")
	}
}
