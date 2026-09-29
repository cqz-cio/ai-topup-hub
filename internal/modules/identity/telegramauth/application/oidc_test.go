package telegramauthapp

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	"github.com/golang-jwt/jwt/v5"
)

func newTestTelegramAuthServiceOIDC(t *testing.T) *Service {
	t.Helper()
	svc := NewService(config.TelegramAuthConfig{
		Enabled:         true,
		BotUsername:     "mybot",
		BotToken:        "123456789:AAAA-bbbbcccc",
		ClientSecret:    "topsecret",
		OIDCRedirectURI: "https://shop.example.com/auth/telegram/callback",
	})
	store := map[string]string{}
	svc.oidcStateSet = func(ctx context.Context, key string, value string, ttlSeconds int) (bool, error) {
		if _, ok := store[key]; ok {
			return false, nil
		}
		store[key] = value
		return true, nil
	}
	svc.oidcStateTake = func(ctx context.Context, key string) (string, bool, error) {
		v, ok := store[key]
		if ok {
			delete(store, key)
		}
		return v, ok, nil
	}
	return svc
}

func TestStartOIDCLoginBuildsAuthURL(t *testing.T) {
	svc := newTestTelegramAuthServiceOIDC(t)
	authURL, err := svc.StartOIDCLogin(context.Background(), telegramOIDCIntentLogin, 0)
	if err != nil {
		t.Fatalf("StartOIDCLogin error: %v", err)
	}
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("bad url: %v", err)
	}
	if u.Scheme != "https" || u.Host != "oauth.telegram.org" || u.Path != "/auth" {
		t.Fatalf("unexpected endpoint: %s", authURL)
	}
	q := u.Query()
	if q.Get("client_id") != "123456789" {
		t.Fatalf("client_id = %q", q.Get("client_id"))
	}
	if q.Get("redirect_uri") != "https://shop.example.com/auth/telegram/callback" {
		t.Fatalf("redirect_uri = %q", q.Get("redirect_uri"))
	}
	if q.Get("response_type") != "code" {
		t.Fatalf("response_type = %q", q.Get("response_type"))
	}
	if q.Get("scope") != "openid profile" {
		t.Fatalf("scope = %q", q.Get("scope"))
	}
	if q.Get("code_challenge_method") != "S256" {
		t.Fatalf("code_challenge_method = %q", q.Get("code_challenge_method"))
	}
	if len(q.Get("state")) < 16 || len(q.Get("code_challenge")) < 16 {
		t.Fatalf("state/code_challenge too short")
	}
}

func TestStartOIDCLoginRejectsWidgetMode(t *testing.T) {
	svc := NewService(config.TelegramAuthConfig{Enabled: true, BotUsername: "mybot", BotToken: "1:abc"})
	if _, err := svc.StartOIDCLogin(context.Background(), telegramOIDCIntentLogin, 0); err == nil {
		t.Fatalf("expected error in widget mode")
	}
}

func TestPKCEChallengeMatchesVerifier(t *testing.T) {
	verifier, challenge, err := newPKCEPair()
	if err != nil {
		t.Fatalf("newPKCEPair: %v", err)
	}
	if len(verifier) < 43 || len(verifier) > 128 {
		t.Fatalf("verifier length %d out of range", len(verifier))
	}
	if strings.ContainsAny(challenge, "+/=") {
		t.Fatalf("challenge not base64url: %q", challenge)
	}
	if s256Challenge(verifier) != challenge {
		t.Fatalf("challenge does not match verifier")
	}
}

func TestCompleteOIDCLoginHappyPath(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	const kid = "test-kid-1"
	const clientID = "123456789"

	jwks := map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": kid,
		"n": base64.RawURLEncoding.EncodeToString(priv.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes()),
	}}}

	now := time.Now()
	claims := jwt.MapClaims{
		"iss":                "https://oauth.telegram.org",
		"aud":                clientID,
		"sub":                "1234123412341234123",
		"iat":                now.Unix(),
		"exp":                now.Add(time.Hour).Unix(),
		"id":                 float64(987654321),
		"name":               "John Doe",
		"preferred_username": "johndoe",
		"picture":            "https://cdn.telesco.pe/file/abc",
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid
	idToken, err := tok.SignedString(priv)
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/jwks":
			_ = json.NewEncoder(w).Encode(jwks)
		case "/token":
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(400)
				return
			}
			_ = r.ParseForm()
			if r.Form.Get("grant_type") != "authorization_code" || r.Form.Get("code") != "the-code" || r.Form.Get("code_verifier") == "" {
				w.WriteHeader(400)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "at", "token_type": "Bearer", "expires_in": 3600, "id_token": idToken})
		default:
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()

	svc := newTestTelegramAuthServiceOIDC(t)
	svc.oidcTokenEndpoint = srv.URL + "/token"
	svc.oidcJWKSEndpoint = srv.URL + "/jwks"
	svc.httpClient = srv.Client()
	svc.replaySetNX = func(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
		return true, nil
	}

	authURL, err := svc.StartOIDCLogin(context.TODO(), telegramOIDCIntentLogin, 0)
	if err != nil {
		t.Fatal(err)
	}
	state := mustQueryParam(t, authURL, "state")

	verified, intent, userID, err := svc.CompleteOIDCLogin(context.TODO(), "the-code", state)
	if err != nil {
		t.Fatalf("CompleteOIDCLogin: %v", err)
	}
	if intent != telegramOIDCIntentLogin || userID != 0 {
		t.Fatalf("intent/userID mismatch: %q %d", intent, userID)
	}
	if verified.Provider != constants.UserOAuthProviderTelegram {
		t.Fatalf("provider %q", verified.Provider)
	}
	if verified.ProviderUserID != "987654321" {
		t.Fatalf("provider_user_id %q", verified.ProviderUserID)
	}
	if len(verified.ProviderUserIDAliases) != 1 || verified.ProviderUserIDAliases[0] != "1234123412341234123" {
		t.Fatalf("provider_user_id aliases %#v", verified.ProviderUserIDAliases)
	}
	if verified.Username != "johndoe" {
		t.Fatalf("username %q", verified.Username)
	}
	if verified.FirstName != "John" || verified.LastName != "Doe" {
		t.Fatalf("name split: %q %q", verified.FirstName, verified.LastName)
	}
	if verified.AvatarURL != "https://cdn.telesco.pe/file/abc" {
		t.Fatalf("avatar %q", verified.AvatarURL)
	}

	if _, _, _, err := svc.CompleteOIDCLogin(context.TODO(), "the-code", state); err == nil {
		t.Fatalf("expected error on reused state")
	}
}

func TestCompleteOIDCLoginRejectsBadAudience(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	const kid = "k"
	jwks := map[string]any{"keys": []map[string]string{{
		"kty": "RSA", "alg": "RS256", "kid": kid,
		"n": base64.RawURLEncoding.EncodeToString(priv.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes()),
	}}}
	now := time.Now()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": "https://oauth.telegram.org", "aud": "999", "sub": "1", "iat": now.Unix(), "exp": now.Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	idToken, _ := tok.SignedString(priv)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks" {
			_ = json.NewEncoder(w).Encode(jwks)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"id_token": idToken})
	}))
	defer srv.Close()
	svc := newTestTelegramAuthServiceOIDC(t)
	svc.oidcTokenEndpoint, svc.oidcJWKSEndpoint, svc.httpClient = srv.URL+"/token", srv.URL+"/jwks", srv.Client()
	svc.replaySetNX = func(ctx context.Context, key string, value interface{}, ttl time.Duration) (bool, error) {
		return true, nil
	}
	authURL, _ := svc.StartOIDCLogin(context.TODO(), telegramOIDCIntentLogin, 0)
	state := mustQueryParam(t, authURL, "state")
	if _, _, _, err := svc.CompleteOIDCLogin(context.TODO(), "c", state); err == nil {
		t.Fatalf("expected aud mismatch error")
	}
}

func TestCompleteOIDCLoginUnknownState(t *testing.T) {
	svc := newTestTelegramAuthServiceOIDC(t)
	if _, _, _, err := svc.CompleteOIDCLogin(context.TODO(), "c", "no-such-state"); err == nil {
		t.Fatalf("expected state invalid error")
	}
}

func TestParseTelegramJWKS(t *testing.T) {
	priv, _ := rsa.GenerateKey(rand.Reader, 2048)
	raw := []byte(`{"keys":[{"kty":"RSA","alg":"RS256","kid":"abc","n":"` +
		base64.RawURLEncoding.EncodeToString(priv.N.Bytes()) + `","e":"` +
		base64.RawURLEncoding.EncodeToString(big.NewInt(int64(priv.E)).Bytes()) + `"}]}`)
	keys, err := parseTelegramJWKS(raw)
	if err != nil {
		t.Fatalf("parseTelegramJWKS: %v", err)
	}
	if keys["abc"] == nil {
		t.Fatalf("missing kid abc")
	}
	if keys["abc"].N.Cmp(priv.N) != 0 || keys["abc"].E != priv.E {
		t.Fatalf("key mismatch")
	}
}

func mustQueryParam(t *testing.T, rawURL, key string) string {
	t.Helper()
	u, err := url.Parse(rawURL)
	if err != nil {
		t.Fatal(err)
	}
	v := u.Query().Get(key)
	if v == "" {
		t.Fatalf("missing query param %q", key)
	}
	return v
}
