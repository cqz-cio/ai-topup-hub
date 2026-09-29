package publicconfighttp

import (
	"reflect"
	"testing"
)

type googleAuthPublicStub map[string]interface{}

func (s googleAuthPublicStub) PublicConfig() map[string]interface{} {
	return map[string]interface{}(s)
}

func TestResolveGoogleAuthPublicConfig(t *testing.T) {
	tests := []struct {
		name     string
		source   GoogleAuthPublic
		fallback GoogleAuthFallback
		want     map[string]interface{}
	}{
		{
			name:     "fallback enabled",
			fallback: GoogleAuthFallback{Enabled: true, ClientID: " fallback-client "},
			want:     map[string]interface{}{"enabled": true, "client_id": "fallback-client"},
		},
		{
			name:     "service overrides fallback",
			source:   googleAuthPublicStub{"enabled": true, "client_id": " runtime-client ", "unexpected": "private"},
			fallback: GoogleAuthFallback{Enabled: false, ClientID: "fallback-client"},
			want:     map[string]interface{}{"enabled": true, "client_id": "runtime-client"},
		},
		{
			name:     "missing client id fails closed",
			source:   googleAuthPublicStub{"enabled": true, "client_id": ""},
			fallback: GoogleAuthFallback{Enabled: true, ClientID: "fallback-client"},
			want:     map[string]interface{}{"enabled": false, "client_id": ""},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resolveGoogleAuthPublicConfig(test.source, test.fallback); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("public config = %#v, want %#v", got, test.want)
			}
		})
	}
}
