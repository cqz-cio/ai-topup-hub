package settingsapp

import (
	"reflect"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestRegistryNormalizesRegisteredKeysAndPassesUnknownValuesThrough(t *testing.T) {
	registry, err := NewRegistry(
		Definition{
			Key: "site_config",
			Normalize: func(value jsonmap.JSON) jsonmap.JSON {
				return jsonmap.JSON{"normalized": value["raw"]}
			},
		},
	)
	if err != nil {
		t.Fatalf("create registry: %v", err)
	}

	registered := jsonmap.JSON{"raw": "value"}
	if got := registry.Normalize("site_config", registered); !reflect.DeepEqual(got, jsonmap.JSON{"normalized": "value"}) {
		t.Fatalf("registered normalization mismatch: %#v", got)
	}

	unknown := jsonmap.JSON{"keep": "unchanged"}
	if got := registry.Normalize("custom_config", unknown); !reflect.DeepEqual(got, unknown) {
		t.Fatalf("unknown settings must pass through unchanged: %#v", got)
	}
}

func TestRegistryRejectsInvalidDefinitions(t *testing.T) {
	tests := []struct {
		name        string
		definitions []Definition
		wantError   string
	}{
		{
			name: "empty key",
			definitions: []Definition{{
				Normalize: func(value jsonmap.JSON) jsonmap.JSON { return value },
			}},
			wantError: "key",
		},
		{
			name: "surrounding whitespace",
			definitions: []Definition{{
				Key:       " site_config ",
				Normalize: func(value jsonmap.JSON) jsonmap.JSON { return value },
			}},
			wantError: "whitespace",
		},
		{
			name: "missing capabilities",
			definitions: []Definition{{
				Key: "site_config",
			}},
			wantError: "capability",
		},
		{
			name: "duplicate key",
			definitions: []Definition{
				{Key: "site_config", Normalize: func(value jsonmap.JSON) jsonmap.JSON { return value }},
				{Key: "site_config", Normalize: func(value jsonmap.JSON) jsonmap.JSON { return value }},
			},
			wantError: "duplicate",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRegistry(test.definitions...)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected error containing %q, got %v", test.wantError, err)
			}
		})
	}
}

func TestRegistrySupportsEffectOnlyDefinitionsAndReturnsDetachedEffects(t *testing.T) {
	declaredEffects := []Effect{EffectInvalidatePublicConfigCache}
	registry := MustNewRegistry(Definition{
		Key:     "wallet_config",
		Effects: declaredEffects,
	})
	declaredEffects[0] = Effect("mutated_after_construction")
	input := jsonmap.JSON{"wallet_only_payment": true}

	if got := registry.Normalize("wallet_config", input); !reflect.DeepEqual(got, input) {
		t.Fatalf("effect-only definition must pass values through: %#v", got)
	}

	effects := registry.Effects("wallet_config")
	if !reflect.DeepEqual(effects, []Effect{EffectInvalidatePublicConfigCache}) {
		t.Fatalf("effects mismatch: %#v", effects)
	}
	effects[0] = Effect("mutated")
	if got := registry.Effects("wallet_config"); !reflect.DeepEqual(got, []Effect{EffectInvalidatePublicConfigCache}) {
		t.Fatalf("registry effects leaked mutable state: %#v", got)
	}
	if got := registry.Effects("unknown_config"); got != nil {
		t.Fatalf("unknown setting must not emit effects: %#v", got)
	}
}

func TestRegistryRejectsInvalidEffects(t *testing.T) {
	identity := func(value jsonmap.JSON) jsonmap.JSON { return value }
	tests := []struct {
		name       string
		definition Definition
		wantError  string
	}{
		{
			name: "empty effect",
			definition: Definition{
				Key:       "site_config",
				Normalize: identity,
				Effects:   []Effect{""},
			},
			wantError: "empty effect",
		},
		{
			name: "duplicate effect",
			definition: Definition{
				Key:       "site_config",
				Normalize: identity,
				Effects: []Effect{
					EffectInvalidatePublicConfigCache,
					EffectInvalidatePublicConfigCache,
				},
			},
			wantError: "duplicate effect",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := NewRegistry(test.definition)
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("expected error containing %q, got %v", test.wantError, err)
			}
		})
	}
}

func TestRegistryKeysAreSortedAndDetached(t *testing.T) {
	identity := func(value jsonmap.JSON) jsonmap.JSON { return value }
	registry := MustNewRegistry(
		Definition{Key: "zeta", Normalize: identity},
		Definition{Key: "alpha", Normalize: identity},
	)

	keys := registry.Keys()
	if !reflect.DeepEqual(keys, []string{"alpha", "zeta"}) {
		t.Fatalf("sorted keys mismatch: %#v", keys)
	}
	keys[0] = "mutated"
	if got := registry.Keys(); !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("registry keys leaked mutable state: %#v", got)
	}
}
