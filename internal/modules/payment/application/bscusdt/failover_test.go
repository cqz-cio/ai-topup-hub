package bscusdt

import (
	"context"
	"testing"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
	"github.com/dujiao-next/internal/modules/payment/infrastructure/chainrpc"
)

type failingLogs struct{ chain.Reader }

func (r failingLogs) Logs(context.Context, uint64, uint64, string) ([]chain.Log, error) {
	return nil, chain.ErrRPC
}

func TestThreeNetworksFailoverPreservesCursorAndSettlesOnce(t *testing.T) {
	for _, network := range []string{"bsc", "plasma", "tron"} {
		t.Run(network, func(t *testing.T) {
			f := setupNetwork(t, network)
			ctx := context.Background()
			before, err := f.s.store.Cursor(ctx, f.s.stream)
			if err != nil {
				t.Fatal(err)
			}
			f.chain.head, f.chain.final = 120, 120
			f.chain.logs = []Log{f.event(f.invoice.Units, 101, 0)}
			backup := *f.chain
			bothDown, _ := chainrpc.New([]chain.Reader{failingLogs{f.chain}, failingLogs{&backup}}, nil)
			f.s.chain = bothDown
			if err = f.s.RunOnce(ctx); err != chain.ErrRPC {
				t.Fatal("all-down scan should fail", err)
			}
			after, _ := f.s.store.Cursor(ctx, f.s.stream)
			if after.NextBlock != before.NextBlock || after.PreviousHash != before.PreviousHash || f.calls != 0 {
				t.Fatal("outage lost progress or settled")
			}
			// A restarted listener uses the persisted checkpoint, then fails
			// over during logs without reissuing or changing the invoice.
			var switches int
			recovered, _ := chainrpc.New([]chain.Reader{failingLogs{f.chain}, &backup}, func(e chainrpc.Event) {
				if e.Kind == "switched" {
					switches++
				}
			})
			f.s.chain = recovered
			f.run(t)
			f.run(t)
			if f.calls != 1 || switches != 1 {
				t.Fatal("missed/duplicate settlement or no switch", f.calls, switches)
			}
			final, _ := f.s.store.Cursor(ctx, f.s.stream)
			if final.NextBlock <= before.NextBlock {
				t.Fatal("did not resume")
			}
		})
	}
}

func TestFailoverDoesNotBypassReorgOrBadReceipt(t *testing.T) {
	for _, kind := range []string{"reorg", "receipt"} {
		t.Run(kind, func(t *testing.T) {
			f := setup(t)
			before, _ := f.s.store.Cursor(context.Background(), f.s.stream)
			f.chain.head, f.chain.final = 120, 120
			f.chain.logs = []Log{f.event(f.invoice.Units, 101, 0)}
			if kind == "reorg" {
				f.chain.reorg = 101
			} else {
				f.chain.badReceipt = true
			}
			pool, _ := chainrpc.New([]chain.Reader{failingLogs{f.chain}, f.chain}, nil)
			f.s.chain = pool
			if err := f.s.RunOnce(context.Background()); err == nil {
				t.Fatal("accepted invalid transfer")
			}
			after, _ := f.s.store.Cursor(context.Background(), f.s.stream)
			if after.NextBlock != before.NextBlock || f.calls != 0 {
				t.Fatal("invalid payment advanced or settled")
			}
		})
	}
}
