package chainrpc

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
	tron "github.com/dujiao-next/internal/modules/payment/infrastructure/tronusdt"
)

func TestConfigurationRejectsUnsafeOrAmbiguousFallbacks(t *testing.T) {
	for _, raw := range []string{
		`[{"url":"http://node.example"}]`,
		`[{"url":"https://user:secret@node.example"}]`,
		`[{"url":"https://node.example","unknown":true}]`,
		`[{"url":"https://node.example"}] {}`,
		`[{"url":"https://node.example"},{"url":"https://node.example"}]`,
		`[{}, {}, {}, {}]`,
		`[{"url":"https://node.example","api_key_env":"JWT_SECRET"}]`,
	} {
		if _, err := Configure("tron", &fakeReader{}, raw, func(string) string { return "secret" }, nil); err != chain.ErrRPC {
			t.Fatal("accepted unsafe configuration", err)
		}
	}
	r := &fakeReader{}
	for _, raw := range []string{"", "[]", "null"} {
		got, err := Configure("bsc", r, raw, nil, nil)
		if err != nil || got != r {
			t.Fatal("changed legacy single-node behavior")
		}
	}
}

func TestTRONBackupCredentialsAreExplicitAndIsolated(t *testing.T) {
	for _, keyEnv := range []string{"", "TRON_BACKUP_API_KEY"} {
		t.Run(keyEnv, func(t *testing.T) {
			s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				want := ""
				if keyEnv != "" {
					want = "test-backup-token"
				}
				if got := r.Header.Get("TRON-PRO-API-KEY"); got != want {
					t.Error("credential forwarded to wrong provider")
				}
				switch r.URL.Path {
				case "/walletsolidity/getblockbynum":
					json.NewEncoder(w).Encode(map[string]any{"blockID": tron.MainnetGenesis, "block_header": map[string]any{"raw_data": map[string]int{"number": 0}}})
				case "/wallet/triggerconstantcontract":
					json.NewEncoder(w).Encode(map[string]any{"result": map[string]bool{"result": true}, "constant_result": []string{"0000000000000000000000000000000000000000000000000000000000000006"}})
				default:
					t.Error("unexpected API")
				}
			}))
			defer s.Close()
			old := http.DefaultTransport
			http.DefaultTransport = s.Client().Transport
			defer func() { http.DefaultTransport = old }()
			raw, _ := json.Marshal([]Endpoint{{URL: s.URL, APIKeyEnv: keyEnv}})
			reader, err := Configure("tron", &fakeReader{checkErr: chain.ErrRPC}, string(raw), func(name string) string {
				if name != "TRON_BACKUP_API_KEY" {
					t.Error("read unrelated credential")
				}
				return "test-backup-token"
			}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if err = reader.Check(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRealEVMProtocolRejectsWrongChainThenUsesBackup(t *testing.T) {
	makeServer := func(id string) *httptest.Server {
		return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var req struct {
				Method string `json:"method"`
			}
			json.NewDecoder(r.Body).Decode(&req)
			var result any
			switch req.Method {
			case "eth_chainId":
				result = id
			case "eth_call":
				result = "0x0000000000000000000000000000000000000000000000000000000000000012"
			case "eth_getLogs":
				result = []any{}
			default:
				t.Error("unexpected method", req.Method)
			}
			json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": 1, "result": result})
		}))
	}
	wrong, good := makeServer("0x1"), makeServer("0x38")
	defer wrong.Close()
	defer good.Close()
	roots := x509.NewCertPool()
	roots.AddCert(wrong.Certificate())
	roots.AddCert(good.Certificate())
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = wrong.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	transport.TLSClientConfig.RootCAs = roots
	old := http.DefaultTransport
	http.DefaultTransport = transport
	defer func() { http.DefaultTransport = old; transport.CloseIdleConnections() }()
	raw, _ := json.Marshal([]Endpoint{{URL: wrong.URL}, {URL: good.URL}})
	reader, err := Configure("bsc", &fakeReader{checkErr: chain.ErrRPC}, string(raw), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = reader.Logs(context.Background(), 100, 101, "0x66a8bab15067eab1efc948ac03684f0397c8416f"); err != nil {
		t.Fatal(err)
	}
	if reader.(*Pool).active != 2 {
		t.Fatal("wrong-chain endpoint selected")
	}
}
