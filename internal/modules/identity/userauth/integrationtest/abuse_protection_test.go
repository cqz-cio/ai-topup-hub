package integrationtest

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
	"github.com/dujiao-next/internal/platform/ratelimit"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

type guardedSender struct {
	calls atomic.Int32
	fail  bool
}

func (s *guardedSender) SendVerifyCode(_, _, _, _ string, _ mailbrand.Brand) error {
	s.calls.Add(1)
	if s.fail {
		return errors.New("mock SMTP unavailable")
	}
	return nil
}

func TestPublicSendRejectsAuthenticatedPurposes(t *testing.T) {
	sender := &guardedSender{}
	svc, _, _ := newRegistrationDomainPolicyAuthServiceWithSender(t, sender)
	for _, purpose := range []string{"change_email_old", "change_email_new", "telegram_bind", "unknown"} {
		if err := svc.SendVerifyCode(context.Background(), "buyer@example.com", purpose, "zh-CN"); !errors.Is(err, userauthapp.ErrInvalidVerifyPurpose) {
			t.Fatalf("purpose %s: %v", purpose, err)
		}
	}
	if sender.calls.Load() != 0 {
		t.Fatal("invalid purposes reached SMTP")
	}
}

func TestConcurrentVerificationSendAndSMTPFailureRemainLimited(t *testing.T) {
	for _, fail := range []bool{false, true} {
		sender := &guardedSender{fail: fail}
		svc, _, db := newRegistrationDomainPolicyAuthServiceWithSender(t, sender)
		sqlDB, err := db.DB()
		if err != nil {
			t.Fatal(err)
		}
		sqlDB.SetMaxOpenConns(1)
		var wg sync.WaitGroup
		for i := 0; i < 30; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_ = svc.SendVerifyCode(context.Background(), " Buyer@Example.com ", "register", "zh-CN")
			}()
		}
		wg.Wait()
		if sender.calls.Load() != 1 {
			t.Fatalf("fail=%v SMTP calls=%d, want 1", fail, sender.calls.Load())
		}
	}
}

func TestVerificationStoreFailurePreventsSMTP(t *testing.T) {
	sender := &guardedSender{}
	svc, _, _ := newRegistrationDomainPolicyAuthServiceWithSender(t, sender)
	svc.SetVerificationQuota(ratelimit.New(nil, true))
	err := svc.SendVerifyCode(context.Background(), "buyer@example.com", "register", "zh-CN")
	if !errors.Is(err, userauthapp.ErrVerificationUnavailable) || sender.calls.Load() != 0 {
		t.Fatalf("store outage: err=%v calls=%d", err, sender.calls.Load())
	}
}

func TestEmailNormalizationRejectsMailboxWrappers(t *testing.T) {
	for _, email := range []string{"Name <buyer@example.com>", "<buyer@example.com>", "buyer@example.com (name)"} {
		if _, err := userauthapp.NormalizeEmail(email); !errors.Is(err, userauthapp.ErrInvalidEmail) {
			t.Fatalf("accepted mailbox wrapper %q", email)
		}
	}
	got, err := userauthapp.NormalizeEmail(" Buyer@Example.com ")
	if err != nil || got != "buyer@example.com" {
		t.Fatalf("canonical email: %q %v", got, err)
	}
}

func TestRecipientCooldownIsSharedAcrossPublicAndAuthenticatedPurposes(t *testing.T) {
	sender := &guardedSender{}
	svc, _, db := newRegistrationDomainPolicyAuthServiceWithSender(t, sender)
	ctx := context.Background()
	if err := svc.SendVerifyCode(ctx, "buyer@example.com", "register", "zh-CN"); err != nil {
		t.Fatal(err)
	}
	user := &userdomain.User{Email: "buyer@example.com", PasswordHash: "unused-mock-hash", Status: "active"}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	if err := svc.SendVerifyCode(ctx, user.Email, "reset", "zh-CN"); !errors.Is(err, userauthapp.ErrVerifyCodeTooFrequent) {
		t.Fatalf("purpose rotation bypass: %v", err)
	}
	if err := svc.SendChangeEmailCode(ctx, user.ID, "old", "", "zh-CN"); !errors.Is(err, userauthapp.ErrVerifyCodeTooFrequent) {
		t.Fatalf("authenticated purpose rotation bypass: %v", err)
	}
	if err := svc.SendChangeEmailCode(ctx, user.ID, "new", "new@example.com", "zh-CN"); err != nil {
		t.Fatalf("legitimate new-email flow: %v", err)
	}
	if sender.calls.Load() != 2 {
		t.Fatalf("SMTP calls=%d want=2", sender.calls.Load())
	}
}
