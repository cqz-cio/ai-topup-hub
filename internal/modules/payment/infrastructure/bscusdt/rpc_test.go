package bscusdt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRPCValidatesChainAndTokenDecimals(t *testing.T) {
	if !isHex(TransferTopic, 32) {
		t.Fatal("invalid transfer event signature")
	}
	for _, kind := range []string{"ok", "wrong_chain", "wrong_decimals", "error", "null", "wrong_id"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Method string `json:"method"`
				}
				json.NewDecoder(r.Body).Decode(&request)
				result := "0x38"
				if request.Method == "eth_call" {
					result = "0x" + strings.Repeat("0", 62) + "12"
				}
				if kind == "wrong_chain" {
					result = "0x1"
				}
				if kind == "wrong_decimals" && request.Method == "eth_call" {
					result = "0x" + strings.Repeat("0", 63) + "6"
				}
				payload := map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result}
				if kind == "error" {
					payload["error"] = map[string]interface{}{"message": "must-not-leak-api-secret"}
				}
				if kind == "null" {
					payload["result"] = nil
				}
				if kind == "wrong_id" {
					payload["id"] = 2
				}
				json.NewEncoder(w).Encode(payload)
			}))
			defer server.Close()
			rpc, err := NewRPC(server.URL)
			if err != nil {
				t.Fatal(err)
			}
			rpc.client = server.Client()
			err = rpc.Check(context.Background())
			if (kind == "ok") != (err == nil) {
				t.Fatal("unexpected RPC result", err)
			}
			if err != nil && strings.Contains(err.Error(), "secret") {
				t.Fatal("RPC body leaked")
			}
		})
	}
}
func TestRPCRejectsInsecureEndpointsAndRedirects(t *testing.T) {
	for _, endpoint := range []string{"", "http://rpc.test", "https://user:pass@rpc.test", "https://rpc.test/#fragment"} {
		if _, err := NewRPC(endpoint); err == nil {
			t.Fatal("unsafe RPC URL accepted")
		}
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://example.invalid/secret", http.StatusFound)
	}))
	defer server.Close()
	rpc, _ := NewRPC(server.URL)
	rpc.client.Transport = server.Client().Transport
	if err := rpc.Check(context.Background()); err != ErrRPC {
		t.Fatal(err)
	}
}
