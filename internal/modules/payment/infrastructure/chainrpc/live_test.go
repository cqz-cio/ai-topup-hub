package chainrpc

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
)

// Explicit opt-in only. Uses public historical transfers and read-only APIs;
// never opens the shop database or constructs/broadcasts a transaction.
func TestLiveReadOnlyFallbackProfiles(t *testing.T) {
	path := os.Getenv("CHAIN_RPC_PROBE_FILE")
	if path == "" {
		t.Skip("read-only external probe requires CHAIN_RPC_PROBE_FILE")
	}
	var profiles []struct {
		Name     string   `json:"name"`
		Network  string   `json:"network"`
		Endpoint Endpoint `json:"endpoint"`
		Tx       string   `json:"tx"`
	}
	raw, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(raw, &profiles) != nil || len(profiles) == 0 {
		t.Fatal("invalid probe file")
	}
	for _, profile := range profiles {
		t.Run(profile.Name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			raw, _ := json.Marshal([]Endpoint{profile.Endpoint})
			var switches int
			reader, err := Configure(profile.Network, &fakeReader{checkErr: chain.ErrRPC}, string(raw), os.Getenv, func(e Event) {
				if e.Kind == "switched" {
					switches++
				}
			})
			if err != nil {
				t.Fatal("invalid profile")
			}
			if err = reader.Check(ctx); err != nil {
				t.Fatal("identity", err)
			}
			final, err := reader.Header(ctx, "finalized")
			if err != nil {
				t.Fatal("finalized", err)
			}
			latest, err := reader.Header(ctx, "latest")
			if err != nil {
				t.Fatal("latest", err)
			}
			f, _ := chain.Quantity(final.Number)
			h, _ := chain.Quantity(latest.Number)
			if f > h {
				t.Fatal("finality exceeds head")
			}
			tx := profile.Tx
			if !strings.HasPrefix(tx, "0x") {
				tx = "0x" + tx
			}
			receipt, err := reader.Receipt(ctx, tx)
			if err != nil || receipt == nil || receipt.Status != "0x1" || !strings.EqualFold(receipt.TransactionHash, tx) {
				t.Fatal("historical receipt", err)
			}
			n, _ := chain.ResolveNetwork(profile.Network)
			var event *chain.Log
			for i := range receipt.Logs {
				l := &receipt.Logs[i]
				if strings.EqualFold(l.Address, n.Token) && len(l.Topics) == 3 && l.Topics[0] == chain.TransferTopic {
					event = l
					break
				}
			}
			if event == nil {
				t.Fatal("no sample USDT transfer")
			}
			block, err := reader.Header(ctx, event.BlockNumber)
			if err != nil || block.Hash != event.BlockHash {
				t.Fatal("historical inclusion", err)
			}
			number, _ := chain.Quantity(event.BlockNumber)
			recipient := n.DisplayAddress("0x" + event.Topics[2][26:])
			logs, err := reader.Logs(ctx, number, number, recipient)
			if err != nil {
				t.Fatal("historical logs", err)
			}
			found := false
			for _, l := range logs {
				if l.TransactionHash == tx && l.LogIndex == event.LogIndex && l.BlockHash == event.BlockHash && l.Data == event.Data {
					found = true
				}
			}
			if !found || switches != 1 {
				t.Fatal("missing transfer or failover", switches)
			}
			if profile.Network == "tron" && number >= 9 {
				// Exercise a complete ten-block batch, including pacing of
				// every underlying block/receipt HTTP request.
				batch, e := reader.Logs(ctx, number-9, number, recipient)
				if e != nil {
					t.Fatal("ten-block historical scan", e)
				}
				included := false
				for _, l := range batch {
					if l.TransactionHash == tx && l.LogIndex == event.LogIndex {
						included = true
					}
				}
				if !included {
					t.Fatal("ten-block batch lost sample transfer")
				}
			}
			t.Log("identity, fresh finality, historical header, recipient logs, receipt and forced failover verified")
		})
	}
}
