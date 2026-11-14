package application

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/dujiao-next/internal/modules/sitemap/contract"
)

type catalogStub struct {
	calls      int
	categories []contract.Category
	products   []contract.Product
}

func (c *catalogStub) ListActiveCategories() ([]contract.Category, error) {
	c.calls++
	return c.categories, nil
}

func TestRechargeCategoryUsesCanonicalURLAndActualEditorialDate(t *testing.T) {
	catalog := &catalogStub{categories: []contract.Category{
		{ID: 1, Slug: "gpt-recharge", CreatedAt: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		{ID: 2, Slug: "other", CreatedAt: time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)},
	}, products: []contract.Product{{CategoryID: 1, Slug: "plus"}, {CategoryID: 2, Slug: "other"}}}
	service, err := NewService(catalog, &postStub{}, &cacheStub{})
	if err != nil {
		t.Fatal(err)
	}
	entries, err := service.collectURLs(context.Background(), "https://www.qzweb.cloud")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Location, "/categories/gpt-recharge") && entry.LastModified != "2026-11-10" {
			t.Fatal(entry)
		}
		if strings.HasSuffix(entry.Location, "/categories/other") && entry.LastModified != "2026-10-02" {
			t.Fatal("unrelated category date changed")
		}
	}
}

func (c *catalogStub) ListActiveProducts(int) ([]contract.Product, error) {
	c.calls++
	return c.products, nil
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

func TestSitemapOmitsEmptyCategoriesWithoutScanningProductsTwice(t *testing.T) {
	catalog := &catalogStub{
		categories: []contract.Category{{ID: 1, Slug: "visible"}, {ID: 2, Slug: "empty"}, {ID: 3, Slug: "only-invalid-slug"}},
		products:   []contract.Product{{CategoryID: 1, Slug: "available"}, {CategoryID: 1, Slug: "sold-out"}, {CategoryID: 3, Slug: " "}},
	}
	service, err := NewService(catalog, &postStub{}, &cacheStub{})
	if err != nil {
		t.Fatal(err)
	}
	xml, err := service.Generate(context.Background(), "https://example.com")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"/categories/visible", "/products/available", "/products/sold-out"} {
		if !strings.Contains(xml, want) {
			t.Fatal("missing visible URL", want)
		}
	}
	for _, forbidden := range []string{"/categories/empty", "/categories/only-invalid-slug", "/products/%20"} {
		if strings.Contains(xml, forbidden) {
			t.Fatal("included empty category or invalid URL", forbidden)
		}
	}
	if catalog.calls != 2 {
		t.Fatal("duplicate catalog scan", catalog.calls)
	}
}

func TestSitemapMatchesPublicRootCategoryExpansion(t *testing.T) {
	catalog := &catalogStub{
		categories: []contract.Category{{ID: 1, Slug: "parent"}, {ID: 2, ParentID: 1, Slug: "child"}, {ID: 3, Slug: "empty"}},
		products:   []contract.Product{{CategoryID: 2, Slug: "child-product"}},
	}
	service, err := NewService(catalog, &postStub{}, &cacheStub{})
	if err != nil {
		t.Fatal(err)
	}
	xml, err := service.Generate(context.Background(), "https://example.com")
	if err != nil || !strings.Contains(xml, "/categories/parent") || !strings.Contains(xml, "/categories/child") || strings.Contains(xml, "/categories/empty") {
		t.Fatal(xml, err)
	}
}
