package application

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dujiao-next/internal/config"

	"github.com/golang-jwt/jwt/v5"
)

const (
	defaultGoogleJWKSURL     = "https://www.googleapis.com/oauth2/v3/certs"
	defaultHTTPTimeout       = 5 * time.Second
	defaultJWKSCacheTTL      = time.Hour
	maxJWKSResponseBytes     = 1 << 20
	googleTokenClockSkew     = time.Minute
	maxGoogleSubjectBytes    = 255
	maxGoogleCredentialBytes = 64 << 10
	maxGooglePictureURLBytes = 2048
	unknownKidRefreshWait    = 5 * time.Second
)

// VerifiedIdentity is the normalized identity extracted from a verified
// Google Identity Services ID token.
type VerifiedIdentity struct {
	Sub                string
	Email              string
	Name               string
	Picture            string
	HD                 string
	ClientID           string
	EmailAuthoritative bool
	AuthAt             time.Time
}

type googleIDTokenClaims struct {
	Email           string `json:"email"`
	EmailVerified   bool   `json:"email_verified"`
	Name            string `json:"name"`
	Picture         string `json:"picture"`
	HD              string `json:"hd,omitempty"`
	AuthorizedParty string `json:"azp,omitempty"`
	jwt.RegisteredClaims
}

type googleJWKSet struct {
	Keys []googleJWK `json:"keys"`
}

type googleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// Option customizes the Google credential verifier.
type Option func(*Service)

// WithJWKSURL overrides Google's JWKS endpoint. Intended for tests.
func WithJWKSURL(endpoint string) Option {
	return func(service *Service) {
		if normalized := strings.TrimSpace(endpoint); normalized != "" {
			service.jwksURL = normalized
		}
	}
}

// WithHTTPClient injects the HTTP client used for JWKS retrieval.
func WithHTTPClient(client *http.Client) Option {
	return func(service *Service) {
		if client != nil {
			service.httpClient = client
		}
	}
}

// WithClock injects the verifier clock. Intended for deterministic tests.
func WithClock(clock func() time.Time) Option {
	return func(service *Service) {
		if clock != nil {
			service.now = clock
		}
	}
}

// Service verifies Google Identity Services credentials and owns a
// concurrency-safe runtime configuration and JWKS cache.
type Service struct {
	configMu sync.RWMutex
	cfg      config.GoogleAuthConfig

	httpClient *http.Client
	jwksURL    string
	now        func() time.Time

	refreshMu          sync.Mutex
	keysMu             sync.RWMutex
	keys               map[string]*rsa.PublicKey
	keysUntil          time.Time
	lastForcedRefresh  time.Time
	unknownKidCooldown time.Duration
}

// NewService constructs a Google credential verifier.
func NewService(cfg config.GoogleAuthConfig, options ...Option) *Service {
	service := &Service{
		httpClient:         &http.Client{Timeout: defaultHTTPTimeout},
		jwksURL:            defaultGoogleJWKSURL,
		now:                time.Now,
		keys:               make(map[string]*rsa.PublicKey),
		unknownKidCooldown: unknownKidRefreshWait,
	}
	service.SetConfig(cfg)
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service
}

// SetConfig applies dynamic Google auth settings without racing in-flight
// credential verification.
func (s *Service) SetConfig(cfg config.GoogleAuthConfig) {
	if s == nil {
		return
	}
	cfg.ClientID = strings.TrimSpace(cfg.ClientID)
	s.configMu.Lock()
	s.cfg = cfg
	s.configMu.Unlock()
}

func (s *Service) configSnapshot() config.GoogleAuthConfig {
	if s == nil {
		return config.GoogleAuthConfig{}
	}
	s.configMu.RLock()
	defer s.configMu.RUnlock()
	return s.cfg
}

// PublicConfig returns only browser-safe configuration.
func (s *Service) PublicConfig() map[string]interface{} {
	cfg := s.configSnapshot()
	return map[string]interface{}{
		"enabled":   cfg.Enabled && cfg.ClientID != "",
		"client_id": cfg.ClientID,
	}
}

// ValidateRuntimeConfig checks whether a server-side Google authentication
// operation may start without requiring a credential to be supplied first.
func (s *Service) ValidateRuntimeConfig() error {
	if s == nil {
		return ErrGoogleAuthConfigInvalid
	}
	cfg := s.configSnapshot()
	if !cfg.Enabled {
		return ErrGoogleAuthDisabled
	}
	if cfg.ClientID == "" || s.httpClient == nil || strings.TrimSpace(s.jwksURL) == "" {
		return ErrGoogleAuthConfigInvalid
	}
	return nil
}

// RuntimeClientID returns the currently active audience after validating that
// Google authentication is enabled.
func (s *Service) RuntimeClientID() (string, error) {
	if err := s.ValidateRuntimeConfig(); err != nil {
		return "", err
	}
	return s.configSnapshot().ClientID, nil
}

// VerifyCredential verifies a Google Identity Services ID token credential.
func (s *Service) VerifyCredential(ctx context.Context, credential string) (*VerifiedIdentity, error) {
	cfg := s.configSnapshot()
	if !cfg.Enabled {
		return nil, ErrGoogleAuthDisabled
	}
	if cfg.ClientID == "" || s == nil || s.httpClient == nil || strings.TrimSpace(s.jwksURL) == "" {
		return nil, ErrGoogleAuthConfigInvalid
	}
	if len(credential) > maxGoogleCredentialBytes {
		return nil, ErrGoogleCredentialInvalid
	}
	credential = strings.TrimSpace(credential)
	if credential == "" {
		return nil, ErrGoogleCredentialInvalid
	}
	if ctx == nil {
		ctx = context.Background()
	}

	claims := &googleIDTokenClaims{}
	token, err := jwt.ParseWithClaims(
		credential,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			kid, _ := token.Header["kid"].(string)
			kid = strings.TrimSpace(kid)
			if kid == "" {
				return nil, ErrGoogleCredentialInvalid
			}
			return s.keyForID(ctx, kid)
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		jwt.WithAudience(cfg.ClientID),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithLeeway(googleTokenClockSkew),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		if isGoogleJWKSFailure(err) {
			return nil, fmt.Errorf("%w: %v", ErrGoogleJWKSUnavailable, err)
		}
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, fmt.Errorf("%w: %v", ErrGoogleCredentialExpired, err)
		}
		return nil, fmt.Errorf("%w: %v", ErrGoogleCredentialInvalid, err)
	}
	if token == nil || !token.Valid {
		return nil, ErrGoogleCredentialInvalid
	}

	now := s.now()
	if claims.Issuer != "accounts.google.com" && claims.Issuer != "https://accounts.google.com" {
		return nil, ErrGoogleCredentialInvalid
	}
	if len(claims.Audience) != 1 || claims.Audience[0] != cfg.ClientID {
		return nil, ErrGoogleCredentialInvalid
	}
	if claims.AuthorizedParty != "" && claims.AuthorizedParty != cfg.ClientID {
		return nil, ErrGoogleCredentialInvalid
	}
	subject := strings.TrimSpace(claims.Subject)
	if subject == "" || len(subject) > maxGoogleSubjectBytes {
		return nil, ErrGoogleCredentialInvalid
	}
	if claims.IssuedAt == nil || claims.IssuedAt.Time.After(now.Add(googleTokenClockSkew)) {
		return nil, ErrGoogleCredentialInvalid
	}
	email, err := normalizeGoogleEmail(claims.Email)
	if err != nil {
		return nil, ErrGoogleCredentialInvalid
	}
	if !claims.EmailVerified {
		return nil, ErrGoogleEmailUnverified
	}

	hostedDomain := strings.ToLower(strings.TrimSpace(claims.HD))
	return &VerifiedIdentity{
		Sub:                subject,
		Email:              email,
		Name:               strings.TrimSpace(claims.Name),
		Picture:            normalizeGooglePictureURL(claims.Picture),
		HD:                 hostedDomain,
		ClientID:           cfg.ClientID,
		EmailAuthoritative: strings.HasSuffix(email, "@gmail.com") || hostedDomain != "",
		AuthAt:             claims.IssuedAt.Time,
	}, nil
}

func normalizeGooglePictureURL(value string) string {
	normalized := strings.TrimSpace(value)
	if normalized == "" || len(normalized) > maxGooglePictureURLBytes {
		return ""
	}
	parsed, err := url.Parse(normalized)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.String()
}

func normalizeGoogleEmail(value string) (string, error) {
	normalized := strings.ToLower(strings.TrimSpace(value))
	address, err := mail.ParseAddress(normalized)
	if err != nil || address.Address != normalized {
		return "", ErrGoogleCredentialInvalid
	}
	return normalized, nil
}

func isGoogleJWKSFailure(err error) bool {
	return errors.Is(err, ErrGoogleJWKSUnavailable)
}

func (s *Service) keyForID(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if key, fresh := s.cachedKey(kid); fresh {
		if key != nil {
			return key, nil
		}
		// The cache is fresh but the kid is absent. Force at most one refresh
		// per short global window to pick up a real Google key rotation without
		// allowing arbitrary kid values to amplify outbound JWKS traffic.
		if err := s.refreshKeys(ctx, true); err != nil {
			return nil, err
		}
		if key, _ := s.cachedKey(kid); key != nil {
			return key, nil
		}
		return nil, ErrGoogleCredentialInvalid
	}

	// An empty or expired cache is already refreshed once here. If the kid is
	// still absent, a second forced request cannot add information and would
	// only double anonymous outbound traffic.
	if err := s.refreshKeys(ctx, false); err != nil {
		return nil, err
	}
	if key, _ := s.cachedKey(kid); key != nil {
		return key, nil
	}
	return nil, ErrGoogleCredentialInvalid
}

func (s *Service) cachedKey(kid string) (*rsa.PublicKey, bool) {
	s.keysMu.RLock()
	defer s.keysMu.RUnlock()
	return s.keys[kid], s.now().Before(s.keysUntil)
}

func (s *Service) refreshKeys(ctx context.Context, force bool) error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	if !force {
		s.keysMu.RLock()
		fresh := s.now().Before(s.keysUntil) && len(s.keys) > 0
		s.keysMu.RUnlock()
		if fresh {
			return nil
		}
	} else {
		s.keysMu.RLock()
		now := s.now()
		if !s.lastForcedRefresh.IsZero() && now.Sub(s.lastForcedRefresh) < s.unknownKidCooldown {
			s.keysMu.RUnlock()
			return nil
		}
		s.keysMu.RUnlock()
		// Record completion (success or failure), not start time: requests
		// queued behind a slow five-second fetch must still observe a full
		// cooldown instead of each starting another outbound call in turn.
		defer func() {
			s.keysMu.Lock()
			s.lastForcedRefresh = s.now()
			s.keysMu.Unlock()
		}()
	}

	requestCtx, cancel := context.WithTimeout(ctx, defaultHTTPTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(requestCtx, http.MethodGet, s.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGoogleJWKSUnavailable, err)
	}
	response, err := s.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrGoogleJWKSUnavailable, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("%w: status %d", ErrGoogleJWKSUnavailable, response.StatusCode)
	}

	var payload googleJWKSet
	decoder := json.NewDecoder(io.LimitReader(response.Body, maxJWKSResponseBytes))
	if err := decoder.Decode(&payload); err != nil {
		return fmt.Errorf("%w: %v", ErrGoogleJWKSUnavailable, err)
	}
	keys := make(map[string]*rsa.PublicKey, len(payload.Keys))
	for _, item := range payload.Keys {
		key, err := parseGoogleRSAJWK(item)
		if err != nil {
			continue
		}
		keys[strings.TrimSpace(item.Kid)] = key
	}
	if len(keys) == 0 {
		return fmt.Errorf("%w: no usable RSA signing keys", ErrGoogleJWKSUnavailable)
	}
	ttl := resolveJWKSCacheTTL(response.Header)
	s.keysMu.Lock()
	s.keys = keys
	s.keysUntil = s.now().Add(ttl)
	s.keysMu.Unlock()
	return nil
}

func parseGoogleRSAJWK(item googleJWK) (*rsa.PublicKey, error) {
	if item.Kty != "RSA" || strings.TrimSpace(item.Kid) == "" {
		return nil, ErrGoogleCredentialInvalid
	}
	if item.Use != "" && item.Use != "sig" {
		return nil, ErrGoogleCredentialInvalid
	}
	if item.Alg != "" && item.Alg != jwt.SigningMethodRS256.Alg() {
		return nil, ErrGoogleCredentialInvalid
	}
	modulusBytes, err := base64.RawURLEncoding.DecodeString(item.N)
	if err != nil || len(modulusBytes) == 0 {
		return nil, ErrGoogleCredentialInvalid
	}
	exponentBytes, err := base64.RawURLEncoding.DecodeString(item.E)
	if err != nil || len(exponentBytes) == 0 || len(exponentBytes) > 4 {
		return nil, ErrGoogleCredentialInvalid
	}
	exponent := 0
	for _, value := range exponentBytes {
		exponent = exponent<<8 | int(value)
	}
	if exponent < 3 {
		return nil, ErrGoogleCredentialInvalid
	}
	return &rsa.PublicKey{N: new(big.Int).SetBytes(modulusBytes), E: exponent}, nil
}

func resolveJWKSCacheTTL(headers http.Header) time.Duration {
	for _, directive := range strings.Split(headers.Get("Cache-Control"), ",") {
		name, value, found := strings.Cut(strings.TrimSpace(directive), "=")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "max-age") {
			continue
		}
		seconds, err := strconv.Atoi(strings.Trim(strings.TrimSpace(value), `"`))
		if err == nil && seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
	}
	return defaultJWKSCacheTTL
}
