package bscusdt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
)

func TestPlasmaRPCChecksNetworkTokenAndSixDecimals(t *testing.T) {
	for _, kind := range []string{"ok", "bsc_chain", "bsc_decimals"} {
		t.Run(kind, func(t *testing.T) {
			network, _ := chain.ResolveNetwork("plasma")
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Method string            `json:"method"`
					Params []json.RawMessage `json:"params"`
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				var result any
				switch request.Method {
				case "eth_chainId":
					result = "0x2611"
					if kind == "bsc_chain" {
						result = "0x38"
					}
				case "eth_call":
					var call map[string]string
					if err := json.Unmarshal(request.Params[0], &call); err != nil || call["to"] != network.Token || call["data"] != "0x313ce567" {
						t.Error("wrong token decimals call")
					}
					result = fmt.Sprintf("0x%064x", 6)
					if kind == "bsc_decimals" {
						result = fmt.Sprintf("0x%064x", 18)
					}
				case "eth_getLogs":
					var filter struct {
						Address string `json:"address"`
						Topics  []any  `json:"topics"`
					}
					if err := json.Unmarshal(request.Params[0], &filter); err != nil || filter.Address != network.Token || len(filter.Topics) != 3 || filter.Topics[0] != TransferTopic || filter.Topics[2] != addressTopic("0x"+strings.Repeat("a", 40)) {
						t.Error("wrong Plasma log filter")
					}
					result = []any{}
				default:
					t.Error("unexpected RPC method", request.Method)
				}
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
			}))
			defer server.Close()
			rpc, err := NewRPCForNetwork(server.URL, "", "plasma")
			if err != nil {
				t.Fatal(err)
			}
			rpc.client = server.Client()
			if err = rpc.Check(context.Background()); (err == nil) != (kind == "ok") {
				t.Fatal("incorrect Plasma RPC validation", err)
			}
			if kind == "ok" {
				if _, err = rpc.Logs(context.Background(), 1, 2, "0x"+strings.Repeat("a", 40)); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestRPCRejectsUnsupportedNetwork(t *testing.T) {
	if _, err := NewRPCForNetwork("https://rpc.example.test", "", "unknown"); err == nil {
		t.Fatal("unsupported network accepted")
	}
}
