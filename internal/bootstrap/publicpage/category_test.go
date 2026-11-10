package publicpagewiring

import (
	"context"
	"errors"
	"strings"
	"testing"

	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	productpresenter "github.com/dujiao-next/internal/modules/catalog/product/transport/presenter"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/publicpage"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/shopspring/decimal"
)

type categoryFixture struct {
	categories []categorydomain.Category
	rows       []string
	items      map[string]productpresenter.Product
	err        error
	tenant     reseller.TenantContext
	categoryID string
}

func (f *categoryFixture) ListActive() ([]categorydomain.Category, error) { return f.categories, f.err }
func (f *categoryFixture) ListAnonymousProductSlugs(ctx context.Context, category string, page, size int) ([]string, int64, error) {
	f.tenant, _ = reseller.TenantFromContext(ctx)
	f.categoryID = category
	return f.rows, int64(len(f.rows)), f.err
}
func (f *categoryFixture) ReadAnonymousProduct(_ context.Context, slug string) (productpresenter.Product, error) {
	if f.err != nil {
		return productpresenter.Product{}, f.err
	}
	if p, ok := f.items[slug]; ok {
		return p, nil
	}
	return productpresenter.Product{}, productcontract.ErrNotFound
}
func fixture() *categoryFixture {
	discount := money.FromDecimal(decimal.NewFromInt(120))
	return &categoryFixture{
		categories: []categorydomain.Category{{ID: 9, Slug: "qzweb-category-4", NameJSON: jsonmap.JSON{"zh-CN": "GPT（卡冲）"}}},
		rows:       []string{"pro", "hidden", "plus"},
		items: map[string]productpresenter.Product{
			"pro":  {Slug: "pro", Title: jsonmap.JSON{"zh-CN": "Pro"}, PriceAmount: money.FromDecimal(decimal.NewFromInt(700)), IsSoldOut: true},
			"plus": {Slug: "plus", Title: jsonmap.JSON{"zh-CN": "Plus"}, PriceAmount: money.FromDecimal(decimal.NewFromInt(130)), EffectivePriceAmount: &discount},
		},
	}
}
func TestCategoryUsesVisibleLiveAnonymousProductsAndCurrentPrices(t *testing.T) {
	f := fixture()
	s := source{categories: f, catalog: f, products: f}
	page, err := s.Category(context.Background(), "qzweb-category-4")
	if err != nil {
		t.Fatal(err)
	}
	if f.categoryID != "9" || len(page.Items) != 2 || page.Items[0].Path != "/products/plus" || page.Items[0].Price != "120.00" || page.Items[1].Availability != "https://schema.org/OutOfStock" {
		t.Fatalf("unexpected projection: %+v", page)
	}
	if !strings.Contains(page.Content, "/blog/chatgpt-plus-code-guide") || page.SEOTitle == "" {
		t.Fatal("guide missing")
	}
	item := f.items["plus"]
	item.EffectivePriceAmount = nil
	item.PriceAmount = money.FromDecimal(decimal.NewFromInt(140))
	f.items["plus"] = item
	updated, _ := s.Category(context.Background(), "qzweb-category-4")
	if updated.Items[0].Price != "140.00" {
		t.Fatal("stale price")
	}
}
func TestCategoryRejectsMissingEmptyAndFailedCatalogs(t *testing.T) {
	for _, testCase := range []string{"missing", "empty", "failed"} {
		t.Run(testCase, func(t *testing.T) {
			f := fixture()
			if testCase == "missing" {
				f.categories = nil
			}
			if testCase == "empty" {
				f.rows = nil
			}
			if testCase == "failed" {
				f.err = errors.New("database failure")
			}
			_, err := (&source{categories: f, catalog: f, products: f}).Category(context.Background(), "qzweb-category-4")
			if err == nil {
				t.Fatal("unexpected success")
			}
			if testCase != "failed" && !errors.Is(err, publicpage.ErrNotFound) {
				t.Fatal(err)
			}
		})
	}
}
func TestCategoryDoesNotExportMainStoreCopyToReseller(t *testing.T) {
	f := fixture()
	ctx := reseller.WithTenantContext(context.Background(), reseller.ResellerTenantContext("tenant.example", 3, 4, "tenant.example"))
	page, err := (&source{categories: f, catalog: f, products: f}).Category(ctx, "qzweb-category-4")
	if err != nil {
		t.Fatal(err)
	}
	if page.Content != "" || page.Title != "GPT（卡冲）" || !f.tenant.IsReseller() {
		t.Fatal("main-site copy crossed tenant boundary")
	}
}
