package userauthhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type verifySettingsStub struct{}

func (verifySettingsStub) GetEmailVerificationEnabled(bool) (bool, error) { return true, nil }
func (verifySettingsStub) GetRegistrationEnabled(bool) (bool, error)      { return true, nil }

type verifyAuthStub struct {
	calls int
	err   error
}

func (s *verifyAuthStub) SendVerifyCode(context.Context, string, string, string) error {
	s.calls++
	return s.err
}

func TestAnonymousPurposeWhitelistRunsBeforeService(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, purpose := range []string{"change_email_old", "change_email_new", "telegram_bind"} {
		auth := &verifyAuthStub{}
		router := gin.New()
		router.POST("/code", NewUserVerifyHandler(verifySettingsStub{}, nil, auth).SendUserVerifyCode)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/code", strings.NewReader(`{"email":"buyer@example.com","purpose":"`+purpose+`"}`)))
		if auth.calls != 0 || strings.Contains(rec.Body.String(), `"sent":true`) {
			t.Fatalf("purpose %s reached sender", purpose)
		}
	}
}

func TestSendProtectionUsesHTTPStatusAndRetryAfter(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{ErrVerifyCodeTooFrequent, 429}, {ErrVerificationUnavailable, 503}} {
		auth := &verifyAuthStub{err: tc.err}
		router := gin.New()
		router.POST("/code", NewUserVerifyHandler(verifySettingsStub{}, nil, auth).SendUserVerifyCode)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/code", strings.NewReader(`{"email":"buyer@example.com","purpose":"register"}`)))
		if rec.Code != tc.status || rec.Header().Get("Retry-After") == "" {
			t.Fatalf("status=%d retry=%q", rec.Code, rec.Header().Get("Retry-After"))
		}
	}
}
