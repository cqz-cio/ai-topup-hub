package turnstile

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/modules/captcha/contract"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
)

func TestClientPostsSiteverifyForm(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method got %s", r.Method)
		}
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
		}
		if r.Form.Get("secret") != "secret-1" || r.Form.Get("response") != "token-1" || r.Form.Get("remoteip") != "127.0.0.1" {
			t.Errorf("unexpected form: %v", r.Form)
		}
		_, _ = w.Write([]byte(`{"success":true}`))
	}))
	t.Cleanup(server.Close)

	err := New().Verify(settingssecurity.CaptchaTurnstileSetting{
		SecretKey: "secret-1",
		VerifyURL: server.URL,
		TimeoutMS: 2000,
	}, "token-1", "127.0.0.1")
	if err != nil {
		t.Fatalf("verify failed: %v", err)
	}
}

func TestClientMapsRejectedToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"success":false,"error-codes":["invalid-input-response"]}`))
	}))
	t.Cleanup(server.Close)

	err := New().Verify(settingssecurity.CaptchaTurnstileSetting{
		SecretKey: "secret-1",
		VerifyURL: server.URL,
		TimeoutMS: 2000,
	}, "bad-token", "")
	if !errors.Is(err, contract.ErrInvalid) {
		t.Fatalf("rejected token error got %v", err)
	}
}
