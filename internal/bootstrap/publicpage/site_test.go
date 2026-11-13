package publicpagewiring

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/dujiao-next/internal/constants"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contentdomain "github.com/dujiao-next/internal/modules/content/domain"
	contenthttp "github.com/dujiao-next/internal/modules/content/transport/http"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/publicpage"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type siteSettings struct {
	values map[string]jsonmap.JSON
	err    error
}

func (s siteSettings) GetByKey(key string) (jsonmap.JSON, error) { return s.values[key], s.err }
func (s siteSettings) GetSiteCurrency(string) (string, error)    { return "CNY", s.err }

type postListFixture struct {
	query contentapp.PublicPostQuery
	fail  bool
}

func (p *postListFixture) ListPublic(_ context.Context, query contentapp.PublicPostQuery) ([]contentdomain.Post, int64, error) {
	p.query = query
	if p.fail {
		return nil, 0, errors.New("database offline")
	}
	return []contentdomain.Post{{Slug: "published", Type: "blog", TitleJSON: jsonmap.JSON{"zh-CN": "已发布文章"}, SummaryJSON: jsonmap.JSON{"zh-CN": "完整摘要"}}}, 13, nil
}
func (*postListFixture) GetPublicBySlug(context.Context, string) (*contentdomain.Post, error) {
	return nil, contentcontract.ErrNotFound
}
func (*postListFixture) ListRelatedProducts(context.Context, uint) ([]contentcontract.RelatedProduct, error) {
	return nil, nil
}

func TestHomeUsesLiveAnonymousPricesAndWhitelistedSiteConfig(t *testing.T) {
	f := fixture()
	settings := siteSettings{values: map[string]jsonmap.JSON{constants.SettingKeySiteConfig: {"brand": jsonmap.JSON{"site_name": "π星卡社", "site_url": "https://www.qzweb.cloud", "site_logo": "/brand/logo.png"}, "seo": jsonmap.JSON{"title": map[string]any{"zh-CN": "首页标题"}}, "private_key": "never expose"}}}
	s := source{products: f, catalog: f, settings: settings}
	page, err := s.Home(context.Background())
	if err != nil || len(page.Items) != 2 || page.Items[1].Price != "120.00" || f.categoryID != "" {
		t.Fatal(page, err)
	}
	if !strings.Contains(page.Content, "保存邮箱和查询密码") || page.SEOTitle != "首页标题" {
		t.Fatal("missing visible copy/metadata", page)
	}
	data, _ := s.publicConfig(context.Background(), false)
	if _, exists := data["private_key"]; exists {
		t.Fatal("private config escaped whitelist")
	}
	site, err := s.Site(context.Background())
	if err != nil || site.Name != "π星卡社" || site.Logo != "/brand/logo.png" {
		t.Fatal(site, err)
	}
	if site.ReturnPolicyPath != "/blog/purchase-and-agent-guide" {
		t.Fatal("main-store policy missing")
	}
	ctx := reseller.WithTenantContext(context.Background(), reseller.ResellerTenantContext("tenant.example", 3, 4, "tenant.example"))
	resellerSite, err := s.Site(ctx)
	if err != nil || resellerSite.ReturnPolicyPath != "" {
		t.Fatal("main-store policy crossed tenant boundary", resellerSite, err)
	}
	page, err = s.Home(ctx)
	if err != nil || strings.Contains(page.Content, "保存邮箱") || len(page.Links) != 0 {
		t.Fatal("main-home editorial content crossed tenant boundary")
	}
	f.err = errors.New("catalog unavailable")
	if _, err := s.Home(context.Background()); err == nil {
		t.Fatal("catalog failure became empty successful home")
	}
}

func TestBlogListSharesPublishedQueryPaginationAndErrorSemantics(t *testing.T) {
	f := &postListFixture{}
	s := source{posts: contenthttp.NewPublicHandler(f, nil, nil)}
	page, err := s.BlogList(context.Background(), 1, "")
	if err != nil || f.query.Type != "blog" || f.query.PageSize != 12 || len(page.Entries) != 1 || page.Pagination[0].Path != "/blog?page=2" {
		t.Fatal(page, f.query, err)
	}
	page, err = s.BlogList(context.Background(), 2, "")
	if err != nil || page.Path != "/blog?page=2" || page.Pagination[0].Path != "/blog" {
		t.Fatal(page, err)
	}
	if _, err := s.BlogList(context.Background(), 3, ""); !errors.Is(err, publicpage.ErrNotFound) {
		t.Fatal("out of range is not 404", err)
	}
	page, err = s.BlogList(context.Background(), 1, "Plus")
	if err != nil || !page.NoIndex || page.Path != "/blog" || f.query.Search != "Plus" {
		t.Fatal("search indexing", page, err)
	}
	f.fail = true
	if _, err := s.BlogList(context.Background(), 1, ""); err == nil || errors.Is(err, publicpage.ErrNotFound) {
		t.Fatal("database failure misclassified", err)
	}
}

func TestProductsAndLegalReadOnlyPublicContent(t *testing.T) {
	f := fixture()
	settings := siteSettings{values: map[string]jsonmap.JSON{constants.SettingKeySiteConfig: {
		"legal":   jsonmap.JSON{"terms": jsonmap.JSON{"zh-CN": "<h2>真实服务条款</h2>"}, "privacy": jsonmap.JSON{"en-US": "<p>Privacy policy</p>"}, "internal_notes": "secret"},
		"scripts": jsonmap.JSON{"private": "secret"},
	}}}
	s := source{products: f, catalog: f, settings: settings}
	page, err := s.Products(context.Background())
	if err != nil || page.Kind != "products" || page.Path != "/products" || len(page.Items) != 2 || page.Items[1].Price != "120.00" {
		t.Fatal(page, err)
	}
	page, err = s.Terms(context.Background())
	if err != nil || page.Content != "<h2>真实服务条款</h2>" || page.Kind != "legal" {
		t.Fatal(page, err)
	}
	page, err = s.Privacy(context.Background())
	if err != nil || page.Content != "<p>Privacy policy</p>" {
		t.Fatal(page, err)
	}
	config, err := s.publicConfig(context.Background(), false)
	if err != nil || config["scripts"] != nil || configMap(config["legal"])["internal_notes"] != nil {
		t.Fatal("private data escaped whitelist", config, err)
	}
	settings.values[constants.SettingKeySiteConfig]["legal"] = nil
	if _, err = s.Terms(context.Background()); !errors.Is(err, publicpage.ErrNotFound) {
		t.Fatal("empty policy must not return success", err)
	}
}

func TestNoticeListUsesPublishedNoticeQueryAndOwnPagination(t *testing.T) {
	f := &postListFixture{}
	s := source{posts: contenthttp.NewPublicHandler(f, nil, nil)}
	page, err := s.NoticeList(context.Background(), 1, "")
	if err != nil || f.query.Type != "notice" || page.Kind != "notice-list" || page.Pagination[0].Path != "/notice?page=2" || page.Entries[0].Path != "/blog/published" {
		t.Fatal(page, f.query, err)
	}
	page, err = s.NoticeList(context.Background(), 2, "售后")
	if err != nil || !page.NoIndex || page.Path != "/notice" || f.query.Search != "售后" {
		t.Fatal(page, err)
	}
	if _, err = s.NoticeList(context.Background(), 3, ""); !errors.Is(err, publicpage.ErrNotFound) {
		t.Fatal(err)
	}
}
