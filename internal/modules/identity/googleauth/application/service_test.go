package application

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

type googleTokenTestClaims struct {
	Email           string `json:"email"`
	EmailVerified   bool   `json:"email_verified"`
	Name            string `json:"name"`
	Picture         string `json:"picture"`
	HD              string `json:"hd,omitempty"`
	AuthorizedParty string `json:"azp,omitempty"`
	jwt.RegisteredClaims
}

func generateGoogleTestKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate RSA key: %v", err)
	}
	return key
}

func googleTestJWK(kid string, key *rsa.PrivateKey) map[string]string {
	return map[string]string{
		"kty": "RSA",
		"kid": kid,
		"use": "sig",
		"alg": "RS256",
		"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
		"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
	}
}

func signGoogleTestCredential(
	t *testing.T,
	key *rsa.PrivateKey,
	kid string,
	now time.Time,
	mutate func(*googleTokenTestClaims),
) string {
	t.Helper()
	claims := &googleTokenTestClaims{
		Email:           "buyer@gmail.com",
		EmailVerified:   true,
		Name:            "Google Buyer",
		Picture:         "https://example.com/avatar.png",
		AuthorizedParty: "google-client-id",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "https://accounts.google.com",
			Subject:   "google-subject-123",
			Audience:  jwt.ClaimStrings{"google-client-id"},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(now.Add(-time.Minute)),
		},
	}
	if mutate != nil {
		mutate(claims)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = kid
	signed, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign credential: %v", err)
	}
	return signed
}

func newGoogleJWKSServer(t *testing.T, keys func() []map[string]string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=3600")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{"keys": keys()}); err != nil {
			t.Errorf("encode JWKS: %v", err)
		}
	}))
}

func TestVerifyCredentialHappyPath(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	key := generateGoogleTestKey(t)
	server := newGoogleJWKSServer(t, func() []map[string]string {
		return []map[string]string{googleTestJWK("key-1", key)}
	})
	defer server.Close()

	service := NewService(
		config.GoogleAuthConfig{Enabled: true, ClientID: "google-client-id"},
		WithJWKSURL(server.URL),
		WithHTTPClient(server.Client()),
		WithClock(func() time.Time { return now }),
	)
	verified, err := service.VerifyCredential(
		context.Background(),
		signGoogleTestCredential(t, key, "key-1", now, nil),
	)
	if err != nil {
		t.Fatalf("verify credential: %v", err)
	}
	if verified.Sub != "google-subject-123" || verified.Email != "buyer@gmail.com" {
		t.Fatalf("unexpected verified identity: %+v", verified)
	}
	if verified.ClientID != "google-client-id" {
		t.Fatalf("verified client ID = %q", verified.ClientID)
	}
	if !verified.EmailAuthoritative {
		t.Fatalf("gmail identity should be authoritative")
	}
	if verified.Name != "Google Buyer" || verified.Picture != "https://example.com/avatar.png" {
		t.Fatalf("profile claims lost: %+v", verified)
	}
}

func TestVerifyCredentialRejectsInvalidClaims(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	key := generateGoogleTestKey(t)
	server := newGoogleJWKSServer(t, func() []map[string]string {
		return []map[string]string{googleTestJWK("key-1", key)}
	})
	defer server.Close()

	service := NewService(
		config.GoogleAuthConfig{Enabled: true, ClientID: "google-client-id"},
		WithJWKSURL(server.URL),
		WithHTTPClient(server.Client()),
		WithClock(func() time.Time { return now }),
	)

	tests := []struct {
		name   string
		mutate func(*googleTokenTestClaims)
		target error
	}{
		{
			name: "bad issuer",
			mutate: func(claims *googleTokenTestClaims) {
				claims.Issuer = "https://evil.example"
			},
			target: ErrGoogleCredentialInvalid,
		},
		{
			name: "bad audience",
			mutate: func(claims *googleTokenTestClaims) {
				claims.Audience = jwt.ClaimStrings{"other-client"}
			},
			target: ErrGoogleCredentialInvalid,
		},
		{
			name: "multiple audiences",
			mutate: func(claims *googleTokenTestClaims) {
				claims.Audience = jwt.ClaimStrings{"google-client-id", "another-client"}
			},
			target: ErrGoogleCredentialInvalid,
		},
		{
			name: "authorized party mismatch",
			mutate: func(claims *googleTokenTestClaims) {
				claims.AuthorizedParty = "another-client"
			},
			target: ErrGoogleCredentialInvalid,
		},
		{
			name: "unverified email",
			mutate: func(claims *googleTokenTestClaims) {
				claims.EmailVerified = false
			},
			target: ErrGoogleEmailUnverified,
		},
		{
			name: "future issued at",
			mutate: func(claims *googleTokenTestClaims) {
				claims.IssuedAt = jwt.NewNumericDate(now.Add(5 * time.Minute))
			},
			target: ErrGoogleCredentialInvalid,
		},
		{
			name: "missing subject",
			mutate: func(claims *googleTokenTestClaims) {
				claims.Subject = ""
			},
			target: ErrGoogleCredentialInvalid,
		},
		{
			name: "subject exceeds persistence contract",
			mutate: func(claims *googleTokenTestClaims) {
				claims.Subject = strings.Repeat("s", maxGoogleSubjectBytes+1)
			},
			target: ErrGoogleCredentialInvalid,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.VerifyCredential(
				context.Background(),
				signGoogleTestCredential(t, key, "key-1", now, tt.mutate),
			)
			if !errors.Is(err, tt.target) {
				t.Fatalf("error = %v, want target %v", err, tt.target)
			}
		})
	}
}

func TestVerifyCredentialAcceptsGoogleSubjectUpTo255Bytes(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	key := generateGoogleTestKey(t)
	server := newGoogleJWKSServer(t, func() []map[string]string {
		return []map[string]string{googleTestJWK("key-1", key)}
	})
	defer server.Close()
	service := NewService(
		config.GoogleAuthConfig{Enabled: true, ClientID: "google-client-id"},
		WithJWKSURL(server.URL),
		WithHTTPClient(server.Client()),
		WithClock(func() time.Time { return now }),
	)

	for _, length := range []int{129, 255} {
		t.Run(fmt.Sprintf("%d bytes", length), func(t *testing.T) {
			subject := strings.Repeat("s", length)
			credential := signGoogleTestCredential(t, key, "key-1", now, func(claims *googleTokenTestClaims) {
				claims.Subject = subject
			})
			identity, err := service.VerifyCredential(context.Background(), credential)
			if err != nil {
				t.Fatalf("verify %d-byte subject: %v", length, err)
			}
			if identity.Sub != subject {
				t.Fatalf("subject length = %d, want %d", len(identity.Sub), length)
			}
		})
	}
}

func TestVerifyCredentialRejectsOversizedCredentialBeforeJWKSFetch(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		http.Error(w, "unexpected", http.StatusInternalServerError)
	}))
	defer server.Close()
	service := NewService(
		config.GoogleAuthConfig{Enabled: true, ClientID: "google-client-id"},
		WithJWKSURL(server.URL),
		WithHTTPClient(server.Client()),
	)

	_, err := service.VerifyCredential(context.Background(), strings.Repeat("x", maxGoogleCredentialBytes+1))
	if !errors.Is(err, ErrGoogleCredentialInvalid) {
		t.Fatalf("error = %v, want invalid credential", err)
	}
	if requests != 0 {
		t.Fatalf("JWKS requests = %d, want 0", requests)
	}
}

func TestVerifyCredentialDropsUnsafePictureClaim(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	key := generateGoogleTestKey(t)
	server := newGoogleJWKSServer(t, func() []map[string]string {
		return []map[string]string{googleTestJWK("key-1", key)}
	})
	defer server.Close()
	service := NewService(
		config.GoogleAuthConfig{Enabled: true, ClientID: "google-client-id"},
		WithJWKSURL(server.URL),
		WithHTTPClient(server.Client()),
		WithClock(func() time.Time { return now }),
	)

	tests := []string{
		"javascript:alert(1)",
		"https://user:password@example.com/avatar.png",
		"https://example.com/" + strings.Repeat("x", maxGooglePictureURLBytes),
	}
	for _, picture := range tests {
		credential := signGoogleTestCredential(t, key, "key-1", now, func(claims *googleTokenTestClaims) {
			claims.Picture = picture
		})
		identity, err := service.VerifyCredential(context.Background(), credential)
		if err != nil {
			t.Fatalf("verify credential: %v", err)
		}
		if identity.Picture != "" {
			t.Fatalf("unsafe picture persisted as %q", identity.Picture)
		}
	}
}

func TestVerifyCredentialDerivesEmailAuthorityFromGoogleClaims(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	key := generateGoogleTestKey(t)
	server := newGoogleJWKSServer(t, func() []map[string]string {
		return []map[string]string{googleTestJWK("key-1", key)}
	})
	defer server.Close()
	service := NewService(
		config.GoogleAuthConfig{Enabled: true, ClientID: "google-client-id"},
		WithJWKSURL(server.URL),
		WithHTTPClient(server.Client()),
		WithClock(func() time.Time { return now }),
	)

	tests := []struct {
		name          string
		email         string
		hostedDomain  string
		authoritative bool
	}{
		{
			name:          "workspace domain",
			email:         "buyer@workspace.example",
			hostedDomain:  "workspace.example",
			authoritative: true,
		},
		{
			name:          "custom domain without hosted domain",
			email:         "buyer@example.com",
			authoritative: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			credential := signGoogleTestCredential(t, key, "key-1", now, func(claims *googleTokenTestClaims) {
				claims.Email = tt.email
				claims.HD = tt.hostedDomain
			})
			verified, err := service.VerifyCredential(context.Background(), credential)
			if err != nil {
				t.Fatalf("verify credential: %v", err)
			}
			if verified.EmailAuthoritative != tt.authoritative {
				t.Fatalf("EmailAuthoritative = %v, want %v; identity=%+v", verified.EmailAuthoritative, tt.authoritative, verified)
			}
		})
	}
}

func TestVerifyCredentialRefreshesJWKSOnKidRotation(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	firstKey := generateGoogleTestKey(t)
	rotatedKey := generateGoogleTestKey(t)
	var mu sync.Mutex
	currentKeys := []map[string]string{googleTestJWK("key-1", firstKey)}
	requests := 0
	server := newGoogleJWKSServer(t, func() []map[string]string {
		mu.Lock()
		defer mu.Unlock()
		requests++
		result := make([]map[string]string, len(currentKeys))
		copy(result, currentKeys)
		return result
	})
	defer server.Close()

	service := NewService(
		config.GoogleAuthConfig{Enabled: true, ClientID: "google-client-id"},
		WithJWKSURL(server.URL),
		WithHTTPClient(server.Client()),
		WithClock(func() time.Time { return now }),
	)
	if _, err := service.VerifyCredential(
		context.Background(),
		signGoogleTestCredential(t, firstKey, "key-1", now, nil),
	); err != nil {
		t.Fatalf("verify first key: %v", err)
	}

	mu.Lock()
	currentKeys = []map[string]string{googleTestJWK("key-2", rotatedKey)}
	mu.Unlock()
	if _, err := service.VerifyCredential(
		context.Background(),
		signGoogleTestCredential(t, rotatedKey, "key-2", now, nil),
	); err != nil {
		t.Fatalf("verify rotated key: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if requests != 2 {
		t.Fatalf("JWKS requests = %d, want 2 (initial + forced rotation refresh)", requests)
	}
}

func TestUnknownKidsDoNotAmplifyJWKSRequests(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	trustedKey := generateGoogleTestKey(t)
	attackerKey := generateGoogleTestKey(t)
	var mu sync.Mutex
	requests := 0
	server := newGoogleJWKSServer(t, func() []map[string]string {
		mu.Lock()
		requests++
		mu.Unlock()
		return []map[string]string{googleTestJWK("trusted-key", trustedKey)}
	})
	defer server.Close()
	service := NewService(
		config.GoogleAuthConfig{Enabled: true, ClientID: "google-client-id"},
		WithJWKSURL(server.URL),
		WithHTTPClient(server.Client()),
		WithClock(func() time.Time { return now }),
	)
	if _, err := service.VerifyCredential(
		context.Background(),
		signGoogleTestCredential(t, trustedKey, "trusted-key", now, nil),
	); err != nil {
		t.Fatalf("warm JWKS cache: %v", err)
	}

	const attempts = 20
	credentials := make([]string, attempts)
	for index := range credentials {
		credentials[index] = signGoogleTestCredential(
			t,
			attackerKey,
			fmt.Sprintf("unknown-key-%d", index),
			now,
			nil,
		)
	}
	var waitGroup sync.WaitGroup
	errorsSeen := make(chan error, attempts)
	for _, credential := range credentials {
		waitGroup.Add(1)
		go func(value string) {
			defer waitGroup.Done()
			_, err := service.VerifyCredential(context.Background(), value)
			errorsSeen <- err
		}(credential)
	}
	waitGroup.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if !errors.Is(err, ErrGoogleCredentialInvalid) {
			t.Fatalf("unknown kid error = %v, want invalid credential", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if requests != 2 {
		t.Fatalf("JWKS requests = %d, want 2 (warm cache + one forced refresh)", requests)
	}
}

func TestUnknownKidsQueuedBehindSlowRefreshDoNotRetrySequentially(t *testing.T) {
	trustedKey := generateGoogleTestKey(t)
	attackerKey := generateGoogleTestKey(t)
	var mu sync.Mutex
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		requests++
		currentRequest := requests
		mu.Unlock()
		if currentRequest > 1 {
			time.Sleep(50 * time.Millisecond)
		}
		w.Header().Set("Cache-Control", "public, max-age=3600")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"keys": []map[string]string{googleTestJWK("trusted-key", trustedKey)},
		}); err != nil {
			t.Errorf("encode JWKS: %v", err)
		}
	}))
	defer server.Close()

	service := NewService(
		config.GoogleAuthConfig{Enabled: true, ClientID: "google-client-id"},
		WithJWKSURL(server.URL),
		WithHTTPClient(server.Client()),
	)
	service.unknownKidCooldown = 20 * time.Millisecond
	now := time.Now()
	if _, err := service.VerifyCredential(
		context.Background(),
		signGoogleTestCredential(t, trustedKey, "trusted-key", now, nil),
	); err != nil {
		t.Fatalf("warm JWKS cache: %v", err)
	}

	const attempts = 20
	start := make(chan struct{})
	var waitGroup sync.WaitGroup
	errorsSeen := make(chan error, attempts)
	for index := 0; index < attempts; index++ {
		credential := signGoogleTestCredential(
			t,
			attackerKey,
			fmt.Sprintf("slow-unknown-key-%d", index),
			now,
			nil,
		)
		waitGroup.Add(1)
		go func(value string) {
			defer waitGroup.Done()
			<-start
			_, err := service.VerifyCredential(context.Background(), value)
			errorsSeen <- err
		}(credential)
	}
	close(start)
	waitGroup.Wait()
	close(errorsSeen)
	for err := range errorsSeen {
		if !errors.Is(err, ErrGoogleCredentialInvalid) {
			t.Fatalf("unknown kid error = %v, want invalid credential", err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if requests != 2 {
		t.Fatalf("JWKS requests = %d, want 2 (warm cache + one completed slow refresh)", requests)
	}
}

func TestSetConfigAndPublicConfigAreConcurrencySafe(t *testing.T) {
	service := NewService(config.GoogleAuthConfig{Enabled: true, ClientID: "initial"})
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			service.SetConfig(config.GoogleAuthConfig{Enabled: true, ClientID: "updated"})
		}()
		go func() {
			defer wg.Done()
			_ = service.PublicConfig()
		}()
	}
	wg.Wait()
	if got := service.PublicConfig(); got["enabled"] != true || got["client_id"] != "updated" {
		t.Fatalf("unexpected public config: %+v", got)
	}
}

func TestValidateRuntimeConfigAndClientID(t *testing.T) {
	service := NewService(config.GoogleAuthConfig{})
	if err := service.ValidateRuntimeConfig(); !errors.Is(err, ErrGoogleAuthDisabled) {
		t.Fatalf("disabled config error = %v", err)
	}
	service.SetConfig(config.GoogleAuthConfig{Enabled: true})
	if err := service.ValidateRuntimeConfig(); !errors.Is(err, ErrGoogleAuthConfigInvalid) {
		t.Fatalf("missing client ID error = %v", err)
	}
	service.SetConfig(config.GoogleAuthConfig{Enabled: true, ClientID: "client-id"})
	clientID, err := service.RuntimeClientID()
	if err != nil || clientID != "client-id" {
		t.Fatalf("RuntimeClientID() = %q, %v", clientID, err)
	}
}
