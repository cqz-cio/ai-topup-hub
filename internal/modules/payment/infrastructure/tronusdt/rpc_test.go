package tronusdt

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/dujiao-next/internal/modules/payment/contract/chain"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Explicitly opt-in: reads public mainnet blocks only, never creates an invoice,
// signs a transaction, or triggers fulfillment.
func TestReadOnlyMainnetProtocol(t *testing.T) {
	if os.Getenv("TRON_READONLY_PROBE") != "1" {
		t.Skip("read-only mainnet probe is opt-in")
	}
	rpc, err := NewRPC(DefaultEndpoint, os.Getenv("TRON_API_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	if err = rpc.Check(ctx); err != nil {
		t.Fatal("mainnet identity/decimals", err)
	}
	h, err := rpc.Header(ctx, "finalized")
	if err != nil {
		t.Fatal(err)
	}
	n, _ := chain.Quantity(h.Number)
	address, _ := chain.TronAddressBase58("0x" + strings.Repeat("6", 40))
	logs, err := rpc.Logs(ctx, n, n, address)
	if err != nil {
		t.Fatal("solidified block receipts", err)
	}
	var infos []transactionInfo
	if err = rpc.call(ctx, "/walletsolidity/gettransactioninfobyblocknum", map[string]any{"num": n}, &infos); err != nil {
		t.Fatal(err)
	}
	verified := false
	for _, info := range infos {
		if info.Receipt.Result != "SUCCESS" {
			continue
		}
		for _, event := range info.Logs {
			if event.Address != tokenHex || len(event.Topics) != 3 || event.Topics[0] != chain.TransferTopic[2:] {
				continue
			}
			receiver, e := chain.TronAddressBase58("0x" + event.Topics[2][24:])
			if e != nil {
				t.Fatal(e)
			}
			matched, e := rpc.Logs(ctx, n, n, receiver)
			if e != nil || len(matched) == 0 {
				t.Fatal("real USDT event was not detected", e)
			}
			receipt, e := rpc.Receipt(ctx, "0x"+info.ID)
			if e != nil || receipt.Status != "0x1" {
				t.Fatal("real USDT receipt rejected", e)
			}
			verified = true
			break
		}
		if verified {
			break
		}
	}
	if !verified {
		t.Fatal("no successful USDT event in sampled block")
	}
	t.Logf("read-only mainnet confirmed: block=%d hash=%s matching_events=%d public_receipt_verified=true", n, h.Hash, len(logs))
}

func rpcFixture(t *testing.T, kind string) (*RPC, string) {
	t.Helper()
	recipient, _ := chain.TronAddressBase58("0x" + strings.Repeat("6", 40))
	stamp := time.Now().UnixMilli()
	blockID := fmt.Sprintf("%016x", 100) + strings.Repeat("a", 48)
	txID := strings.Repeat("b", 64)
	b := block{ID: blockID}
	b.Header.Raw.Number = 100
	b.Header.Raw.Timestamp = stamp
	b.Header.Raw.ParentHash = fmt.Sprintf("%016x", 99) + strings.Repeat("a", 48)
	b.Transactions = append(b.Transactions, struct {
		ID string `json:"txID"`
	}{txID})
	info := transactionInfo{ID: txID, BlockNumber: 100, Timestamp: stamp}
	info.Receipt.Result = "SUCCESS"
	event := struct {
		Address string   `json:"address"`
		Topics  []string `json:"topics"`
		Data    string   `json:"data"`
	}{tokenHex, []string{chain.TransferTopic[2:], strings.Repeat("0", 24) + strings.Repeat("1", 40), chain.AddressTopic(recipient)[2:]}, fmt.Sprintf("%064x", 15040000)}
	info.Logs = append(info.Logs, event)
	switch kind {
	case "fake_token":
		info.Logs[0].Address = strings.Repeat("f", 40)
	case "wrong_recipient":
		info.Logs[0].Topics[2] = strings.Repeat("0", 24) + strings.Repeat("9", 40)
	case "failed":
		info.Receipt.Result = "REVERT"
	case "malformed_amount":
		info.Logs[0].Data = "not-an-integer"
	case "malformed_topic":
		info.Logs[0].Topics[1] = "short"
	case "wrong_block":
		info.BlockNumber = 101
	case "wrong_time":
		info.Timestamp++
	case "foreign_transaction":
		info.ID = strings.Repeat("c", 64)
	case "two_events":
		info.Logs = append(info.Logs, event)
	}
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("TRON-PRO-API-KEY") != "test-node-key" {
			t.Error("incorrect node request")
		}
		if kind == "rate_limit" {
			w.WriteHeader(429)
			return
		}
		if kind == "api_error" {
			json.NewEncoder(w).Encode(map[string]any{"Error": "redacted-node-error"})
			return
		}
		if kind == "redirect" {
			http.Redirect(w, r, "https://example.test/", 302)
			return
		}
		var p map[string]json.RawMessage
		json.NewDecoder(r.Body).Decode(&p)
		var response any
		switch r.URL.Path {
		case "/walletsolidity/getblockbynum":
			if kind == "missing_block" {
				json.NewEncoder(w).Encode(map[string]any{})
				return
			}
			if string(p["num"]) == "0" {
				id := MainnetGenesis
				if kind == "wrong_network" {
					id = strings.Repeat("f", 64)
				}
				response = map[string]any{"blockID": id}
			} else {
				response = b
			}
		case "/wallet/triggerconstantcontract":
			if string(p["contract_address"]) != "\""+chain.TronUSDTContract+"\"" || string(p["function_selector"]) != "\"decimals()\"" {
				t.Error("wrong token queried")
			}
			decimals := 6
			if kind == "wrong_decimals" {
				decimals = 18
			}
			response = map[string]any{"result": map[string]any{"result": true}, "constant_result": []string{fmt.Sprintf("%064x", decimals)}}
		case "/wallet/getnowblock", "/walletsolidity/getnowblock":
			response = b
		case "/walletsolidity/getblockbylimitnext":
			if string(p["startNum"]) != "100" || string(p["endNum"]) != "101" {
				t.Error("wrong scan range")
			}
			blocks := []block{b}
			if kind == "missing_block" {
				blocks = nil
			}
			response = map[string]any{"block": blocks}
		case "/walletsolidity/gettransactioninfobyblocknum":
			infos := []transactionInfo{info}
			if kind == "missing_receipt" {
				infos = nil
			}
			if kind == "duplicate_receipt" {
				infos = append(infos, info)
			}
			response = infos
		case "/walletsolidity/gettransactioninfobyid":
			response = info
		default:
			t.Errorf("unexpected node method %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(response)
	}))
	t.Cleanup(server.Close)
	rpc, err := NewRPC(server.URL, "test-node-key")
	if err != nil {
		t.Fatal(err)
	}
	rpc.client.Transport = server.Client().Transport
	return rpc, recipient
}

func TestOnlyOfficialUSDTIncomingSuccessfulTransfers(t *testing.T) {
	for _, kind := range []string{"valid", "fake_token", "wrong_recipient", "failed", "two_events"} {
		t.Run(kind, func(t *testing.T) {
			rpc, address := rpcFixture(t, kind)
			ctx := context.Background()
			if err := rpc.Check(ctx); err != nil {
				t.Fatal(err)
			}
			logs, err := rpc.Logs(ctx, 100, 100, address)
			if err != nil {
				t.Fatal(err)
			}
			want := 0
			if kind == "valid" {
				want = 1
			}
			if kind == "two_events" {
				want = 2
			}
			if len(logs) != want {
				t.Fatalf("got %d transfers, want %d", len(logs), want)
			}
			for i, event := range logs {
				if event.Address != "0x"+tokenHex || event.LogIndex != chain.HexBlock(uint64(i)) || event.Topics[2] != chain.AddressTopic(address) || event.Data != fmt.Sprintf("0x%064x", 15040000) {
					t.Fatal("incorrect normalized transfer")
				}
				receipt, err := rpc.Receipt(ctx, event.TransactionHash)
				if err != nil || receipt.Status != "0x1" || receipt.BlockHash != event.BlockHash || len(receipt.Logs) != want {
					t.Fatal("receipt mismatch", err)
				}
			}
		})
	}
}
func TestIncompleteOrMalformedBlocksFailClosed(t *testing.T) {
	for _, kind := range []string{"missing_block", "missing_receipt", "duplicate_receipt", "foreign_transaction", "wrong_block", "wrong_time", "malformed_amount", "malformed_topic", "rate_limit", "api_error", "redirect"} {
		t.Run(kind, func(t *testing.T) {
			rpc, address := rpcFixture(t, kind)
			if _, err := rpc.Logs(context.Background(), 100, 100, address); err == nil {
				t.Fatal("invalid page accepted")
			}
		})
	}
}
func TestMainnetAndTokenPrecisionArePinned(t *testing.T) {
	for _, kind := range []string{"wrong_network", "wrong_decimals", "api_error", "rate_limit"} {
		t.Run(kind, func(t *testing.T) {
			rpc, _ := rpcFixture(t, kind)
			if rpc.Check(context.Background()) == nil {
				t.Fatal("invalid network accepted")
			}
		})
	}
}
func TestHeadersAndInputValidation(t *testing.T) {
	rpc, address := rpcFixture(t, "valid")
	for _, tag := range []string{"latest", "finalized", "0x64"} {
		h, err := rpc.Header(context.Background(), tag)
		if err != nil || h.Number != "0x64" {
			t.Fatal("invalid header", err)
		}
	}
	if _, err := rpc.Header(context.Background(), "0x65"); err == nil {
		t.Fatal("wrong block accepted")
	}
	for _, v := range []string{"http://example.test", "https://u:p@example.test", "https://example.test/#fragment", "https://example.test/?key=secret"} {
		if _, err := NewRPC(v, ""); err == nil {
			t.Fatal("unsafe endpoint accepted")
		}
	}
	if _, err := rpc.Logs(context.Background(), 100, 111, address); err == nil {
		t.Fatal("unbounded scan allowed")
	}
	if _, err := rpc.Logs(context.Background(), 100, 100, "invalid"); err == nil {
		t.Fatal("invalid address accepted")
	}
	if _, err := rpc.Receipt(context.Background(), "invalid"); err == nil {
		t.Fatal("invalid hash accepted")
	}
}
