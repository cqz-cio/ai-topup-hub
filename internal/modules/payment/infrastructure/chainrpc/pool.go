// Package chainrpc provides bounded, sticky failover for read-only chain APIs.
package chainrpc

import (
	"context"
	"sync"
	"time"

	"github.com/dujiao-next/internal/modules/payment/contract/chain"
)

// Event contains indexes, never endpoint URLs or provider error messages.
type Event struct {
	Kind string
	From int
	To   int
}

type node struct {
	reader   chain.Reader
	checked  time.Time
	retryAt  time.Time
	failures int
}

type Pool struct {
	mu             sync.Mutex
	nodes          []node
	active         int
	now            func() time.Time
	notify         func(Event)
	attemptTimeout time.Duration
}

// New keeps each reader's credentials and capability endpoints isolated.
// No network request occurs until the first read. At most four providers may
// be configured, keeping a failed scan within the caller's deadline.
func New(readers []chain.Reader, notify func(Event)) (*Pool, error) {
	if len(readers) < 1 || len(readers) > 4 {
		return nil, chain.ErrRPC
	}
	p := &Pool{now: time.Now, notify: notify, attemptTimeout: 20 * time.Second}
	for _, reader := range readers {
		if reader == nil {
			return nil, chain.ErrRPC
		}
		p.nodes = append(p.nodes, node{reader: reader})
	}
	return p, nil
}

// do serializes selection and requests, so concurrent checkout and scanning
// cannot race health updates or fan out onto a rate-limited provider.
func (p *Pool) do(ctx context.Context, read func(context.Context, chain.Reader) error) error {
	ctx, cancelAll := context.WithTimeout(ctx, 40*time.Second)
	defer cancelAll()
	// Acquire interruptibly: a waiting checkout must respect cancellation.
	for !p.mu.TryLock() {
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	var events []Event
	defer func() {
		p.mu.Unlock()
		if p.notify != nil {
			for _, e := range events {
				p.notify(e)
			}
		}
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	var candidates []int
	for offset := 0; offset < len(p.nodes); offset++ {
		i := (p.active + offset) % len(p.nodes)
		if !p.now().Before(p.nodes[i].retryAt) {
			candidates = append(candidates, i)
		}
	}
	for pos, i := range candidates {
		if err := ctx.Err(); err != nil {
			return err
		}
		n := &p.nodes[i]
		budget := p.attemptTimeout
		if deadline, ok := ctx.Deadline(); ok {
			share := time.Until(deadline) / time.Duration(len(candidates)-pos)
			if share < budget {
				budget = share
			}
		}
		attempt, cancel := context.WithTimeout(ctx, budget)
		var err error
		// Validate every provider before using its result; failures invalidate
		// the cache. Identity checks are otherwise cached for five minutes.
		if n.checked.IsZero() || p.now().Sub(n.checked) >= 5*time.Minute {
			err = n.reader.Check(attempt)
			if err == nil {
				n.checked = p.now()
			}
		}
		if err == nil && read != nil {
			err = read(attempt, n.reader)
		}
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			if p.active != i {
				events = append(events, Event{Kind: "switched", From: p.active, To: i})
			}
			p.active = i
			if read != nil {
				n.failures, n.retryAt = 0, time.Time{}
			}
			return nil
		}
		n.checked = time.Time{}
		n.failures++
		shift := n.failures - 1
		if shift > 4 {
			shift = 4
		}
		n.retryAt = p.now().Add(30 * time.Second * time.Duration(1<<shift))
		events = append(events, Event{Kind: "unavailable", From: i, To: i})
	}
	// Cooling providers are not hammered on every ten-second scanner tick.
	return chain.ErrRPC
}

func (p *Pool) Check(ctx context.Context) error {
	return p.do(ctx, nil)
}
func (p *Pool) Header(ctx context.Context, tag string) (*chain.Header, error) {
	var result *chain.Header
	err := p.do(ctx, func(ctx context.Context, r chain.Reader) error {
		h, err := r.Header(ctx, tag)
		if err != nil {
			return err
		}
		if h == nil || !chain.IsHex(h.Hash, 32) {
			return chain.ErrRPC
		}
		number, err := chain.Quantity(h.Number)
		if err != nil {
			return err
		}
		stamp, err := chain.Quantity(h.Timestamp)
		if err != nil {
			return err
		}
		if tag == "latest" || tag == "finalized" {
			limit := 2 * time.Minute
			if tag == "finalized" {
				limit = 5 * time.Minute
			}
			now := p.now().Unix()
			if stamp > uint64(now+30) || stamp < uint64(now-int64(limit/time.Second)) {
				return chain.ErrRPC
			}
		} else {
			want, e := chain.Quantity(tag)
			if e != nil || want != number {
				return chain.ErrRPC
			}
		}
		result = h
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (p *Pool) Logs(ctx context.Context, from, to uint64, recipient string) ([]chain.Log, error) {
	if from > to || chain.AddressTopic(recipient) == "" {
		return nil, chain.ErrRPC
	}
	var result []chain.Log
	err := p.do(ctx, func(ctx context.Context, r chain.Reader) error {
		logs, err := r.Logs(ctx, from, to, recipient)
		if err != nil {
			return err
		}
		result = logs
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
func (p *Pool) Receipt(ctx context.Context, hash string) (*chain.Receipt, error) {
	if !chain.IsHex(hash, 32) {
		return nil, chain.ErrRPC
	}
	var result *chain.Receipt
	err := p.do(ctx, func(ctx context.Context, r chain.Reader) error {
		receipt, err := r.Receipt(ctx, hash)
		if err != nil {
			return err
		}
		if receipt == nil {
			return chain.ErrRPC
		}
		result = receipt
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}
