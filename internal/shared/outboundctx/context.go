package outboundctx

import (
	"context"
	"time"
)

const DefaultTimeout = 12 * time.Second

// Detach removes caller cancellation while preserving values, then applies a
// bounded timeout for the outbound operation.
func Detach(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	if parent == nil {
		return context.WithTimeout(context.Background(), timeout)
	}
	return context.WithTimeout(context.WithoutCancel(parent), timeout)
}

// WithDefaultTimeout applies the shared outbound timeout unless a deadline is
// already present.
func WithDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, ok := ctx.Deadline(); ok {
		return ctx, func() {}
	}
	return context.WithTimeout(ctx, DefaultTimeout)
}
