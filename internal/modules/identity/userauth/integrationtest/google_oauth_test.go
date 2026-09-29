package integrationtest

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	externalidentitydomain "github.com/dujiao-next/internal/modules/identity/externalidentity/domain"
	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
	telegramauthapp "github.com/dujiao-next/internal/modules/identity/telegramauth/application"
	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type competingGoogleRegistrationUnitOfWork struct {
	db   *gorm.DB
	once sync.Once
	err  error
}

func (u *competingGoogleRegistrationUnitOfWork) WithinTransaction(
	_ context.Context,
	_ func(userauthapp.AuthTransaction) error,
) error {
	u.once.Do(func() {
		now := time.Now()
		user := &userdomain.User{
			Email: "postgres-email-race@gmail.com", PasswordHash: "external-only",
			PasswordSetupRequired: true, DisplayName: "Winner", Status: constants.UserStatusActive,
			EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
		}
		if err := u.db.Create(user).Error; err != nil {
			u.err = err
			return
		}
		u.err = u.db.Create(&externalidentitydomain.Identity{
			UserID: user.ID, Provider: constants.UserOAuthProviderGoogle,
			ProviderUserID: "google-postgres-winner", Username: user.Email,
			CreatedAt: now, UpdatedAt: now,
		}).Error
	})
	if u.err != nil {
		return u.err
	}
	return errors.New("duplicate key value violates unique constraint users_email_key (SQLSTATE 23505)")
}

func verifiedGoogleIdentity(email, sub string, authoritative bool) *googleauthapp.VerifiedIdentity {
	return &googleauthapp.VerifiedIdentity{
		Sub:                sub,
		Email:              email,
		Name:               "Google Buyer",
		Picture:            "https://example.com/google-avatar.png",
		EmailAuthoritative: authoritative,
		AuthAt:             time.Now(),
	}
}

func TestLoginVerifiedGoogleCreatesVerifiedUserAndIdentity(t *testing.T) {
	svc, _, db := setupTelegramOAuthTestService(t)

	res, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("buyer@gmail.com", "google-new-1", true))
	if err != nil {
		t.Fatalf("login verified Google: %v", err)
	}
	if res == nil || res.User == nil || res.Token == "" {
		t.Fatalf("expected completed login, got %+v", res)
	}
	if res.User.Email != "buyer@gmail.com" || res.User.EmailVerifiedAt == nil {
		t.Fatalf("new Google user email not verified: %+v", res.User)
	}
	if !res.User.PasswordSetupRequired {
		t.Fatalf("new Google user must require local password setup")
	}
	if res.User.DisplayName != "Google Buyer" {
		t.Fatalf("display name = %q", res.User.DisplayName)
	}

	var identity externalidentitydomain.Identity
	if err := db.Where("provider = ? AND provider_user_id = ?", constants.UserOAuthProviderGoogle, "google-new-1").First(&identity).Error; err != nil {
		t.Fatalf("load Google identity: %v", err)
	}
	if identity.UserID != res.User.ID || identity.Username != "buyer@gmail.com" {
		t.Fatalf("unexpected Google identity: %+v", identity)
	}
}

func TestLoginVerifiedGoogleRespectsRegistrationAndEmailDomainSettings(t *testing.T) {
	t.Run("registration disabled", func(t *testing.T) {
		svc, settings, _ := setupTelegramOAuthTestService(t)
		if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
			constants.SettingFieldRegistrationEnabled: false,
		}); err != nil {
			t.Fatalf("disable registration: %v", err)
		}
		_, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("new@gmail.com", "google-disabled", true))
		if !errors.Is(err, userauthapp.ErrRegistrationDisabled) {
			t.Fatalf("error = %v, want registration disabled", err)
		}
	})

	t.Run("email domain allowlist", func(t *testing.T) {
		svc, settings, _ := setupTelegramOAuthTestService(t)
		if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
			constants.SettingFieldRegistrationEnabled:         true,
			constants.SettingFieldEmailDomainAllowlistEnabled: true,
			constants.SettingFieldAllowedEmailDomains:         []interface{}{"qq.com"},
		}); err != nil {
			t.Fatalf("set email allowlist: %v", err)
		}
		_, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("new@gmail.com", "google-domain", true))
		if !errors.Is(err, settingsapp.ErrEmailDomainNotAllowed) {
			t.Fatalf("error = %v, want email domain not allowed", err)
		}
	})

	t.Run("existing binding remains usable when registration is disabled", func(t *testing.T) {
		svc, settings, db := setupTelegramOAuthTestService(t)
		now := time.Now()
		user := &userdomain.User{
			Email: "bound@example.com", PasswordHash: "external-login-only", PasswordSetupRequired: true,
			DisplayName: "Bound", Status: constants.UserStatusActive, EmailVerifiedAt: &now,
			CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
		if err := db.Create(&externalidentitydomain.Identity{
			UserID: user.ID, Provider: constants.UserOAuthProviderGoogle,
			ProviderUserID: "google-bound-disabled-registration", Username: user.Email,
			CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create identity: %v", err)
		}
		if _, err := settings.Update(constants.SettingKeyRegistrationConfig, map[string]interface{}{
			constants.SettingFieldRegistrationEnabled: false,
		}); err != nil {
			t.Fatalf("disable registration: %v", err)
		}

		result, err := svc.LoginVerifiedGoogle(
			verifiedGoogleIdentity(user.Email, "google-bound-disabled-registration", false),
		)
		if err != nil {
			t.Fatalf("login existing binding: %v", err)
		}
		if result.User.ID != user.ID || result.Token == "" {
			t.Fatalf("unexpected login result: %+v", result)
		}
	})
}

func TestLoginVerifiedGoogleAutoLinksOnlyAuthoritativeEmail(t *testing.T) {
	t.Run("gmail links existing user", func(t *testing.T) {
		svc, _, db := setupTelegramOAuthTestService(t)
		now := time.Now()
		user := &userdomain.User{
			Email:        "existing@gmail.com",
			PasswordHash: "$2a$10$existingPasswordHashForGoogleTests000000000000000000000",
			DisplayName:  "Existing",
			Status:       constants.UserStatusActive,
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create existing user: %v", err)
		}

		res, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("existing@gmail.com", "google-existing", true))
		if err != nil {
			t.Fatalf("login authoritative identity: %v", err)
		}
		if res.User.ID != user.ID {
			t.Fatalf("linked user = %d, want %d", res.User.ID, user.ID)
		}
		if res.User.EmailVerifiedAt == nil {
			t.Fatalf("authoritative Google link must verify the matching local email")
		}
		var users int64
		if err := db.Model(&userdomain.User{}).Count(&users).Error; err != nil {
			t.Fatalf("count users: %v", err)
		}
		if users != 1 {
			t.Fatalf("users = %d, want 1", users)
		}
	})

	t.Run("non authoritative custom email requires explicit bind", func(t *testing.T) {
		svc, _, db := setupTelegramOAuthTestService(t)
		now := time.Now()
		user := &userdomain.User{
			Email:           "owner@example.com",
			PasswordHash:    "$2a$10$existingPasswordHashForGoogleTests000000000000000000000",
			DisplayName:     "Owner",
			Status:          constants.UserStatusActive,
			EmailVerifiedAt: &now,
			CreatedAt:       now,
			UpdatedAt:       now,
		}
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create existing user: %v", err)
		}

		_, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("owner@example.com", "google-custom", false))
		if !errors.Is(err, userauthapp.ErrGoogleAutoLinkForbidden) {
			t.Fatalf("error = %v, want explicit-bind requirement", err)
		}
		var identities int64
		if err := db.Model(&externalidentitydomain.Identity{}).Count(&identities).Error; err != nil {
			t.Fatalf("count identities: %v", err)
		}
		if identities != 0 {
			t.Fatalf("identities = %d, want 0", identities)
		}
	})

	t.Run("non authoritative custom email cannot create a new account", func(t *testing.T) {
		svc, _, db := setupTelegramOAuthTestService(t)
		_, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("new-owner@example.com", "google-new-custom", false))
		if !errors.Is(err, userauthapp.ErrGoogleAutoLinkForbidden) {
			t.Fatalf("error = %v, want explicit-bind requirement", err)
		}
		var users int64
		if err := db.Model(&userdomain.User{}).Count(&users).Error; err != nil {
			t.Fatalf("count users: %v", err)
		}
		var identities int64
		if err := db.Model(&externalidentitydomain.Identity{}).Count(&identities).Error; err != nil {
			t.Fatalf("count identities: %v", err)
		}
		if users != 0 || identities != 0 {
			t.Fatalf("custom-domain auto login persisted users=%d identities=%d", users, identities)
		}
	})
}

func TestLoginVerifiedGoogleRollsBackUserWhenIdentityCreateFails(t *testing.T) {
	svc, _, db := setupTelegramOAuthTestService(t)
	injected := errors.New("injected Google identity create failure")
	callbackName := "test:fail_google_identity_create"
	if err := db.Callback().Create().Before("gorm:create").Register(callbackName, func(tx *gorm.DB) {
		if tx.Statement != nil && tx.Statement.Table == "user_oauth_identities" {
			tx.AddError(injected)
		}
	}); err != nil {
		t.Fatalf("register failure callback: %v", err)
	}

	_, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("atomic@gmail.com", "google-atomic", true))
	if !errors.Is(err, injected) {
		t.Fatalf("error = %v, want injected identity failure", err)
	}
	var users int64
	if err := db.Model(&userdomain.User{}).Where("email = ?", "atomic@gmail.com").Count(&users).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	var identities int64
	if err := db.Model(&externalidentitydomain.Identity{}).
		Where("provider = ? AND provider_user_id = ?", constants.UserOAuthProviderGoogle, "google-atomic").
		Count(&identities).Error; err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if users != 0 || identities != 0 {
		t.Fatalf("failed atomic login persisted users=%d identities=%d", users, identities)
	}

	if err := db.Callback().Create().Remove(callbackName); err != nil {
		t.Fatalf("remove failure callback: %v", err)
	}
	result, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("atomic@gmail.com", "google-atomic", true))
	if err != nil {
		t.Fatalf("retry after transient identity failure: %v", err)
	}
	if result == nil || result.User == nil || result.Token == "" {
		t.Fatalf("unexpected retry result: %+v", result)
	}
}

func TestBindVerifiedGoogleConcurrentDifferentSubjectsCreatesOneBinding(t *testing.T) {
	svc, _, db := setupTelegramOAuthTestService(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("local-password"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	now := time.Now()
	user := &userdomain.User{
		Email: "concurrent-bind@example.com", PasswordHash: string(passwordHash), DisplayName: "Concurrent",
		Status: constants.UserStatusActive, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for _, sub := range []string{"google-concurrent-a", "google-concurrent-b"} {
		sub := sub
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			_, bindErr := svc.BindVerifiedGoogle(
				user.ID,
				verifiedGoogleIdentity(sub+"@gmail.com", sub, true),
			)
			results <- bindErr
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)

	successes := 0
	conflicts := 0
	for resultErr := range results {
		switch {
		case resultErr == nil:
			successes++
		case errors.Is(resultErr, userauthapp.ErrUserOAuthAlreadyBound):
			conflicts++
		default:
			t.Fatalf("unexpected bind error: %v", resultErr)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("results successes=%d conflicts=%d, want 1/1", successes, conflicts)
	}
	var count int64
	if err := db.Model(&externalidentitydomain.Identity{}).
		Where("user_id = ? AND provider = ?", user.ID, constants.UserOAuthProviderGoogle).
		Count(&count).Error; err != nil {
		t.Fatalf("count bindings: %v", err)
	}
	if count != 1 {
		t.Fatalf("bindings = %d, want 1", count)
	}
}

func TestConcurrentFirstGoogleLoginConvergesWithoutOrphanAccounts(t *testing.T) {
	t.Run("same subject and email are idempotent", func(t *testing.T) {
		svc, _, db := setupTelegramOAuthTestService(t)
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("get sql db: %v", err)
		}
		sqlDB.SetMaxOpenConns(2)

		start := make(chan struct{})
		results := make(chan error, 2)
		var waitGroup sync.WaitGroup
		for index := 0; index < 2; index++ {
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				<-start
				_, loginErr := svc.LoginVerifiedGoogle(
					verifiedGoogleIdentity("same-concurrent@gmail.com", "google-same-concurrent", true),
				)
				results <- loginErr
			}()
		}
		close(start)
		waitGroup.Wait()
		close(results)
		for resultErr := range results {
			if resultErr != nil {
				t.Fatalf("concurrent idempotent login error: %v", resultErr)
			}
		}
		assertGoogleLoginRowCounts(t, db, "same-concurrent@gmail.com", 1, 1)
	})

	t.Run("same email with different subjects keeps one binding", func(t *testing.T) {
		svc, _, db := setupTelegramOAuthTestService(t)
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatalf("get sql db: %v", err)
		}
		sqlDB.SetMaxOpenConns(2)

		start := make(chan struct{})
		results := make(chan error, 2)
		var waitGroup sync.WaitGroup
		for _, subject := range []string{"google-email-race-a", "google-email-race-b"} {
			subject := subject
			waitGroup.Add(1)
			go func() {
				defer waitGroup.Done()
				<-start
				_, loginErr := svc.LoginVerifiedGoogle(
					verifiedGoogleIdentity("email-race@gmail.com", subject, true),
				)
				results <- loginErr
			}()
		}
		close(start)
		waitGroup.Wait()
		close(results)
		successes := 0
		conflicts := 0
		for resultErr := range results {
			switch {
			case resultErr == nil:
				successes++
			case errors.Is(resultErr, userauthapp.ErrUserOAuthAlreadyBound):
				conflicts++
			default:
				t.Fatalf("unexpected concurrent email login error: %v", resultErr)
			}
		}
		if successes != 1 || conflicts != 1 {
			t.Fatalf("results successes=%d conflicts=%d, want 1/1", successes, conflicts)
		}
		assertGoogleLoginRowCounts(t, db, "email-race@gmail.com", 1, 1)
	})
}

func TestGoogleLoginMapsConcurrentEmailUniqueRaceToAlreadyBound(t *testing.T) {
	svc, _, db := setupTelegramOAuthTestService(t)
	svc.SetAuthUnitOfWork(&competingGoogleRegistrationUnitOfWork{db: db})

	_, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity(
		"postgres-email-race@gmail.com",
		"google-postgres-loser",
		true,
	))
	if !errors.Is(err, userauthapp.ErrUserOAuthAlreadyBound) {
		t.Fatalf("error = %v, want stable already-bound conflict", err)
	}
	assertGoogleLoginRowCounts(t, db, "postgres-email-race@gmail.com", 1, 1)
}

func assertGoogleLoginRowCounts(t *testing.T, db *gorm.DB, email string, wantUsers, wantIdentities int64) {
	t.Helper()
	var users int64
	if err := db.Model(&userdomain.User{}).Where("email = ?", email).Count(&users).Error; err != nil {
		t.Fatalf("count users: %v", err)
	}
	var identities int64
	if err := db.Model(&externalidentitydomain.Identity{}).
		Joins("JOIN users ON users.id = user_oauth_identities.user_id").
		Where("users.email = ? AND user_oauth_identities.provider = ?", email, constants.UserOAuthProviderGoogle).
		Count(&identities).Error; err != nil {
		t.Fatalf("count identities: %v", err)
	}
	if users != wantUsers || identities != wantIdentities {
		t.Fatalf(
			"row counts users=%d identities=%d, want users=%d identities=%d",
			users,
			identities,
			wantUsers,
			wantIdentities,
		)
	}
}

func TestGoogleAndTelegramCannotBothBeUnboundWithoutLocalPassword(t *testing.T) {
	telegramService := telegramauthapp.NewService(config.TelegramAuthConfig{
		Enabled:     true,
		BotUsername: "usable_login_bot",
		BotToken:    "usable-login-token",
	})

	for _, firstProvider := range []string{
		constants.UserOAuthProviderGoogle,
		constants.UserOAuthProviderTelegram,
	} {
		t.Run(firstProvider+" first", func(t *testing.T) {
			svc, _, db := setupTelegramOAuthTestService(t, telegramService)
			login, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity(
				firstProvider+"-order@gmail.com",
				"google-"+firstProvider+"-order",
				true,
			))
			if err != nil {
				t.Fatalf("create Google account: %v", err)
			}
			now := time.Now()
			if err := db.Create(&externalidentitydomain.Identity{
				UserID: login.User.ID, Provider: constants.UserOAuthProviderTelegram,
				ProviderUserID: "telegram-" + firstProvider + "-order", Username: "recovery",
				CreatedAt: now, UpdatedAt: now,
			}).Error; err != nil {
				t.Fatalf("create Telegram identity: %v", err)
			}

			googleBinding, err := svc.GetGoogleBinding(login.User.ID)
			if err != nil || !googleBinding.CanUnbind {
				t.Fatalf("Google binding should initially be removable: binding=%+v err=%v", googleBinding, err)
			}
			telegramBinding, err := svc.GetTelegramBinding(login.User.ID)
			if err != nil || !telegramBinding.CanUnbind {
				t.Fatalf("Telegram binding should initially be removable: binding=%+v err=%v", telegramBinding, err)
			}

			if firstProvider == constants.UserOAuthProviderGoogle {
				if err := svc.UnbindGoogle(login.User.ID); err != nil {
					t.Fatalf("unbind Google first: %v", err)
				}
				if err := svc.UnbindTelegram(login.User.ID); !errors.Is(err, userauthapp.ErrTelegramUnbindRequiresEmail) {
					t.Fatalf("second Telegram unbind error = %v, want locked", err)
				}
			} else {
				if err := svc.UnbindTelegram(login.User.ID); err != nil {
					t.Fatalf("unbind Telegram first: %v", err)
				}
				if err := svc.UnbindGoogle(login.User.ID); !errors.Is(err, userauthapp.ErrGoogleUnbindLocked) {
					t.Fatalf("second Google unbind error = %v, want locked", err)
				}
			}

			var remaining int64
			if err := db.Model(&externalidentitydomain.Identity{}).
				Where("user_id = ?", login.User.ID).
				Count(&remaining).Error; err != nil {
				t.Fatalf("count remaining identities: %v", err)
			}
			if remaining != 1 {
				t.Fatalf("remaining identities = %d, want 1", remaining)
			}
		})
	}
}

func TestConcurrentGoogleAndTelegramUnbindLeavesOneLoginMethod(t *testing.T) {
	telegramService := telegramauthapp.NewService(config.TelegramAuthConfig{
		Enabled:     true,
		BotUsername: "usable_login_bot",
		BotToken:    "usable-login-token",
	})
	svc, _, db := setupTelegramOAuthTestService(t, telegramService)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	login, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity(
		"concurrent-unbind@gmail.com",
		"google-concurrent-unbind",
		true,
	))
	if err != nil {
		t.Fatalf("create Google account: %v", err)
	}
	now := time.Now()
	if err := db.Create(&externalidentitydomain.Identity{
		UserID: login.User.ID, Provider: constants.UserOAuthProviderTelegram,
		ProviderUserID: "telegram-concurrent-unbind", Username: "recovery",
		CreatedAt: now, UpdatedAt: now,
	}).Error; err != nil {
		t.Fatalf("create Telegram identity: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	var waitGroup sync.WaitGroup
	for _, unbind := range []func(uint) error{svc.UnbindGoogle, svc.UnbindTelegram} {
		unbind := unbind
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			<-start
			results <- unbind(login.User.ID)
		}()
	}
	close(start)
	waitGroup.Wait()
	close(results)

	successes := 0
	locked := 0
	for resultErr := range results {
		switch {
		case resultErr == nil:
			successes++
		case errors.Is(resultErr, userauthapp.ErrGoogleUnbindLocked),
			errors.Is(resultErr, userauthapp.ErrTelegramUnbindRequiresEmail):
			locked++
		default:
			t.Fatalf("unexpected unbind error: %v", resultErr)
		}
	}
	if successes != 1 || locked != 1 {
		t.Fatalf("results successes=%d locked=%d, want 1/1", successes, locked)
	}
	var remaining int64
	if err := db.Model(&externalidentitydomain.Identity{}).
		Where("user_id = ?", login.User.ID).
		Count(&remaining).Error; err != nil {
		t.Fatalf("count remaining identities: %v", err)
	}
	if remaining != 1 {
		t.Fatalf("remaining identities = %d, want 1", remaining)
	}
}

func TestLoginVerifiedGoogleReturnsTwoFactorChallenge(t *testing.T) {
	svc, _, db := setupTelegramOAuthTestService(t)
	now := time.Now()
	user := &userdomain.User{
		Email:           "twofactor@gmail.com",
		PasswordHash:    "$2a$10$existingPasswordHashForGoogleTests000000000000000000000",
		DisplayName:     "2FA",
		Status:          constants.UserStatusActive,
		EmailVerifiedAt: &now,
		TOTPEnabledAt:   &now,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("create 2FA user: %v", err)
	}

	res, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("twofactor@gmail.com", "google-2fa", true))
	if err != nil {
		t.Fatalf("login Google 2FA user: %v", err)
	}
	if !res.RequiresTOTP || res.ChallengeToken == "" || res.Token != "" {
		t.Fatalf("expected 2FA challenge only, got %+v", res)
	}
	claims, err := svc.ParseUserChallengeToken(res.ChallengeToken)
	if err != nil {
		t.Fatalf("parse challenge: %v", err)
	}
	if claims.LoginSource != constants.LoginLogSourceGoogle {
		t.Fatalf("challenge login source = %q", claims.LoginSource)
	}
}

func TestBindVerifiedGoogleConflictsAndUnbindSafety(t *testing.T) {
	t.Run("usable local password permits unbind", func(t *testing.T) {
		svc, _, db := setupTelegramOAuthTestService(t)
		passwordHash, err := bcrypt.GenerateFromPassword([]byte("local-password"), bcrypt.DefaultCost)
		if err != nil {
			t.Fatalf("hash password: %v", err)
		}
		now := time.Now()
		user := &userdomain.User{
			Email: "local@example.com", PasswordHash: string(passwordHash), DisplayName: "Local",
			Status: constants.UserStatusActive, EmailVerifiedAt: &now, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create local user: %v", err)
		}
		binding, err := svc.BindVerifiedGoogle(user.ID, verifiedGoogleIdentity("google@gmail.com", "google-local", true))
		if err != nil {
			t.Fatalf("bind Google: %v", err)
		}
		if !binding.CanUnbind {
			t.Fatalf("usable local password should permit Google unbind")
		}
		if err := svc.UnbindGoogle(user.ID); err != nil {
			t.Fatalf("unbind Google: %v", err)
		}
	})

	t.Run("provider identity belongs to another user", func(t *testing.T) {
		svc, _, db := setupTelegramOAuthTestService(t)
		now := time.Now()
		first := &userdomain.User{
			Email: "first@example.com", PasswordHash: "usable", DisplayName: "First",
			Status: constants.UserStatusActive, CreatedAt: now, UpdatedAt: now,
		}
		second := &userdomain.User{
			Email: "second@example.com", PasswordHash: "usable", DisplayName: "Second",
			Status: constants.UserStatusActive, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(first).Error; err != nil {
			t.Fatalf("create first user: %v", err)
		}
		if err := db.Create(second).Error; err != nil {
			t.Fatalf("create second user: %v", err)
		}
		if err := db.Create(&externalidentitydomain.Identity{
			UserID: second.ID, Provider: constants.UserOAuthProviderGoogle,
			ProviderUserID: "google-occupied", Username: "second@gmail.com",
			CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create occupied identity: %v", err)
		}

		_, err := svc.BindVerifiedGoogle(first.ID, verifiedGoogleIdentity("first@gmail.com", "google-occupied", true))
		if !errors.Is(err, userauthapp.ErrUserOAuthIdentityExists) {
			t.Fatalf("error = %v, want occupied conflict", err)
		}
	})

	t.Run("current user already has another Google identity", func(t *testing.T) {
		svc, _, db := setupTelegramOAuthTestService(t)
		now := time.Now()
		user := &userdomain.User{
			Email: "already-bound@example.com", PasswordHash: "usable", DisplayName: "Bound",
			Status: constants.UserStatusActive, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(user).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
		if err := db.Create(&externalidentitydomain.Identity{
			UserID: user.ID, Provider: constants.UserOAuthProviderGoogle,
			ProviderUserID: "google-current", Username: "current@gmail.com",
			CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create current identity: %v", err)
		}

		_, err := svc.BindVerifiedGoogle(user.ID, verifiedGoogleIdentity("other@gmail.com", "google-other", true))
		if !errors.Is(err, userauthapp.ErrUserOAuthAlreadyBound) {
			t.Fatalf("error = %v, want already-bound conflict", err)
		}
	})

	t.Run("Google-created account cannot lose its only login method", func(t *testing.T) {
		telegramService := telegramauthapp.NewService(config.TelegramAuthConfig{
			Enabled:     true,
			BotUsername: "usable_login_bot",
			BotToken:    "usable-login-token",
		})
		svc, _, db := setupTelegramOAuthTestService(t, telegramService)
		login, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("locked@gmail.com", "google-locked", true))
		if err != nil {
			t.Fatalf("create Google user: %v", err)
		}
		binding, err := svc.GetGoogleBinding(login.User.ID)
		if err != nil {
			t.Fatalf("get Google binding: %v", err)
		}
		if binding.CanUnbind {
			t.Fatalf("new Google-only account must not be allowed to unbind")
		}
		if err := svc.UnbindGoogle(login.User.ID); !errors.Is(err, userauthapp.ErrGoogleUnbindLocked) {
			t.Fatalf("unbind error = %v, want locked", err)
		}

		now := time.Now()
		if err := db.Create(&externalidentitydomain.Identity{
			UserID: login.User.ID, Provider: constants.UserOAuthProviderTelegram,
			ProviderUserID: "telegram-recovery", Username: "recovery",
			CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create recovery identity: %v", err)
		}
		binding, err = svc.GetGoogleBinding(login.User.ID)
		if err != nil {
			t.Fatalf("get recoverable binding: %v", err)
		}
		if !binding.CanUnbind {
			t.Fatalf("another external identity should allow unbind")
		}
		if err := svc.UnbindGoogle(login.User.ID); err != nil {
			t.Fatalf("unbind recoverable Google identity: %v", err)
		}
	})

	t.Run("disabled or unknown providers are not recovery methods", func(t *testing.T) {
		telegramService := telegramauthapp.NewService(config.TelegramAuthConfig{
			Enabled:     false,
			BotUsername: "disabled_bot",
			BotToken:    "disabled-token",
		})
		svc, _, db := setupTelegramOAuthTestService(t, telegramService)
		login, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("failclosed@gmail.com", "google-failclosed", true))
		if err != nil {
			t.Fatalf("create Google user: %v", err)
		}
		now := time.Now()
		for _, identity := range []*externalidentitydomain.Identity{
			{
				UserID: login.User.ID, Provider: constants.UserOAuthProviderTelegram,
				ProviderUserID: "disabled-telegram", Username: "disabled",
				CreatedAt: now, UpdatedAt: now,
			},
			{
				UserID: login.User.ID, Provider: "future-provider",
				ProviderUserID: "unknown-provider-id", Username: "unknown",
				CreatedAt: now, UpdatedAt: now,
			},
		} {
			if err := db.Create(identity).Error; err != nil {
				t.Fatalf("create alternate identity: %v", err)
			}
		}
		binding, err := svc.GetGoogleBinding(login.User.ID)
		if err != nil {
			t.Fatalf("get binding: %v", err)
		}
		if binding.CanUnbind {
			t.Fatalf("disabled and unknown providers must fail closed")
		}
		if err := svc.UnbindGoogle(login.User.ID); !errors.Is(err, userauthapp.ErrGoogleUnbindLocked) {
			t.Fatalf("unbind error = %v, want locked", err)
		}
	})

	t.Run("Telegram OIDC without a public bot username is not a recovery method", func(t *testing.T) {
		telegramService := telegramauthapp.NewService(config.TelegramAuthConfig{
			Enabled:         true,
			BotToken:        "123456789:test-token",
			ClientSecret:    "test-client-secret",
			OIDCRedirectURI: "https://shop.example.com/auth/telegram/callback",
		})
		svc, _, db := setupTelegramOAuthTestService(t, telegramService)
		login, err := svc.LoginVerifiedGoogle(verifiedGoogleIdentity("oidc-no-ui@gmail.com", "google-oidc-no-ui", true))
		if err != nil {
			t.Fatalf("create Google user: %v", err)
		}
		now := time.Now()
		if err := db.Create(&externalidentitydomain.Identity{
			UserID: login.User.ID, Provider: constants.UserOAuthProviderTelegram,
			ProviderUserID: "oidc-without-username", Username: "hidden",
			CreatedAt: now, UpdatedAt: now,
		}).Error; err != nil {
			t.Fatalf("create Telegram identity: %v", err)
		}
		binding, err := svc.GetGoogleBinding(login.User.ID)
		if err != nil {
			t.Fatalf("get binding: %v", err)
		}
		if binding.CanUnbind {
			t.Fatalf("OIDC without frontend-reachable bot username must fail closed")
		}
	})
}
