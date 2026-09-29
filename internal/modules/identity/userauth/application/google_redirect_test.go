package application

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/dujiao-next/internal/config"
	googleauthapp "github.com/dujiao-next/internal/modules/identity/googleauth/application"
)

type fakeGoogleRedirectStore struct {
	mu           sync.Mutex
	intents      map[string]GoogleRedirectIntent
	handoffs     map[string]GoogleRedirectHandoff
	takeIntent   int
	takeHandoff  int
	intentTTL    time.Duration
	handoffTTL   time.Duration
	putIntentErr error
}

func newFakeGoogleRedirectStore() *fakeGoogleRedirectStore {
	return &fakeGoogleRedirectStore{
		intents:  make(map[string]GoogleRedirectIntent),
		handoffs: make(map[string]GoogleRedirectHandoff),
	}
}

func (s *fakeGoogleRedirectStore) PutIntent(
	_ context.Context,
	state string,
	intent GoogleRedirectIntent,
	ttl time.Duration,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putIntentErr != nil {
		return s.putIntentErr
	}
	s.intents[state] = intent
	s.intentTTL = ttl
	return nil
}

func (s *fakeGoogleRedirectStore) TakeIntent(
	_ context.Context,
	state string,
) (*GoogleRedirectIntent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.takeIntent++
	intent, ok := s.intents[state]
	if !ok {
		return nil, nil
	}
	delete(s.intents, state)
	copyValue := intent
	return &copyValue, nil
}

func (s *fakeGoogleRedirectStore) PutHandoff(
	_ context.Context,
	handle string,
	handoff GoogleRedirectHandoff,
	ttl time.Duration,
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handoffs[handle] = handoff
	s.handoffTTL = ttl
	return nil
}

func (s *fakeGoogleRedirectStore) TakeHandoff(
	_ context.Context,
	handle string,
) (*GoogleRedirectHandoff, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.takeHandoff++
	handoff, ok := s.handoffs[handle]
	if !ok {
		return nil, nil
	}
	delete(s.handoffs, handle)
	copyValue := handoff
	return &copyValue, nil
}

func TestCreateGoogleRedirectIntentUsesCanonicalStateAndTenMinuteTTL(t *testing.T) {
	store := newFakeGoogleRedirectStore()
	service := &Service{
		googleAuthService: googleauthapp.NewService(config.GoogleAuthConfig{
			Enabled:  true,
			ClientID: "client-id",
		}),
		googleRedirectStore: store,
	}
	tenant := GoogleRedirectTenant{Host: "shop.example.com", IsMain: true}

	state, err := service.CreateGoogleRedirectIntent(
		context.Background(),
		GoogleRedirectFlowLogin,
		0,
		tenant,
	)
	if err != nil {
		t.Fatalf("CreateGoogleRedirectIntent() error = %v", err)
	}
	if !ValidGoogleRedirectHandle(state) {
		t.Fatalf("state %q is not a canonical 32-byte base64url handle", state)
	}
	if store.intentTTL != GoogleRedirectIntentTTL {
		t.Fatalf("intent TTL = %s, want %s", store.intentTTL, GoogleRedirectIntentTTL)
	}
	if stored := store.intents[state]; stored.Flow != GoogleRedirectFlowLogin || stored.Tenant != tenant {
		t.Fatalf("stored intent = %#v", stored)
	}
}

func TestCreateGoogleRedirectIntentFailsClosedWhenStoreFails(t *testing.T) {
	store := newFakeGoogleRedirectStore()
	store.putIntentErr = errors.New("redis down")
	service := &Service{
		googleAuthService: googleauthapp.NewService(config.GoogleAuthConfig{
			Enabled:  true,
			ClientID: "client-id",
		}),
		googleRedirectStore: store,
	}

	_, err := service.CreateGoogleRedirectIntent(
		context.Background(),
		GoogleRedirectFlowLogin,
		0,
		GoogleRedirectTenant{Host: "shop.example.com", IsMain: true},
	)
	if !errors.Is(err, ErrGoogleRedirectUnavailable) {
		t.Fatalf("error = %v, want ErrGoogleRedirectUnavailable", err)
	}
}

func TestGoogleRedirectMalformedHandleDoesNotReachStore(t *testing.T) {
	store := newFakeGoogleRedirectStore()
	service := &Service{googleRedirectStore: store}
	tenant := GoogleRedirectTenant{Host: "shop.example.com", IsMain: true}

	if _, err := service.takeGoogleRedirectHandoff(
		context.Background(),
		"not-a-valid-handle",
		GoogleRedirectFlowLogin,
		0,
		tenant,
	); !errors.Is(err, ErrGoogleRedirectSessionExpired) {
		t.Fatalf("error = %v, want ErrGoogleRedirectSessionExpired", err)
	}
	if store.takeHandoff != 0 {
		t.Fatalf("TakeHandoff calls = %d, want 0", store.takeHandoff)
	}
	if _, err := service.CompleteGoogleRedirect(
		context.Background(),
		string(make([]byte, 4096)),
		"credential",
		tenant,
	); !errors.Is(err, ErrGoogleRedirectSessionExpired) {
		t.Fatalf("CompleteGoogleRedirect error = %v, want ErrGoogleRedirectSessionExpired", err)
	}
	if store.takeIntent != 0 {
		t.Fatalf("TakeIntent calls = %d, want 0", store.takeIntent)
	}
}

func TestCompleteGoogleRedirectReturnsConsumedTrustedFlowOnCredentialFailure(t *testing.T) {
	store := newFakeGoogleRedirectStore()
	service := &Service{
		googleAuthService: googleauthapp.NewService(config.GoogleAuthConfig{
			Enabled:  true,
			ClientID: "client-id",
		}),
		googleRedirectStore: store,
	}
	state, err := newGoogleRedirectHandle()
	if err != nil {
		t.Fatalf("newGoogleRedirectHandle() error = %v", err)
	}
	tenant := GoogleRedirectTenant{Host: "shop.example.com", IsMain: true}
	store.intents[state] = GoogleRedirectIntent{
		Flow:   GoogleRedirectFlowBind,
		UserID: 42,
		Tenant: tenant,
	}

	completion, err := service.CompleteGoogleRedirect(
		context.Background(),
		state,
		"",
		tenant,
	)
	if !errors.Is(err, googleauthapp.ErrGoogleCredentialInvalid) {
		t.Fatalf("error = %v, want invalid credential", err)
	}
	if completion == nil || completion.Flow != GoogleRedirectFlowBind || completion.HandoffHandle != "" {
		t.Fatalf("completion = %#v, want trusted bind flow without handoff", completion)
	}
	if _, exists := store.intents[state]; exists {
		t.Fatalf("credential failure restored consumed intent")
	}
}

func TestCompleteGoogleRedirectReturnsConsumedTrustedFlowOnTenantFailure(t *testing.T) {
	store := newFakeGoogleRedirectStore()
	service := &Service{googleRedirectStore: store}
	state, err := newGoogleRedirectHandle()
	if err != nil {
		t.Fatalf("newGoogleRedirectHandle() error = %v", err)
	}
	store.intents[state] = GoogleRedirectIntent{
		Flow:   GoogleRedirectFlowBind,
		UserID: 42,
		Tenant: GoogleRedirectTenant{Host: "shop.example.com", IsMain: true},
	}

	completion, err := service.CompleteGoogleRedirect(
		context.Background(),
		state,
		"credential-must-not-be-verified",
		GoogleRedirectTenant{Host: "other.example.com", IsMain: true},
	)
	if !errors.Is(err, ErrGoogleRedirectTenantMismatch) {
		t.Fatalf("error = %v, want tenant mismatch", err)
	}
	if completion == nil || completion.Flow != GoogleRedirectFlowBind {
		t.Fatalf("completion = %#v, want trusted bind flow", completion)
	}
	if _, exists := store.intents[state]; exists {
		t.Fatalf("tenant failure restored consumed intent")
	}
}

func TestGoogleRedirectTenantMismatchConsumesHandoff(t *testing.T) {
	service, store, handle := redirectExchangeFixture(t, GoogleRedirectFlowLogin, 0)

	_, err := service.ExchangeGoogleRedirectLogin(
		context.Background(),
		handle,
		GoogleRedirectTenant{Host: "other.example.com", IsMain: true},
	)
	if !errors.Is(err, ErrGoogleRedirectTenantMismatch) {
		t.Fatalf("first error = %v, want tenant mismatch", err)
	}
	_, err = service.ExchangeGoogleRedirectLogin(
		context.Background(),
		handle,
		GoogleRedirectTenant{Host: "shop.example.com", IsMain: true},
	)
	if !errors.Is(err, ErrGoogleRedirectSessionExpired) {
		t.Fatalf("second error = %v, want expired", err)
	}
	if store.takeHandoff != 2 {
		t.Fatalf("TakeHandoff calls = %d, want 2", store.takeHandoff)
	}
}

func TestGoogleRedirectBindUserMismatchConsumesHandoff(t *testing.T) {
	service, _, handle := redirectExchangeFixture(t, GoogleRedirectFlowBind, 42)

	_, err := service.ExchangeGoogleRedirectBind(
		context.Background(),
		handle,
		43,
		GoogleRedirectTenant{Host: "shop.example.com", IsMain: true},
	)
	if !errors.Is(err, ErrGoogleRedirectUserMismatch) {
		t.Fatalf("first error = %v, want user mismatch", err)
	}
	_, err = service.ExchangeGoogleRedirectBind(
		context.Background(),
		handle,
		42,
		GoogleRedirectTenant{Host: "shop.example.com", IsMain: true},
	)
	if !errors.Is(err, ErrGoogleRedirectSessionExpired) {
		t.Fatalf("second error = %v, want expired", err)
	}
}

func TestGoogleRedirectExchangeConsumesHandoffWhenProviderWasDisabled(t *testing.T) {
	service, _, handle := redirectExchangeFixture(t, GoogleRedirectFlowLogin, 0)
	service.googleAuthService.SetConfig(config.GoogleAuthConfig{
		Enabled:  false,
		ClientID: "client-id",
	})

	_, err := service.ExchangeGoogleRedirectLogin(
		context.Background(),
		handle,
		GoogleRedirectTenant{Host: "shop.example.com", IsMain: true},
	)
	if !errors.Is(err, googleauthapp.ErrGoogleAuthDisabled) {
		t.Fatalf("first error = %v, want Google auth disabled", err)
	}
	_, err = service.ExchangeGoogleRedirectLogin(
		context.Background(),
		handle,
		GoogleRedirectTenant{Host: "shop.example.com", IsMain: true},
	)
	if !errors.Is(err, ErrGoogleRedirectSessionExpired) {
		t.Fatalf("second error = %v, want expired", err)
	}
}

func TestGoogleRedirectExchangeRejectsHandoffFromPreviousClientID(t *testing.T) {
	service, _, handle := redirectExchangeFixture(t, GoogleRedirectFlowLogin, 0)
	service.googleAuthService.SetConfig(config.GoogleAuthConfig{
		Enabled:  true,
		ClientID: "rotated-client-id",
	})

	_, err := service.ExchangeGoogleRedirectLogin(
		context.Background(),
		handle,
		GoogleRedirectTenant{Host: "shop.example.com", IsMain: true},
	)
	if !errors.Is(err, ErrGoogleRedirectSessionExpired) {
		t.Fatalf("error = %v, want expired after client ID rotation", err)
	}
}

func redirectExchangeFixture(
	t *testing.T,
	flow string,
	userID uint,
) (*Service, *fakeGoogleRedirectStore, string) {
	t.Helper()
	handle, err := newGoogleRedirectHandle()
	if err != nil {
		t.Fatalf("newGoogleRedirectHandle() error = %v", err)
	}
	store := newFakeGoogleRedirectStore()
	store.handoffs[handle] = GoogleRedirectHandoff{
		Flow:   flow,
		UserID: userID,
		Tenant: GoogleRedirectTenant{Host: "shop.example.com", IsMain: true},
		Identity: googleauthapp.VerifiedIdentity{
			Sub:                "google-sub",
			Email:              "person@gmail.com",
			ClientID:           "client-id",
			EmailAuthoritative: true,
			AuthAt:             time.Now(),
		},
		CreatedAt: time.Now(),
	}
	service := &Service{
		googleAuthService: googleauthapp.NewService(config.GoogleAuthConfig{
			Enabled:  true,
			ClientID: "client-id",
		}),
		googleRedirectStore: store,
	}
	return service, store, handle
}
