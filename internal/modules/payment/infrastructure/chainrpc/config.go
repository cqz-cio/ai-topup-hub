package chainrpc

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
	evm "github.com/dujiao-next/internal/modules/payment/infrastructure/bscusdt"
	tron "github.com/dujiao-next/internal/modules/payment/infrastructure/tronusdt"
)

// Endpoint describes one provider, optionally using separate EVM APIs for
// logs/receipts. APIKeyEnv is TRON-only and never inherited from the primary.
type Endpoint struct {
	URL        string `json:"url"`
	LogsURL    string `json:"logs_url,omitempty"`
	ReceiptURL string `json:"receipt_url,omitempty"`
	APIKeyEnv  string `json:"api_key_env,omitempty"`
}

// Configure is opt-in: absent fallbacks preserve the original reader.
// No defaults silently enable third-party providers in an existing deployment.
func Configure(network string, primary chain.Reader, raw string, getenv func(string) string, notify func(Event)) (chain.Reader, error) {
	if strings.TrimSpace(raw) == "" {
		return primary, nil
	}
	var endpoints []Endpoint
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&endpoints) != nil || len(endpoints) > 3 {
		return nil, chain.ErrRPC
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, chain.ErrRPC
	}
	if len(endpoints) == 0 {
		return primary, nil
	}
	readers := []chain.Reader{primary}
	seen := map[string]bool{}
	for _, e := range endpoints {
		e.URL = strings.TrimSpace(e.URL)
		if seen[e.URL] {
			return nil, chain.ErrRPC
		}
		seen[e.URL] = true
		var r chain.Reader
		var err error
		if network == "tron" {
			if e.LogsURL != "" || e.ReceiptURL != "" {
				return nil, chain.ErrRPC
			}
			key := ""
			if e.APIKeyEnv != "" {
				// Only explicitly named TRON credential variables are read.
				if !strings.HasPrefix(e.APIKeyEnv, "TRON_") || !strings.HasSuffix(e.APIKeyEnv, "_API_KEY") || getenv == nil {
					return nil, chain.ErrRPC
				}
				key = getenv(e.APIKeyEnv)
				if key == "" {
					return nil, chain.ErrRPC
				}
			}
			r, err = tron.NewRPC(e.URL, key)
		} else {
			if e.APIKeyEnv != "" {
				return nil, chain.ErrRPC
			}
			r, err = evm.NewRPCWithEndpoints(e.URL, e.LogsURL, e.ReceiptURL, network)
		}
		if err != nil {
			return nil, chain.ErrRPC
		}
		readers = append(readers, r)
	}
	return New(readers, notify)
}
