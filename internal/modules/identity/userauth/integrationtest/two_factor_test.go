package integrationtest

import (
	"fmt"
	"testing"
	"time"

	usercontract "github.com/dujiao-next/internal/modules/identity/user/contract"
	userstore "github.com/dujiao-next/internal/modules/identity/user/infrastructure/gormstore"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	settingsstore "github.com/dujiao-next/internal/modules/settings/infrastructure/gormstore"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	emailverificationdomain "github.com/dujiao-next/internal/modules/identity/emailverification/domain"
	emailverificationstore "github.com/dujiao-next/internal/modules/identity/emailverification/infrastructure/gormstore"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	externalidentitystore "github.com/dujiao-next/internal/modules/identity/externalidentity/infrastructure/gormstore"
	"github.com/dujiao-next/internal/modules/identity/jwttoken"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/dujiao-next/internal/modules/identity/userauth/challenge"
	usertotpapp "github.com/dujiao-next/internal/modules/identity/userauth/totp/application"

	"github.com/glebarez/sqlite"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func newUser2FATestServices(t *testing.T) (*userauthapp.Service, *usertotpapp.Service, usercontract.Store, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:user_auth_2fa_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if err := db.AutoMigrate(&userdomain.User{}, &externalidentitydomain.Identity{}, &emailverificationdomain.Code{}, &settingsstore.SettingRecord{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cfg := &config.Config{
		App: config.AppConfig{SecretKey: "test-app-secret-2fa"},
		UserJWT: config.JWTConfig{
			SecretKey:   "user-jwt-test-secret-2fa",
			ExpireHours: 24,
		},
	}
	userRepo := userstore.New(db)
	authSvc := userauthapp.NewService(
		cfg,
		userRepo,
		externalidentitystore.New(db),
		emailverificationstore.New(db),
		nil,
		nil,
		nil,
	)
	totpSvc := usertotpapp.NewService(cfg, userRepo, nil)
	return authSvc, totpSvc, userRepo, db
}

func createActiveUser(t *testing.T, repo usercontract.Store, email, password string) *userdomain.User {
	t.Helper()
	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	now := time.Now()
	user := &userdomain.User{
		Email:           email,
		PasswordHash:    string(hash),
		DisplayName:     email,
		Status:          constants.UserStatusActive,
		EmailVerifiedAt: &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := repo.Create(user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return user
}

func TestUserLoginStep1WithoutTOTPReturnsToken(t *testing.T) {
	authSvc, _, repo, _ := newUser2FATestServices(t)
	createActiveUser(t, repo, "no2fa@example.com", "secret123")

	res, err := authSvc.LoginStep1("no2fa@example.com", "secret123", false)
	if err != nil {
		t.Fatalf("login step1: %v", err)
	}
	if res.RequiresTOTP {
		t.Fatalf("expected RequiresTOTP=false")
	}
	if res.Token == "" {
		t.Fatalf("expected jwt token")
	}
	if res.ChallengeToken != "" {
		t.Fatalf("did not expect challenge token, got %q", res.ChallengeToken)
	}
}

func TestUserLoginStep1WithTOTPReturnsChallenge(t *testing.T) {
	authSvc, totpSvc, repo, _ := newUser2FATestServices(t)
	user := createActiveUser(t, repo, "twofa@example.com", "secret123")
	setupRes, _ := totpSvc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := totpSvc.Enable(user.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}

	res, err := authSvc.LoginStep1("twofa@example.com", "secret123", true)
	if err != nil {
		t.Fatalf("login step1: %v", err)
	}
	if !res.RequiresTOTP {
		t.Fatalf("expected RequiresTOTP=true")
	}
	if res.Token != "" {
		t.Fatalf("did not expect access token in challenge phase, got %q", res.Token)
	}
	if res.ChallengeToken == "" || res.ChallengeJTI == "" {
		t.Fatalf("expected challenge token + jti")
	}

	claims, err := authSvc.ParseUserChallengeToken(res.ChallengeToken)
	if err != nil {
		t.Fatalf("parse challenge: %v", err)
	}
	if claims.UserID != user.ID {
		t.Fatalf("user id mismatch: %d vs %d", claims.UserID, user.ID)
	}
	if claims.Purpose != challenge.PurposeTwoFactor {
		t.Fatalf("purpose mismatch: %s", claims.Purpose)
	}
	if claims.Typ != jwttoken.TypeTwoFactorChallenge {
		t.Fatalf("typ mismatch: %s", claims.Typ)
	}
	if !claims.RememberMe {
		t.Fatalf("expected remember_me=true to flow into challenge claims")
	}
}

func TestUserCompleteLoginAfter2FAIssuesAccessToken(t *testing.T) {
	authSvc, totpSvc, repo, _ := newUser2FATestServices(t)
	user := createActiveUser(t, repo, "complete@example.com", "secret123")
	setupRes, _ := totpSvc.Setup(user.ID)
	code, _ := totp.GenerateCode(setupRes.Secret, time.Now())
	if _, err := totpSvc.Enable(user.ID, code); err != nil {
		t.Fatalf("enable: %v", err)
	}

	res, err := authSvc.CompleteLoginAfter2FA(user.ID, false)
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if res.Token == "" {
		t.Fatalf("expected access token")
	}
	parsed, err := authSvc.ParseUserJWT(res.Token)
	if err != nil {
		t.Fatalf("parse jwt: %v", err)
	}
	if parsed.UserID != user.ID {
		t.Fatalf("user id mismatch")
	}
	if parsed.Typ != jwttoken.TypeAccess {
		t.Fatalf("expected typ=access, got %q", parsed.Typ)
	}
}

func TestUserParseChallengeRejectsAccessToken(t *testing.T) {
	authSvc, _, repo, _ := newUser2FATestServices(t)
	user := createActiveUser(t, repo, "swap@example.com", "secret123")

	access, _, err := authSvc.GenerateUserJWT(user, 1)
	if err != nil {
		t.Fatalf("gen jwt: %v", err)
	}
	if _, err := authSvc.ParseUserChallengeToken(access); err == nil {
		t.Fatalf("expected error parsing access token as challenge token")
	}
}

func TestUserParseUserJWTRejectsChallengeToken(t *testing.T) {
	// 验证挑战 token 即便签名通过、字段名重叠，typ 也已显式标记为 2fa_challenge，
	// 不能再被当作访问 token；中间件会进一步基于 typ 拒绝。
	authSvc, _, repo, _ := newUser2FATestServices(t)
	user := createActiveUser(t, repo, "sniff@example.com", "secret123")
	challenge, _, _, err := authSvc.IssueUserChallengeToken(user.ID, false)
	if err != nil {
		t.Fatalf("issue challenge: %v", err)
	}
	parsed, err := authSvc.ParseUserJWT(challenge)
	if err != nil {
		// 部分 jwt 库会因签名通过 + claims 兼容而成功解析；这里允许两种结果
		// 但若解析成功，typ 必须是 2fa_challenge 而非 access
		return
	}
	if parsed.Typ == jwttoken.TypeAccess {
		t.Fatalf("challenge token must not present as access token; typ=%q", parsed.Typ)
	}
	if parsed.Typ != jwttoken.TypeTwoFactorChallenge {
		t.Fatalf("expected typ=2fa_challenge after parsing challenge token, got %q", parsed.Typ)
	}
}

func TestUserLoginStep1RejectsInvalidCredentials(t *testing.T) {
	authSvc, _, repo, _ := newUser2FATestServices(t)
	createActiveUser(t, repo, "wrong@example.com", "secret123")
	if _, err := authSvc.LoginStep1("wrong@example.com", "bad", false); err != userauthapp.ErrInvalidCredentials {
		t.Fatalf("expected invalid creds, got %v", err)
	}
	if _, err := authSvc.LoginStep1("none@example.com", "x", false); err != userauthapp.ErrInvalidCredentials {
		t.Fatalf("expected invalid creds for missing user, got %v", err)
	}
}
