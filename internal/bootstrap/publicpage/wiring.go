package publicpagewiring

import (
	"context"
	"errors"
	"net/url"
	"sort"
	"strings"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	producthttp "github.com/dujiao-next/internal/modules/catalog/product/transport/http"
	contentcontract "github.com/dujiao-next/internal/modules/content/contract"
	contenthttp "github.com/dujiao-next/internal/modules/content/transport/http"
	resellerapp "github.com/dujiao-next/internal/modules/reseller/application"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	settingsapp "github.com/dujiao-next/internal/modules/settings/application"
	"github.com/dujiao-next/internal/platform/http/publicpage"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type source struct {
	products *producthttp.PublicHandler
	posts    *contenthttp.PublicHandler
	settings *settingsapp.Service
	overlay  *resellerapp.SiteConfigService
}

func New(c *container.Container, products *producthttp.PublicHandler, posts *contenthttp.PublicHandler) *publicpage.Handler {
	return publicpage.New(&source{products: products, posts: posts, settings: c.SettingService, overlay: c.ResellerSiteConfigService})
}

func (s *source) Site(ctx context.Context) (publicpage.Site, error) {
	config, err := s.settings.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return publicpage.Site{}, err
	}
	currency, err := s.settings.GetSiteCurrency("CNY")
	if err != nil {
		return publicpage.Site{}, err
	}
	data := map[string]interface{}{"brand": config["brand"], "seo": config["seo"]}
	if s.overlay != nil {
		tenant, _ := reseller.TenantFromContext(ctx)
		data, err = s.overlay.ApplyPublicConfigOverlay(ctx, tenant, data)
		if err != nil {
			return publicpage.Site{}, err
		}
	}
	values, _ := data["brand"].(map[string]interface{})
	name, _ := values["site_name"].(string)
	origin, _ := values["site_url"].(string)
	if seo, ok := data["seo"].(map[string]interface{}); ok && strings.TrimSpace(name) == "" {
		if titles, ok := seo["title"].(map[string]interface{}); ok {
			if title, ok := titles["zh-CN"].(string); ok && strings.TrimSpace(title) != "" {
				name = title
			}
		}
	}
	return publicpage.Site{Name: name, URL: origin, Currency: currency}, nil
}

func (s *source) Product(ctx context.Context, slug string) (publicpage.Page, error) {
	p, err := s.products.ReadAnonymousProduct(ctx, slug)
	if errors.Is(err, productcontract.ErrNotFound) || errors.Is(err, productcontract.ErrResellerProductNotListed) {
		return publicpage.Page{}, publicpage.ErrNotFound
	}
	if err != nil {
		return publicpage.Page{}, err
	}
	price := p.PriceAmount
	if p.EffectivePriceAmount != nil {
		price = *p.EffectivePriceAmount
	}
	page := publicpage.Page{Kind: "product", Path: "/products/" + url.PathEscape(p.Slug), Title: localized(p.Title), Summary: localized(p.Description), Content: localized(p.Content), Price: price.String(), Category: localized(p.Category.Name), Images: p.Images, Availability: "https://schema.org/InStock"}
	page.SEOTitle = metadataText(p.SeoMeta["title"])
	page.SEODescription = metadataText(p.SeoMeta["description"])
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
