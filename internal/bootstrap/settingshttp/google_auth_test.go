package settingsbootstrap

import (
	"fmt"
	"sync"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	settingssecurity "github.com/dujiao-next/internal/modules/settings/schema/security"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type googleAuthSettingStore struct {
	mu    sync.RWMutex
	store map[string]jsonmap.JSON
}

func (s *googleAuthSettingStore) GetByKey(key string) (jsonmap.JSON, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.store[key]
	if !ok {
		return nil, false, nil
	}
	copyValue := make(jsonmap.JSON, len(value))
	for field, item := range value {
		copyValue[field] = item
	}
	return copyValue, true, nil
}

func (s *googleAuthSettingStore) Upsert(key string, value jsonmap.JSON) (jsonmap.JSON, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	copyValue := make(jsonmap.JSON, len(value))
	for field, item := range value {
		copyValue[field] = item
	}
	s.store[key] = copyValue
	return copyValue, nil
}

type googleAuthRuntimeStub struct {
	mu  sync.RWMutex
	cfg config.GoogleAuthConfig
}

func (s *googleAuthRuntimeStub) SetConfig(cfg config.GoogleAuthConfig) {
	s.mu.Lock()
	s.cfg = cfg
	s.mu.Unlock()
}

func (s *googleAuthRuntimeStub) snapshot() config.GoogleAuthConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

func TestSettingsGoogleAuthAdapterKeepsStartupFallbackImmutable(t *testing.T) {
	const fallbackClientID = "startup.apps.googleusercontent.com"
	cfg := &config.Config{GoogleAuth: config.GoogleAuthConfig{
		Enabled:  false,
		ClientID: fallbackClientID,
	}}
	store := &googleAuthSettingStore{store: make(map[string]jsonmap.JSON)}
	runtime := &googleAuthRuntimeStub{}
	adapter := settingsGoogleAuthAdapter{
		settings:   settingsapp.NewService(store),
		cfg:        cfg,
		googleAuth: runtime,
	}

	var wait sync.WaitGroup
	for index := 0; index < 64; index++ {
		index := index
		wait.Add(3)
		go func() {
			defer wait.Done()
			if _, err := adapter.GetGoogleAuthSetting(); err != nil {
				t.Errorf("get setting: %v", err)
			}
		}()
		go func() {
			defer wait.Done()
			enabled := true
			clientID := fmt.Sprintf("client-%d.apps.googleusercontent.com", index)
			if _, err := adapter.PatchGoogleAuthSetting(settingssecurity.GoogleAuthSettingPatch{
				Enabled:  &enabled,
				ClientID: &clientID,
			}); err != nil {
				t.Errorf("patch setting: %v", err)
			}
		}()
		go func() {
			defer wait.Done()
			adapter.ApplyRuntime(settingssecurity.GoogleAuthSetting{
				Enabled:  true,
				ClientID: fmt.Sprintf("runtime-%d.apps.googleusercontent.com", index),
			})
		}()
	}
	wait.Wait()

	if cfg.GoogleAuth.Enabled || cfg.GoogleAuth.ClientID != fallbackClientID {
		t.Fatalf("startup fallback was mutated: %#v", cfg.GoogleAuth)
	}
	if _, ok, err := store.GetByKey(constants.SettingKeyGoogleAuthConfig); err != nil || !ok {
		t.Fatalf("database setting missing: ok=%v err=%v", ok, err)
	}

	final := settingssecurity.GoogleAuthSetting{Enabled: true, ClientID: "final.apps.googleusercontent.com"}
	adapter.ApplyRuntime(final)
	if got := runtime.snapshot(); !got.Enabled || got.ClientID != final.ClientID {
		t.Fatalf("runtime config = %#v", got)
	}
}
