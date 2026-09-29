package integrationtest

import (
	"strings"
	"testing"
	"time"

	totpapplication "github.com/dujiao-next/internal/modules/identity/totp/application"

	"github.com/dujiao-next/internal/config"
	admincontract "github.com/dujiao-next/internal/modules/identity/admin/contract"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	adminstore "github.com/dujiao-next/internal/modules/identity/admin/infrastructure/gormstore"
	admintotpapp "github.com/dujiao-next/internal/modules/identity/adminauth/totp/application"

	"github.com/glebarez/sqlite"
	"github.com/pquerna/otp/totp"
	"gorm.io/gorm"
)

func newTOTPTestService(t *testing.T) (*admintotpapp.Service, admincontract.Store, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&admindomain.Admin{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{App: config.AppConfig{SecretKey: "test-secret-key-for-totp"}}
	adminRepo := adminstore.New(db)
	svc := admintotpapp.NewService(cfg, adminRepo, nil)
	return svc, adminRepo, db
}

func createTOTPTestAdmin(t *testing.T, repo admincontract.Store, username string) *admindomain.Admin {
	t.Helper()
	admin := &admindomain.Admin{Username: username, PasswordHash: "x", IsSuper: false}
	if err := repo.Create(admin); err != nil {
		t.Fatalf("create admin: %v", err)
	}
	return admin
}

func TestSetupAndEnableFlow(t *testing.T) {
	svc, repo, _ := newTOTPTestService(t)
	admin := createTOTPTestAdmin(t, repo, "alice")

	setupRes, err := svc.Setup(admin.ID)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}
	if setupRes.Secret == "" || setupRes.OtpauthURL == "" {
		t.Fatalf("expected non-empty secret and otpauth url")
	}
	if !strings.HasPrefix(setupRes.OtpauthURL, "otpauth://totp/") {
		t.Fatalf("otpauth url malformed: %s", setupRes.OtpauthURL)
	}

	now := time.Now()
	code, err := totp.GenerateCode(setupRes.Secret, now)
	if err != nil {
		t.Fatalf("generate code: %v", err)
	}

	enableRes, err := svc.Enable(admin.ID, code)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	if len(enableRes.RecoveryCodes) != admintotpapp.RecoveryCodeCount {
		t.Fatalf("expected %d recovery codes, got %d", admintotpapp.RecoveryCodeCount, len(enableRes.RecoveryCodes))
	}
	for _, c := range enableRes.RecoveryCodes {
		if len(c) != 11 || !strings.Contains(c, "-") {
			t.Fatalf("recovery code format wrong: %q", c)
		}
	}
}

func TestEnableRejectsBadCode(t *testing.T) {
	svc, repo, _ := newTOTPTestService(t)
	admin := createTOTPTestAdmin(t, repo, "bob")
	if _, err := svc.Setup(admin.ID); err != nil {
		t.Fatalf("setup: %v", err)
	}
	_, err := svc.Enable(admin.ID, "000000")
	if err != totpapplication.ErrCodeInvalid {
		t.Fatalf("expected totpapplication.ErrCodeInvalid, got %v", err)
	}
}

func TestEnableExpiredPending(t *testing.T) {
	svc, repo, db := newTOTPTestService(t)
	admin := createTOTPTestAdmin(t, repo, "carol")
	if _, err := svc.Setup(admin.ID); err != nil {
		t.Fatalf("setup: %v", err)
	}
	past := time.Now().Add(-1 * time.Hour)
	if err := db.Model(&admindomain.Admin{}).Where("id = ?", admin.ID).Update("totp_pending_expires_at", past).Error; err != nil {
		t.Fatalf("force expire: %v", err)
	}
	_, err := svc.Enable(admin.ID, "123456")
	if err != totpapplication.ErrPendingExpired {
		t.Fatalf("expected pending expired, got %v", err)
	}
}

func TestVerifyChallengeCode(t *testing.T) {
	svc, repo, _ := newTOTPTestService(t)
	admin := createTOTPTestAdmin(t, repo, "dave")
	setupRes, _ := svc.Setup(admin.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := svc.Enable(admin.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}
	updated, _ := repo.GetByID(admin.ID)
	if updated.TOTPEnabledAt == nil {
		t.Fatalf("expected enabled")
	}
	good, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if err := svc.VerifyChallengeCode(admin.ID, good); err != nil {
		t.Fatalf("verify good: %v", err)
	}
	if err := svc.VerifyChallengeCode(admin.ID, "000000"); err != totpapplication.ErrCodeInvalid {
		t.Fatalf("expected invalid for 000000, got %v", err)
	}
}

func TestRecoveryCodeConsume(t *testing.T) {
	svc, repo, _ := newTOTPTestService(t)
	admin := createTOTPTestAdmin(t, repo, "eve")
	setupRes, _ := svc.Setup(admin.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	enableRes, err := svc.Enable(admin.ID, code)
	if err != nil {
		t.Fatalf("enable: %v", err)
	}
	first := enableRes.RecoveryCodes[0]

	if err := svc.VerifyChallengeRecoveryCode(admin.ID, first); err != nil {
		t.Fatalf("first use: %v", err)
	}
	if err := svc.VerifyChallengeRecoveryCode(admin.ID, first); err != totpapplication.ErrRecoveryCodeInvalid {
		t.Fatalf("expected totpapplication.ErrRecoveryCodeInvalid on reuse, got %v", err)
	}
	st, _ := svc.GetStatus(admin.ID)
	if st.RecoveryCodesRemaining != admintotpapp.RecoveryCodeCount-1 {
		t.Fatalf("expected %d remaining, got %d", admintotpapp.RecoveryCodeCount-1, st.RecoveryCodesRemaining)
	}
}

func TestRegenerateRecoveryCodes(t *testing.T) {
	svc, repo, _ := newTOTPTestService(t)
	admin := createTOTPTestAdmin(t, repo, "frank")
	setupRes, _ := svc.Setup(admin.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	first, _ := svc.Enable(admin.ID, code)

	now := time.Now().Add(31 * time.Second)
	newCode, _ := totp.GenerateCode(setupRes.Secret, now)
	svc = admintotpapp.NewService(
		&config.Config{App: config.AppConfig{SecretKey: "test-secret-key-for-totp"}},
		repo,
		nil,
		admintotpapp.WithClock(func() time.Time { return now }),
	)
	regen, err := svc.RegenerateRecoveryCodes(admin.ID, newCode)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if len(regen) != admintotpapp.RecoveryCodeCount {
		t.Fatalf("expected %d, got %d", admintotpapp.RecoveryCodeCount, len(regen))
	}
	for _, oldCode := range first.RecoveryCodes {
		if err := svc.VerifyChallengeRecoveryCode(admin.ID, oldCode); err != totpapplication.ErrRecoveryCodeInvalid {
			t.Fatalf("expected old code invalid, got %v", err)
		}
	}
}

func TestDisableWithCode(t *testing.T) {
	svc, repo, _ := newTOTPTestService(t)
	admin := createTOTPTestAdmin(t, repo, "grace")
	setupRes, _ := svc.Setup(admin.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := svc.Enable(admin.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}
	good, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if err := svc.Disable(admin.ID, good, false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	st, _ := svc.GetStatus(admin.ID)
	if st.Enabled {
		t.Fatalf("expected disabled")
	}
}

func TestDisableWithRecoveryCode(t *testing.T) {
	svc, repo, _ := newTOTPTestService(t)
	admin := createTOTPTestAdmin(t, repo, "henry")
	setupRes, _ := svc.Setup(admin.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	enableRes, _ := svc.Enable(admin.ID, code)
	if err := svc.Disable(admin.ID, enableRes.RecoveryCodes[0], true); err != nil {
		t.Fatalf("disable with recovery: %v", err)
	}
}

func TestAdminResetRejectsSelf(t *testing.T) {
	svc, repo, _ := newTOTPTestService(t)
	admin := createTOTPTestAdmin(t, repo, "isaac")
	if err := svc.AdminReset(admin.ID, admin.ID); err != admintotpapp.ErrCannotResetSelf {
		t.Fatalf("expected cannot reset self, got %v", err)
	}
}

func TestAdminResetClearsTarget(t *testing.T) {
	svc, repo, _ := newTOTPTestService(t)
	operator := createTOTPTestAdmin(t, repo, "operator")
	target := createTOTPTestAdmin(t, repo, "target")
	setupRes, _ := svc.Setup(target.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := svc.Enable(target.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}
	if err := svc.AdminReset(operator.ID, target.ID); err != nil {
		t.Fatalf("reset: %v", err)
	}
	updated, _ := repo.GetByID(target.ID)
	if updated.TOTPEnabledAt != nil || updated.TOTPSecret != "" {
		t.Fatalf("expected cleared, got enabled_at=%v secret_len=%d", updated.TOTPEnabledAt, len(updated.TOTPSecret))
	}
}
