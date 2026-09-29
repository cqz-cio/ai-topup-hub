package common

import (
	"context"

	"github.com/dujiao-next/internal/shared/outboundctx"
)

// DefaultTimeout 支付请求默认超时。
const DefaultTimeout = outboundctx.DefaultTimeout

// WithDefaultTimeout 在没有 deadline 时添加默认超时。
func WithDefaultTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return outboundctx.WithDefaultTimeout(ctx)
}
