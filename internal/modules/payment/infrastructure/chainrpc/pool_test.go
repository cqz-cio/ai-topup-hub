package chainrpc

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
)

type fakeReader struct {
	checkErr, errorRead error
	checks, reads       atomic.Int32
	stamp               time.Time
	delay               bool
}

func (f *fakeReader) Check(ctx context.Context) error {
	f.checks.Add(1)
	if f.delay {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.checkErr
}
func (f *fakeReader) Header(context.Context, string) (*chain.Header, error) {
	f.reads.Add(1)
	return &chain.Header{Number: "0x64", Hash: fmt.Sprintf("0x%064x", 100), Timestamp: chain.HexBlock(uint64(f.stamp.Unix()))}, f.errorRead
}
func (f *fakeReader) Logs(context.Context, uint64, uint64, string) ([]chain.Log, error) {
	f.reads.Add(1)
	return []chain.Log{{TransactionHash: fmt.Sprintf("0x%064x", 1)}}, f.errorRead
}
func (f *fakeReader) Receipt(context.Context, string) (*chain.Receipt, error) {
	f.reads.Add(1)
	return &chain.Receipt{Status: "0x1"}, f.errorRead
}
func TestFailureSwitchesValidatesAndSticks(t *testing.T) {
	a, b := &fakeReader{errorRead: chain.ErrRPC, stamp: time.Now()}, &fakeReader{stamp: time.Now()}
	var events []Event
	p, _ := New([]chain.Reader{a, b}, func(e Event) { events = append(events, e) })
	for i := 0; i < 3; i++ {
		if _, err := p.Header(context.Background(), "latest"); err != nil {
			t.Fatal(err)
		}
	}
	if a.reads.Load() != 1 || b.reads.Load() != 3 || b.checks.Load() != 1 || len(events) != 2 || events[1].Kind != "switched" {
		t.Fatal("selection/cache/events", events)
	}
}
func TestWrongIdentityNeverReadsAndErrorsAreSanitized(t *testing.T) {
	a := &fakeReader{checkErr: errors.New("private-provider-url")}
	b := &fakeReader{stamp: time.Now()}
	p, _ := New([]chain.Reader{a, b}, nil)
	if _, err := p.Header(context.Background(), "latest"); err != nil || a.reads.Load() != 0 {
		t.Fatal("used unvalidated provider", err)
	}
	p, _ = New([]chain.Reader{a}, nil)
	if err := p.Check(context.Background()); err != chain.ErrRPC {
		t.Fatal("leaked provider error", err)
	}
}
func TestCooldownRecoveryAndIdentityRecheck(t *testing.T) {
	now := time.Now()
	a, b := &fakeReader{errorRead: chain.ErrRPC, stamp: now}, &fakeReader{errorRead: chain.ErrRPC, stamp: now}
	p, _ := New([]chain.Reader{a, b}, nil)
	p.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if _, err := p.Header(context.Background(), "latest"); err != chain.ErrRPC {
			t.Fatal(err)
		}
	}
	if a.reads.Load() != 1 || b.reads.Load() != 1 {
		t.Fatal("cooldown ignored")
	}
	now = now.Add(time.Minute)
	b.errorRead = nil
	if _, err := p.Header(context.Background(), "latest"); err != nil {
		t.Fatal("did not recover", err)
	}
	if b.checks.Load() != 2 {
		t.Fatal("recovery skipped identity")
	}
}
func TestStaleHeadOrFinalitySwitches(t *testing.T) {
	for _, tag := range []string{"latest", "finalized"} {
		t.Run(tag, func(t *testing.T) {
			a, b := &fakeReader{stamp: time.Now().Add(-10 * time.Minute)}, &fakeReader{stamp: time.Now()}
			p, _ := New([]chain.Reader{a, b}, nil)
			if _, err := p.Header(context.Background(), tag); err != nil || p.active != 1 {
				t.Fatal("stale head accepted", err)
			}
		})
	}
}
func TestTimedOutAttemptLeavesBudgetForBackup(t *testing.T) {
	a, b := &fakeReader{delay: true}, &fakeReader{stamp: time.Now()}
	p, _ := New([]chain.Reader{a, b}, nil)
	p.attemptTimeout = 10 * time.Millisecond
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, err := p.Header(ctx, "latest"); err != nil || p.active != 1 {
		t.Fatal("no timed failover", err)
	}
}
func TestCancellationDoesNotPoisonHealthOrCallBackup(t *testing.T) {
	a, b := &fakeReader{stamp: time.Now()}, &fakeReader{stamp: time.Now()}
	p, _ := New([]chain.Reader{a, b}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.Check(ctx); err != context.Canceled || a.checks.Load() != 0 || b.checks.Load() != 0 || p.nodes[0].failures != 0 {
		t.Fatal("cancellation", err)
	}
	p.mu.Lock()
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := p.Check(ctx); err != context.DeadlineExceeded {
		t.Fatal("lock ignored deadline", err)
	}
	p.mu.Unlock()
}
func TestPartialFailureDoesNotEscapeAndReceiptFallsBack(t *testing.T) {
	for _, method := range []string{"logs", "receipt"} {
		t.Run(method, func(t *testing.T) {
			a, b := &fakeReader{errorRead: chain.ErrRPC}, &fakeReader{}
			p, _ := New([]chain.Reader{a, b}, nil)
			if method == "logs" {
				logs, err := p.Logs(context.Background(), 100, 101, "0x66a8bab15067eab1efc948ac03684f0397c8416f")
				if err != nil || len(logs) != 1 {
					t.Fatal("partial data escaped", err)
				}
			} else {
				r, err := p.Receipt(context.Background(), fmt.Sprintf("0x%064x", 1))
				if err != nil || r == nil {
					t.Fatal(err)
				}
			}
			if p.active != 1 || b.checks.Load() != 1 {
				t.Fatal("backup not validated")
			}
		})
	}
}
func TestConcurrentReadsUseOneIdentityCheck(t *testing.T) {
	r := &fakeReader{stamp: time.Now()}
	p, _ := New([]chain.Reader{r}, nil)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.Header(context.Background(), "latest"); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if r.checks.Load() != 1 || r.reads.Load() != 20 {
		t.Fatal("concurrent selection")
	}
}
