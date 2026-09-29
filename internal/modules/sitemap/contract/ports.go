package contract

import (
	"context"
	"time"
)

// Category 是生成 Sitemap 所需的分类投影。
type Category struct {
	Slug      string
	CreatedAt time.Time
}

// Product 是生成 Sitemap 所需的商品投影。
type Product struct {
	Slug      string
	UpdatedAt time.Time
}

// PublishedPost 是生成 Sitemap 所需的已发布文章投影。
type PublishedPost struct {
	Slug        string
	CreatedAt   time.Time
	PublishedAt *time.Time
}

// CatalogReader 提供 Sitemap 所需的可索引商品和分类。
type CatalogReader interface {
	ListActiveCategories() ([]Category, error)
	ListActiveProducts(limit int) ([]Product, error)
}

// PublishedPostReader 提供 Sitemap 所需的已发布文章。
type PublishedPostReader interface {
	ListPublishedPosts(ctx context.Context, limit int) ([]PublishedPost, error)
}

// PublishedPostReaderFunc 将函数适配为 PublishedPostReader。
type PublishedPostReaderFunc func(ctx context.Context, limit int) ([]PublishedPost, error)

// ListPublishedPosts 实现 PublishedPostReader。
func (f PublishedPostReaderFunc) ListPublishedPosts(ctx context.Context, limit int) ([]PublishedPost, error) {
	return f(ctx, limit)
}

// Cache 是 Sitemap 内容缓存所需的最小端口。
type Cache interface {
	GetString(ctx context.Context, key string) (string, error)
	SetString(ctx context.Context, key, value string, ttl time.Duration) error
}
