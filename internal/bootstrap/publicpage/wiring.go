package publicpagewiring

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/app/container"
	categorydomain "github.com/dujiao-next/internal/modules/catalog/category/domain"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	producthttp "github.com/dujiao-next/internal/modules/catalog/product/transport/http"
	productpresenter "github.com/dujiao-next/internal/modules/catalog/product/transport/presenter"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contenthttp "github.com/dujiao-next/internal/modules/content/transport/http"
	resellerapp "github.com/dujiao-next/internal/modules/reseller/application"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/publicpage"
	"github.com/dujiao-next/internal/platform/publiccontent"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/shopspring/decimal"
)

type publicProductReader interface {
	ReadAnonymousProduct(context.Context, string) (productpresenter.Product, error)
}
type categoryReader interface {
	ListActive() ([]categorydomain.Category, error)
}
type catalogReader interface {
	ListAnonymousProductSlugs(context.Context, string, int, int) ([]string, int64, error)
}

type settingsReader interface {
	GetByKey(string) (jsonmap.JSON, error)
	GetSiteCurrency(string) (string, error)
}

type source struct {
	catalogLinks func(context.Context, map[string]string) (map[string]string, error)
	products     publicProductReader
	categories   categoryReader
	catalog      catalogReader
	posts        *contenthttp.PublicHandler
	settings     settingsReader
	overlay      *resellerapp.SiteConfigService
}

func New(c *container.Container, products *producthttp.PublicHandler, posts *contenthttp.PublicHandler) *publicpage.Handler {
	return publicpage.New(&source{products: products, categories: c.CategoryService, catalog: products, posts: posts, settings: c.SettingService, overlay: c.ResellerSiteConfigService, catalogLinks: c.ProductRepo.PublicCatalogLinks})
}

func (s *source) PrepareContext(ctx context.Context) (context.Context, error) {
	if !isMainSite(ctx) || s.catalogLinks == nil {
		return publiccontent.WithCatalogLinks(ctx, map[string]string{}), nil
	}
	links, err := s.catalogLinks(ctx, publiccontent.GPTRecharge().CatalogIdentities)
	if err != nil {
		return ctx, err
	}
	return publiccontent.WithCatalogLinks(ctx, links), nil
}

func (s *source) Site(ctx context.Context) (publicpage.Site, error) {
	data, err := s.publicConfig(ctx, false)
	if err != nil {
		return publicpage.Site{}, err
	}
	currency, err := s.settings.GetSiteCurrency("CNY")
	if err != nil {
		return publicpage.Site{}, err
	}
	brand, seo := configMap(data["brand"]), configMap(data["seo"])
	name, origin := metadataText(brand["site_name"]), metadataText(brand["site_url"])
	if name == "" {
		name = metadataText(seo["title"])
	}
	site := publicpage.Site{Name: name, URL: origin, Currency: currency, Logo: metadataText(brand["site_logo"]), Description: metadataText(seo["description"])}
	site.CatalogLinks = publiccontent.CatalogLinks(ctx)
	site.Icon, site.DefaultImage = metadataText(brand["site_icon"]), metadataText(seo["default_og_image"])
	if isMainSite(ctx) {
		site.ReturnPolicyPath = publiccontent.DeliveryAndReturns().Path
		assets := publiccontent.PublicSEOAssets()
		if site.Logo == assets.LegacyLogo {
			site.Logo = assets.Logo
		}
		if site.Icon == "" || site.Icon == assets.LegacyLogo {
			site.Icon = assets.Icon
		}
		if site.DefaultImage == assets.LegacyLogo || site.DefaultImage == assets.Logo || publicpage.RasterShareURL("https://www.qzweb.cloud", site.DefaultImage) == "" {
			site.DefaultImage = assets.ShareImage
		}
	}
	return site, nil
}

func (s *source) Product(ctx context.Context, slug string) (publicpage.Page, error) {
	p, err := s.products.ReadAnonymousProduct(ctx, slug)
	if errors.Is(err, productcontract.ErrNotFound) || errors.Is(err, productcontract.ErrResellerProductNotListed) {
		return publicpage.Page{}, publicpage.ErrNotFound
	}
	if err != nil {
		return publicpage.Page{}, err
	}
	if p.Slug != slug {
		return publicpage.Page{}, publicpage.Redirect{Path: "/products/" + url.PathEscape(p.Slug)}
	}
	price := p.PriceAmount
	if p.EffectivePriceAmount != nil {
		price = *p.EffectivePriceAmount
	}
	page := publicpage.Page{Kind: "product", Path: "/products/" + url.PathEscape(p.Slug), Title: localized(p.Title), Summary: localized(p.Description), Content: localized(p.Content), Price: price.String(), Category: localized(p.Category.Name), Images: p.Images, Availability: "https://schema.org/InStock"}
	if isMainSite(ctx) {
		guide := publiccontent.GPTRecharge().Resolve(publiccontent.CatalogLinks(ctx))
		page.Content = guide.AppendRelated(guide.RewriteLinks(page.Content, publiccontent.CatalogLinks(ctx)), page.Kind, p.Slug, p.Category.Slug)
		page.Content = publiccontent.DeliveryAndReturns().AppendLink(page.Content)
	}
	page.SEOTitle = metadataText(p.SeoMeta["title"])
	page.SEODescription = metadataText(p.SeoMeta["description"])
	page.ShareImage = metadataText(p.SeoMeta["og_image"])
	page.ShareImageWidth, page.ShareImageHeight = metadataDimension(p.SeoMeta["og_image_width"]), metadataDimension(p.SeoMeta["og_image_height"])
	if p.IsSoldOut {
		page.Availability = "https://schema.org/OutOfStock"
	}
	for _, sku := range p.SKUs {
		if !sku.IsActive {
			continue
		}
		keys := make([]string, 0, len(sku.SpecValues))
		for key := range sku.SpecValues {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var specs []string
		for _, key := range keys {
			if value, ok := sku.SpecValues[key].(string); ok {
				specs = append(specs, key+"："+value)
			}
		}
		if len(specs) == 0 {
			continue
		}
		skuPrice := sku.PriceAmount
		if sku.EffectivePriceAmount != nil {
			skuPrice = *sku.EffectivePriceAmount
		}
		page.Facts = append(page.Facts, publicpage.Fact{Name: strings.Join(specs, " / "), Value: skuPrice.String()})
	}
	for _, post := range p.RelatedPosts {
		page.Links = append(page.Links, publicpage.Link{Title: localized(post.Title), Path: "/blog/" + url.PathEscape(post.Slug)})
	}
	return page, nil
}

func (s *source) Post(ctx context.Context, slug string) (publicpage.Page, error) {
	p, err := s.posts.ReadPublishedPost(ctx, slug)
	if errors.Is(err, contentcontract.ErrNotFound) {
		return publicpage.Page{}, publicpage.ErrNotFound
	}
	if err != nil {
		return publicpage.Page{}, err
	}
	page := publicpage.Page{Kind: "article", Path: "/blog/" + url.PathEscape(p.Slug), Title: localized(p.Title), Summary: localized(p.Summary), Content: localized(p.Content), PublishedAt: p.PublishedAt}
	if p.Type == "notice" {
		page.Kind = "notice"
	}
	if isMainSite(ctx) {
		guide := publiccontent.GPTRecharge().Resolve(publiccontent.CatalogLinks(ctx))
		page.Content = guide.AppendRelated(guide.RewriteLinks(page.Content, publiccontent.CatalogLinks(ctx)), page.Kind, p.Slug, "")
		page.Editorial = isEditorial(p.Slug)
	}
	if p.Thumbnail != "" {
		page.Images = []string{p.Thumbnail}
	}
	for _, product := range p.RelatedProducts {
		// Recheck tenant visibility; an article relation alone is not publication permission.
		item, err := s.products.ReadAnonymousProduct(ctx, product.Slug)
		if err == nil {
			page.Links = append(page.Links, publicpage.Link{Title: localized(item.Title), Path: "/products/" + url.PathEscape(item.Slug)})
		}
	}
	return page, nil
}

func isMainSite(ctx context.Context) bool {
	tenant, ok := reseller.TenantFromContext(ctx)
	return !ok || (tenant.IsMain && !tenant.Unavailable)
}

// Category uses live, publicly listed products. The anonymous detail projection
// remains responsible for stock masking, tenant visibility and current pricing.
func (s *source) Category(ctx context.Context, slug string) (publicpage.Page, error) {
	requested := slug
	if resolver, ok := s.categories.(interface{ ResolvePublicSlug(string) (string, error) }); ok {
		canonical, err := resolver.ResolvePublicSlug(slug)
		if err != nil {
			return publicpage.Page{}, err
		}
		slug = canonical
	}
	categories, err := s.categories.ListActive()
	if err != nil {
		return publicpage.Page{}, err
	}
	var category *categorydomain.Category
	for i := range categories {
		if categories[i].Slug == slug {
			category = &categories[i]
			break
		}
	}
	if category == nil {
		return publicpage.Page{}, publicpage.ErrNotFound
	}
	page := publicpage.Page{Kind: "category", Path: "/categories/" + url.PathEscape(slug), Title: publiccontent.CategoryTitle(localized(category.NameJSON))}
	page.Items, err = s.catalogItems(ctx, strconv.FormatUint(uint64(category.ID), 10))
	if err != nil {
		return publicpage.Page{}, err
	}
	if len(page.Items) == 0 {
		return publicpage.Page{}, publicpage.ErrNotFound
	}
	if requested != category.Slug {
		return publicpage.Page{}, publicpage.Redirect{Path: page.Path}
	}
	page.ShareImage = metadataText(category.SeoMetaJSON["og_image"])
	if category.Icon != "" {
		page.Images = []string{category.Icon}
	}
	page.ShareImageWidth, page.ShareImageHeight = metadataDimension(category.SeoMetaJSON["og_image_width"]), metadataDimension(category.SeoMetaJSON["og_image_height"])
	page.SEOTitle, page.SEODescription = metadataText(category.SeoMetaJSON["title"]), metadataText(category.SeoMetaJSON["description"])
	sort.SliceStable(page.Items, func(i, j int) bool {
		a, _ := decimal.NewFromString(page.Items[i].Price)
		b, _ := decimal.NewFromString(page.Items[j].Price)
		return a.LessThan(b)
	})
	guide := publiccontent.GPTRecharge().Resolve(publiccontent.CatalogLinks(ctx))
	if slug == guide.CategorySlug && isMainSite(ctx) {
		page.Title, page.SEOTitle, page.Summary, page.Content = guide.Title, guide.SEOTitle, guide.Summary, guide.HTML()
	}
	return page, nil
}

// Matches the storefront's first-visit language. A visitor's saved language
// continues to be applied by the client; documents never depend on cookies/UA.
func localized(values jsonmap.JSON) string {
	for _, language := range []string{"zh-CN", "en-US"} {
		if value, ok := values[language].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// Older catalog records may store a plain string instead of localized metadata.
func metadataText(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]interface{}:
		return localized(jsonmap.JSON(v))
	case jsonmap.JSON:
		return localized(v)
	default:
		return ""
	}
}

func metadataDimension(value interface{}) int {
	var n int
	switch v := value.(type) {
	case float64:
		n = int(v)
	case int:
		n = v
	case string:
		n, _ = strconv.Atoi(v)
	}
	if n < 1 || n > 10000 {
		return 0
	}
	return n
}
