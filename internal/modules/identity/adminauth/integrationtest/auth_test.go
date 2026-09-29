package integrationtest

import (
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	admincontract "github.com/dujiao-next/internal/modules/identity/admin/contract"
	admindomain "github.com/dujiao-next/internal/modules/identity/admin/domain"
	adminstore "github.com/dujiao-next/internal/modules/identity/admin/infrastructure/gormstore"
	adminauthapp "github.com/dujiao-next/internal/modules/identity/adminauth/application"
	adminchallenge "github.com/dujiao-next/internal/modules/identity/adminauth/challenge"
	admintotpapp "github.com/dujiao-next/internal/modules/identity/adminauth/totp/application"

	"github.com/glebarez/sqlite"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func newAuthTestService(t *testing.T) (*adminauthapp.Service, *admintotpapp.Service, admincontract.Store) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&admindomain.Admin{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{
		App: config.AppConfig{SecretKey: "auth-test-key"},
		JWT: config.JWTConfig{SecretKey: "jwt-secret-for-test", ExpireHours: 24},
	}
	adminRepo := adminstore.New(db)
	return adminauthapp.NewService(cfg, adminRepo), admintotpapp.NewService(cfg, adminRepo, nil), adminRepo
}

func createAuthTestAdmin(t *testing.T, repo admincontract.Store, username, password string) *admindomain.Admin {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	admin := &admindomain.Admin{Username: username, PasswordHash: string(hash)}
	if err := repo.Create(admin); err != nil {
		t.Fatalf("create: %v", err)
	}
	return admin
}

func TestLoginWithoutTOTP(t *testing.T) {
	auth, _, repo := newAuthTestService(t)
	createAuthTestAdmin(t, repo, "noma", "secret123")

	res, err := auth.Login("noma", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if res.RequiresTOTP {
		t.Fatalf("expected RequiresTOTP=false")
	}
	if res.Token == "" {
		t.Fatalf("expected jwt token")
	}
}

func TestLoginWithTOTPReturnsChallenge(t *testing.T) {
	auth, totpSvc, repo := newAuthTestService(t)
	admin := createAuthTestAdmin(t, repo, "alice", "secret123")
	setupRes, _ := totpSvc.Setup(admin.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := totpSvc.Enable(admin.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}

	res, err := auth.Login("alice", "secret123")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if !res.RequiresTOTP {
		t.Fatalf("expected RequiresTOTP=true")
	}
	if res.ChallengeToken == "" || res.ChallengeJTI == "" {
		t.Fatalf("expected challenge token + jti")
	}
	claims, err := auth.ParseChallengeToken(res.ChallengeToken)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if claims.AdminID != admin.ID {
		t.Fatalf("admin id mismatch: %d vs %d", claims.AdminID, admin.ID)
	}
	if claims.Purpose != adminchallenge.PurposeTwoFactor {
		t.Fatalf("purpose mismatch")
	}
}

func TestLoginInvalidCredentials(t *testing.T) {
	auth, _, repo := newAuthTestService(t)
	createAuthTestAdmin(t, repo, "bob", "secret")
	if _, err := auth.Login("bob", "wrong"); err != adminauthapp.ErrInvalidCredentials {
		t.Fatalf("expected invalid creds, got %v", err)
	}
	if _, err := auth.Login("nosuch", "x"); err != adminauthapp.ErrInvalidCredentials {
		t.Fatalf("expected invalid creds for missing user, got %v", err)
	}
}

func TestParseChallengeRejectsWrongPurpose(t *testing.T) {
	auth, _, repo := newAuthTestService(t)
	createAuthTestAdmin(t, repo, "carol", "secret")
	admin, _ := repo.GetByUsername("carol")
	regular, _, err := auth.GenerateJWT(admin)
	if err != nil {
		t.Fatalf("gen jwt: %v", err)
	}
	if _, err := auth.ParseChallengeToken(regular); err == nil {
		t.Fatalf("expected error for non-challenge token")
	}
}
