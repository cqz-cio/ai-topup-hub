package bscusdt

import (
	"context"
	"testing"
)

func TestBSCBacklogResumesWithoutSkippingOrReprocessing(t *testing.T) {
	f := setup(t)
	ctx := context.Background()
	before, err := f.s.store.Cursor(ctx, f.s.stream)
	if err != nil {
		t.Fatal(err)
	}
	f.chain.head, f.chain.final = before.NextBlock+1200, before.NextBlock+1200
	// A matching transfer in the second batch must wait for that batch.
	f.chain.logs = []Log{f.event(f.invoice.Units, before.NextBlock+501, 0)}
	f.run(t)
	first, err := f.s.store.Cursor(ctx, f.s.stream)
	if err != nil || first.NextBlock != before.NextBlock+500 || f.calls != 0 {
		t.Fatalf("unbounded or skipped first batch: %#v, calls=%d, err=%v", first, f.calls, err)
	}
	// An unavailable provider must preserve the cursor for the next retry.
	f.chain.err = ErrRPC
	if err = f.s.RunOnce(ctx); err != ErrRPC {
		t.Fatal(err)
	}
	failed, err := f.s.store.Cursor(ctx, f.s.stream)
	if err != nil || failed.NextBlock != first.NextBlock || failed.PreviousHash != first.PreviousHash {
		t.Fatal("failed scan advanced cursor", failed, err)
	}
	f.chain.err = nil
	f.run(t)
	second, err := f.s.store.Cursor(ctx, f.s.stream)
	if err != nil || second.NextBlock != before.NextBlock+1000 || f.calls != 1 {
		t.Fatal("retry missed or duplicated payment", second, f.calls, err)
	}
	f.run(t)
	last, err := f.s.store.Cursor(ctx, f.s.stream)
	if err != nil || last.NextBlock != f.chain.head-f.s.config.Confirmations+2 || f.calls != 1 {
		t.Fatal("catch-up crossed confirmation boundary or delivered twice", last, f.calls, err)
	}
}
