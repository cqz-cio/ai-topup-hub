package bscusdt

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

func TestRPCSeparateReceiptEndpointRoutesAndValidatesBothNetworks(t *testing.T) {
	for _, wrongReceiptChain := range []bool{false, true} {
		t.Run(map[bool]string{false: "mainnet", true: "wrong_receipt_network"}[wrongReceiptChain], func(t *testing.T) {
			var mu sync.Mutex
			calls := map[string][]string{}
			serverFor := func(name string, wrongChain bool) *httptest.Server {
				return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var request struct {
						Method string `json:"method"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						return
					}
					mu.Lock()
					calls[name] = append(calls[name], request.Method)
					mu.Unlock()
					var result interface{} = "0x38"
					switch request.Method {
					case "eth_chainId":
						if wrongChain {
							result = "0x1"
						}
					case "eth_call":
						result = "0x" + strings.Repeat("0", 62) + "12"
					case "eth_getLogs":
						result = []interface{}{}
					case "eth_getTransactionReceipt":
						result = map[string]string{"status": "0x1", "transactionHash": "0x" + strings.Repeat("a", 64)}
					}
					json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result})
				}))
			}
			primary := serverFor("primary", false)
			defer primary.Close()
			receipts := serverFor("receipts", wrongReceiptChain)
			defer receipts.Close()
			rpc, err := NewRPCWithReceiptEndpoint(primary.URL, receipts.URL)
			if err != nil {
				t.Fatal(err)
			}
			rpc.client = primary.Client()
			if err = rpc.Check(context.Background()); (err != nil) != wrongReceiptChain {
				t.Fatalf("unexpected network validation: %v", err)
			}
			if wrongReceiptChain {
				return
			}
			if _, err = rpc.Logs(context.Background(), 1, 2, "0x"+strings.Repeat("a", 40)); err != nil {
				t.Fatal(err)
			}
			if _, err = rpc.Receipt(context.Background(), "0x"+strings.Repeat("a", 64)); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if got := strings.Join(calls["primary"], ","); got != "eth_chainId,eth_call,eth_getLogs" {
				t.Fatal("primary routing", got)
			}
			if got := strings.Join(calls["receipts"], ","); got != "eth_chainId,eth_call,eth_getTransactionReceipt" {
				t.Fatal("receipt routing", got)
			}
		})
	}
}

func TestRPCRejectsUnsafeReceiptEndpoint(t *testing.T) {
	for _, endpoint := range []string{"http://rpc.test", "https://user:pass@rpc.test", "https://rpc.test/#fragment"} {
		if _, err := NewRPCWithReceiptEndpoint("https://rpc.test", endpoint); err == nil {
			t.Fatal("unsafe receipt RPC URL accepted")
		}
	}
}

func TestRPCReceiptEndpointRejectsRedirectsAndHidesProviderErrors(t *testing.T) {
	for _, kind := range []string{"redirect", "provider_error"} {
		t.Run(kind, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if kind == "redirect" {
					http.Redirect(w, r, "https://example.invalid/must-not-leak-api-secret", http.StatusFound)
					return
				}
				json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "error": map[string]string{"message": "must-not-leak-api-secret"}})
			}))
			defer server.Close()
			rpc, err := NewRPCWithReceiptEndpoint("https://primary.invalid", server.URL)
			if err != nil {
				t.Fatal(err)
			}
			rpc.client.Transport = server.Client().Transport
			if _, err = rpc.Receipt(context.Background(), "0x"+strings.Repeat("a", 64)); err != ErrRPC {
				t.Fatal("receipt error was not safely rejected", err)
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
