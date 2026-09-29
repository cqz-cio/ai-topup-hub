package application

import (
	"context"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/sitemap/contract"
)

type catalogStub struct {
	calls int
}

func (c *catalogStub) ListActiveCategories() ([]contract.Category, error) {
	c.calls++
	return nil, nil
}

func (c *catalogStub) ListActiveProducts(int) ([]contract.Product, error) {
	c.calls++
	return nil, nil
}

type postStub struct {
	calls int
}

func (p *postStub) ListPublishedPosts(context.Context, int) ([]contract.PublishedPost, error) {
	p.calls++
	return nil, nil
}

type cacheStub struct {
	value    string
	setKey   string
	setValue string
	setTTL   time.Duration
}

func (c *cacheStub) GetString(context.Context, string) (string, error) {
	return c.value, nil
}

func (c *cacheStub) SetString(_ context.Context, key, value string, ttl time.Duration) error {
	c.setKey = key
	c.setValue = value
	c.setTTL = ttl
	return nil
}

func TestGenerateReturnsCachedDocumentWithoutReadingSources(t *testing.T) {
	catalog := &catalogStub{}
	posts := &postStub{}
	cache := &cacheStub{value: "<cached/>"}
	service, err := NewService(catalog, posts, cache)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	got, err := service.Generate(context.Background(), "https://example.com/")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if got != "<cached/>" {
		t.Fatalf("cached document got %q", got)
	}
	if catalog.calls != 0 || posts.calls != 0 {
		t.Fatalf("cache hit must skip sources: catalog=%d posts=%d", catalog.calls, posts.calls)
	}
}

func TestGenerateCachesRenderedDocument(t *testing.T) {
	catalog := &catalogStub{}
	posts := &postStub{}
	cache := &cacheStub{}
	service, err := NewService(catalog, posts, cache)
	if err != nil {
		t.Fatalf("new service failed: %v", err)
	}

	document, err := service.Generate(context.Background(), "https://example.com/")
	if err != nil {
		t.Fatalf("generate failed: %v", err)
	}
	if cache.setKey != "sitemap:xml:https://example.com" {
		t.Fatalf("cache key got %q", cache.setKey)
	}
	if cache.setValue != document || cache.setValue == "" {
		t.Fatal("rendered document was not written to cache")
	}
	if cache.setTTL != 5*time.Minute {
		t.Fatalf("cache ttl got %s", cache.setTTL)
	}
	if catalog.calls != 2 || posts.calls != 1 {
		t.Fatalf("cache miss source calls: catalog=%d posts=%d", catalog.calls, posts.calls)
	}
}
