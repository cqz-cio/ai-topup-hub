package cacheadapter

import (
	"context"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/modules/sitemap/contract"
)

// Cache 将共享缓存能力适配为 Sitemap 应用端口。
type Cache struct{}

var _ contract.Cache = Cache{}

// New 创建 Sitemap 缓存适配器。
func New() Cache { return Cache{} }

// GetString 读取字符串缓存。
func (Cache) GetString(ctx context.Context, key string) (string, error) {
	return cache.GetString(ctx, key)
}

// SetString 写入字符串缓存。
func (Cache) SetString(ctx context.Context, key, value string, ttl time.Duration) error {
	return cache.SetString(ctx, key, value, ttl)
}
