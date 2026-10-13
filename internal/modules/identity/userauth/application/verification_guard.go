package application

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"

	"github.com/dujiao-next/internal/platform/ratelimit"
)

func (s *Service) SetVerificationQuota(quota ratelimit.Reserver) { s.verificationQuota = quota }

type verificationRateError struct{ retry time.Duration }

func (e *verificationRateError) Error() string { return ErrVerifyCodeTooFrequent.Error() }
func (e *verificationRateError) Unwrap() error { return ErrVerifyCodeTooFrequent }
func (e *verificationRateError) RetryAfterSeconds() int {
	return max(1, int((e.retry+time.Second-1)/time.Second))
}

func (s *Service) reserveVerificationMail(ctx context.Context, email string) error {
	if s.verificationQuota == nil {
		return ErrVerificationUnavailable
	}
	limits := s.cfg.Security.AbuseProtection.WithDefaults()
	// No recipient address is stored in Redis. A common hash tag makes the
	// complete recipient + global reservation atomic on clustered Redis too.
	namespace := strings.TrimSpace(s.cfg.Redis.Prefix)
	if namespace == "" {
		namespace = "dj"
	}
	prefix := fmt.Sprintf("{%x}:verify-mail:", sha256.Sum256([]byte(namespace)))
	recipient := fmt.Sprintf("%s%x:", prefix, sha256.Sum256([]byte(email)))
	retry, err := s.verificationQuota.Reserve(ctx, []ratelimit.Limit{
		{Key: recipient + "cooldown", Max: 1, Window: time.Duration(resolveSendIntervalSeconds(s.cfg.Email.VerifyCode)) * time.Second},
		{Key: recipient + "hour", Max: limits.RecipientPerHour, Window: time.Hour},
		{Key: recipient + "day", Max: limits.RecipientPerDay, Window: 24 * time.Hour},
		{Key: prefix + "hour", Max: limits.MailPerHour, Window: time.Hour},
		{Key: prefix + "day", Max: limits.MailPerDay, Window: 24 * time.Hour},
	})
	if err != nil {
		return ErrVerificationUnavailable
	}
	if retry > 0 {
		return &verificationRateError{retry: retry}
	}
	return nil
}
