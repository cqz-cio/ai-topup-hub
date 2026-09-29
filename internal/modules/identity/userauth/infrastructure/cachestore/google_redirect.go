package cachestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/dujiao-next/internal/cache"
	userauthapp "github.com/dujiao-next/internal/modules/identity/userauth/application"
)

const googleRedirectKeyPrefix = "user_auth:google_redirect"

var errInvalidGoogleRedirectHandle = errors.New("invalid google redirect handle")

// GoogleRedirectStore persists short-lived redirect state in Redis. Popup
// login does not depend on this store.
type GoogleRedirectStore struct{}

func NewGoogleRedirectStore() *GoogleRedirectStore {
	return &GoogleRedirectStore{}
}

func (*GoogleRedirectStore) PutIntent(
	ctx context.Context,
	state string,
	intent userauthapp.GoogleRedirectIntent,
	ttl time.Duration,
) error {
	if !userauthapp.ValidGoogleRedirectHandle(state) {
		return errInvalidGoogleRedirectHandle
	}
	return cache.SetJSONRequired(ctx, googleRedirectIntentKey(state), intent, ttl)
}

func (*GoogleRedirectStore) TakeIntent(
	ctx context.Context,
	state string,
) (*userauthapp.GoogleRedirectIntent, error) {
	if !userauthapp.ValidGoogleRedirectHandle(state) {
		return nil, nil
	}
	var intent userauthapp.GoogleRedirectIntent
	found, err := cache.GetDelJSONRequired(ctx, googleRedirectIntentKey(state), &intent)
	if err != nil || !found {
		return nil, err
	}
	return &intent, nil
}

func (*GoogleRedirectStore) PutHandoff(
	ctx context.Context,
	handle string,
	handoff userauthapp.GoogleRedirectHandoff,
	ttl time.Duration,
) error {
	if !userauthapp.ValidGoogleRedirectHandle(handle) {
		return errInvalidGoogleRedirectHandle
	}
	return cache.SetJSONRequired(ctx, googleRedirectHandoffKey(handle), handoff, ttl)
}

func (*GoogleRedirectStore) TakeHandoff(
	ctx context.Context,
	handle string,
) (*userauthapp.GoogleRedirectHandoff, error) {
	if !userauthapp.ValidGoogleRedirectHandle(handle) {
		return nil, nil
	}
	var handoff userauthapp.GoogleRedirectHandoff
	found, err := cache.GetDelJSONRequired(ctx, googleRedirectHandoffKey(handle), &handoff)
	if err != nil || !found {
		return nil, err
	}
	return &handoff, nil
}

func googleRedirectIntentKey(state string) string {
	return fmt.Sprintf("%s:intent:%s", googleRedirectKeyPrefix, state)
}

func googleRedirectHandoffKey(handle string) string {
	return fmt.Sprintf("%s:handoff:%s", googleRedirectKeyPrefix, handle)
}
