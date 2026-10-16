package bscusdt

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
)

func TestRPCSeparateLogsEndpointAndApplicationIdentity(t *testing.T) {
	for _, badNetwork := range []bool{false, true} {
		t.Run(fmt.Sprint(badNetwork), func(t *testing.T) {
			calls := map[string][]string{}
			makeServer := func(name string) *httptest.Server {
				return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.UserAgent() != "PiStarPaymentMonitor/1.0" || r.Header.Get("Content-Type") != "application/json" {
						http.Error(w, "client identity required", http.StatusForbidden)
						return
					}
					var request struct {
						Method string `json:"method"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						return
					}
					calls[name] = append(calls[name], request.Method)
					var result any = "0x38"
					switch request.Method {
					case "eth_chainId":
						if name == "logs" && badNetwork {
							result = "0x1"
						}
					case "eth_call":
						result = fmt.Sprintf("0x%064x", 18)
					case "eth_getLogs":
						if name != "logs" {
							t.Error("logs reached header or receipt provider")
						}
						result = []any{}
					case "eth_getTransactionReceipt":
						if name != "primary" {
							t.Error("receipt reached logs provider")
						}
						result = map[string]string{"status": "0x1"}
					}
					json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
				}))
			}
			primary := makeServer("primary")
			defer primary.Close()
			logs := makeServer("logs")
			defer logs.Close()
			rpc, err := NewRPCWithEndpoints(primary.URL, logs.URL, primary.URL, "bsc")
			if err != nil {
				t.Fatal(err)
			}
			rpc.client = primary.Client()
			if err = rpc.Check(context.Background()); (err != nil) != badNetwork {
				t.Fatalf("wrong network validation: %v", err)
			}
			if badNetwork {
				return
			}
			if _, err = rpc.Logs(context.Background(), 1, 2, "0x"+strings.Repeat("a", 40)); err != nil {
				t.Fatal(err)
			}
			if _, err = rpc.Receipt(context.Background(), "0x"+strings.Repeat("b", 64)); err != nil {
				t.Fatal(err)
			}
			if strings.Join(calls["primary"], ",") != "eth_chainId,eth_call,eth_getTransactionReceipt" {
				t.Fatal(calls)
			}
			if strings.Join(calls["logs"], ",") != "eth_chainId,eth_call,eth_getLogs" {
				t.Fatal(calls)
			}
		})
	}
}

func TestRPCLogPagesAreContiguousAndFailClosed(t *testing.T) {
	for _, kind := range []string{"ok", "second_page_failure", "out_of_page", "null", "redirect", "inverted", "uint64_end"} {
		t.Run(kind, func(t *testing.T) {
			var ranges [][2]uint64
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request struct {
					Method string
					Params []struct {
						FromBlock string
						ToBlock   string
						Address   string
						Topics    []any
					}
				}
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					return
				}
				if request.Method != "eth_getLogs" || len(request.Params) != 1 {
					t.Error("unexpected request")
					return
				}
				filter := request.Params[0]
				from, _ := quantity(filter.FromBlock)
				to, _ := quantity(filter.ToBlock)
				if filter.Address != Token || len(filter.Topics) != 3 || filter.Topics[0] != TransferTopic || filter.Topics[2] != addressTopic("0x"+strings.Repeat("a", 40)) {
					t.Error("changed transfer filter")
				}
				if to < from || to-from >= 50 {
					t.Error("provider page limit exceeded")
				}
				ranges = append(ranges, [2]uint64{from, to})
				if kind == "second_page_failure" && len(ranges) == 2 {
					http.Error(w, "secret must not escape", 403)
					return
				}
				if kind == "redirect" {
					http.Redirect(w, r, "https://example.invalid/secret", 302)
					return
				}
				block := from
				if kind == "out_of_page" {
					block = to + 1
				}
				var result any = []chain.Log{{BlockNumber: hexBlock(block)}}
				if kind == "null" {
					result = nil
				}
				json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
			}))
			defer server.Close()
			rpc, _ := NewRPCWithEndpoints("https://header.invalid", server.URL, "", "bsc")
			rpc.client.Transport = server.Client().Transport
			from, to := uint64(100), uint64(200)
			if kind == "inverted" {
				to = 99
			}
			if kind == "uint64_end" {
				from = math.MaxUint64 - 50
				to = math.MaxUint64
			}
			logs, err := rpc.Logs(context.Background(), from, to, "0x"+strings.Repeat("a", 40))
			if kind == "ok" {
				if err != nil || len(logs) != 3 || !reflect.DeepEqual(ranges, [][2]uint64{{100, 149}, {150, 199}, {200, 200}}) {
					t.Fatalf("lost or duplicated page: %v %v", ranges, err)
				}
			} else if kind == "uint64_end" {
				if err != nil || len(logs) != 2 || len(ranges) != 2 || ranges[1] != [2]uint64{math.MaxUint64, math.MaxUint64} {
					t.Fatal("overflowed page range", ranges, err)
				}
			} else if err != ErrRPC || logs != nil {
				t.Fatalf("partial or invalid scan accepted: %#v %v", logs, err)
			}
			if kind == "inverted" && len(ranges) != 0 {
				t.Fatal("invalid range made a network request")
			}
		})
	}
}

func TestRPCRejectsUnsafeLogsEndpoint(t *testing.T) {
	for _, endpoint := range []string{"http://rpc.test", "https://user:pass@rpc.test", "https://rpc.test/#fragment"} {
		if _, err := NewRPCWithEndpoints("https://rpc.test", endpoint, "", "bsc"); err == nil {
			t.Fatal("unsafe logs endpoint accepted")
		}
	}
}
