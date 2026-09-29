package container

import (
	"errors"
	"testing"

	"github.com/dujiao-next/internal/config"
	"github.com/dujiao-next/internal/constants"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type googleRuntimeSettingStore struct {
	value jsonmap.JSON
	found bool
	err   error
}

func (s *googleRuntimeSettingStore) GetByKey(key string) (jsonmap.JSON, bool, error) {
	if key != constants.SettingKeyGoogleAuthConfig {
		return nil, false, nil
	}
	return s.value, s.found, s.err
}

func (*googleRuntimeSettingStore) Upsert(string, jsonmap.JSON) (jsonmap.JSON, error) {
	return nil, errors.New("unexpected upsert")
}

func TestLoadRuntimeSettingsGoogleAuthThreeStateFailClosed(t *testing.T) {
	tests := []struct {
		name        string
		store       *googleRuntimeSettingStore
		wantEnabled bool
		wantClient  string
	}{
		{
			name: "database disabled overrides enabled yaml",
			store: &googleRuntimeSettingStore{
				found: true,
				value: jsonmap.JSON{"enabled": false, "client_id": "database-client"},
			},
			wantEnabled: false,
			wantClient:  "database-client",
		},
		{
			name:        "missing database setting keeps yaml",
			store:       &googleRuntimeSettingStore{},
			wantEnabled: true,
			wantClient:  "yaml-client",
		},
		{
			name: "database read failure disables yaml fallback",
			store: &googleRuntimeSettingStore{
				err: errors.New("database unavailable"),
			},
			wantEnabled: false,
			wantClient:  "yaml-client",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			container := &Container{
				Config: &config.Config{
					GoogleAuth: config.GoogleAuthConfig{
						Enabled:  true,
						ClientID: "yaml-client",
					},
				},
				SettingService: settingsapp.NewService(test.store),
			}
			container.loadRuntimeSettings()
			if container.Config.GoogleAuth.Enabled != test.wantEnabled {
				t.Fatalf("enabled = %v, want %v", container.Config.GoogleAuth.Enabled, test.wantEnabled)
			}
			if container.Config.GoogleAuth.ClientID != test.wantClient {
				t.Fatalf("client ID = %q, want %q", container.Config.GoogleAuth.ClientID, test.wantClient)
			}
		})
	}
}
