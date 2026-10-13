package smtp

import (
	"strings"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/shared/mailbrand"
)

func TestVerificationMailUsesConfiguredValidity(t *testing.T) {
	service := New(&config.EmailConfig{VerifyCode: config.VerifyCodeConfig{ExpireMinutes: 7}})
	for _, tc := range []struct{ locale, validity, retry string }{
		{i18n.LocaleZH, "有效期：7 分钟", "过期后请重新获取"},
		{i18n.LocaleTW, "有效期：7 分鐘", "逾期後請重新取得"},
		{i18n.LocaleEN, "expires 7 minutes after it is generated", "Request a new code"},
	} {
		t.Run(tc.locale, func(t *testing.T) {
			subject, body := service.buildVerifyCodeContent("123456", "register", tc.locale, mailbrand.Brand{
				SiteName: "π星卡社", SiteURL: "https://www.qzweb.cloud",
			})
			for _, want := range []string{"123456", tc.validity, tc.retry, "https://www.qzweb.cloud"} {
				if !strings.Contains(body, want) {
					t.Fatalf("mail missing %q: %s", want, body)
				}
			}
			if !strings.Contains(subject, "π星卡社") {
				t.Fatalf("brand missing from subject: %s", subject)
			}
		})
	}
}

func TestVerificationMailDefaultValidityMatchesVerificationPolicy(t *testing.T) {
	for _, cfg := range []*config.EmailConfig{nil, {}, {VerifyCode: config.VerifyCodeConfig{ExpireMinutes: -1}}} {
		_, body := New(cfg).buildVerifyCodeContent("123456", "register", i18n.LocaleZH)
		if !strings.Contains(body, "有效期：10 分钟") {
			t.Fatalf("unexpected default validity: %s", body)
		}
	}
}

func TestVerificationMailReflectsUpdatedExpirySetting(t *testing.T) {
	service := New(&config.EmailConfig{VerifyCode: config.VerifyCodeConfig{ExpireMinutes: 7}})
	service.SetConfig(&config.EmailConfig{VerifyCode: config.VerifyCodeConfig{ExpireMinutes: 20}})
	_, body := service.buildVerifyCodeContent("123456", "reset", i18n.LocaleZH)
	if !strings.Contains(body, "有效期：20 分钟") || strings.Contains(body, "有效期：7 分钟") {
		t.Fatalf("mail used stale expiration configuration: %s", body)
	}
}
