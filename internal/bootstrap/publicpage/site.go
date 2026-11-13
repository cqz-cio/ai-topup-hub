package publicpagewiring

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/constants"
	productcontract "github.com/dujiao-next/internal/modules/catalog/product/contract"
	contentapp "github.com/dujiao-next/internal/modules/content/application"
	reseller "github.com/dujiao-next/internal/modules/reseller/contract"
	"github.com/dujiao-next/internal/platform/http/publicpage"
	"github.com/dujiao-next/internal/platform/publiccontent"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// Only public presentation fields may enter document rendering. Never pass the
// raw site configuration (scripts, payment, SMTP, integrations) into a template.
func (s *source) publicConfig(ctx context.Context, about bool) (map[string]any, error) {
	config, err := s.settings.GetByKey(constants.SettingKeySiteConfig)
	if err != nil {
		return nil, err
	}
	data := map[string]any{"brand": config["brand"], "seo": config["seo"], "about": config["about"], "contact": config["contact"]}
	legal := configMap(config["legal"])
	data["legal"] = map[string]any{"terms": legal["terms"], "privacy": legal["privacy"]}
	// Normalize JSON aliases and detach nested maps before tenant overlays.
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, err
	}
	if err = json.Unmarshal(raw, &data); err != nil {
		return nil, err
	}
	if s.overlay != nil {
		tenant, _ := reseller.TenantFromContext(ctx)
		data, err = s.overlay.ApplyPublicConfigOverlay(ctx, tenant, data)
		if err != nil {
			return nil, err
		}
	}
	if about && isMainSite(ctx) {
		page, err := s.settings.GetByKey(constants.SettingKeyAboutPageConfig)
		if err != nil {
			return nil, err
		}
		data["about_page"] = map[string]any(page)
	}
	return data, nil
}

func configMap(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	case jsonmap.JSON:
		return map[string]any(v)
	}
	return nil
}

func (s *source) Home(ctx context.Context) (publicpage.Page, error) {
	config, err := s.publicConfig(ctx, false)
	if err != nil {
		return publicpage.Page{}, err
	}
	seo, brand := configMap(config["seo"]), configMap(config["brand"])
	page := publicpage.Page{Kind: "home", Path: "/", Title: metadataText(brand["site_name"]), SEOTitle: metadataText(seo["title"]), SEODescription: metadataText(seo["description"])}
	if isMainSite(ctx) {
		page.Title, page.Summary, page.Content = publiccontent.HomeCopy()
		page.Links = []publicpage.Link{{Title: "GPT 充值方案与购买须知", Path: publiccontent.GPTRecharge().Path}}
	} else {
		page.Summary = page.SEODescription
	}
	page.Items, err = s.catalogItems(ctx, "")
	return page, err
}

func (s *source) About(ctx context.Context) (publicpage.Page, error) {
	config, err := s.publicConfig(ctx, true)
	if err != nil {
		return publicpage.Page{}, err
	}
	title, summary, body := publiccontent.AboutCopy(configMap(config["about_page"]), configMap(config["about"]), configMap(config["contact"]), isMainSite(ctx))
	if title == "" {
		title = "关于我们"
	}
	return publicpage.Page{Kind: "about", Path: "/about", Title: title, SEOTitle: "关于", Summary: summary, Content: body}, nil
}

func (s *source) BlogList(ctx context.Context, number int, search string) (publicpage.Page, error) {
	return s.postList(ctx, constants.PostTypeBlog, number, search)
}

func (s *source) NoticeList(ctx context.Context, number int, search string) (publicpage.Page, error) {
	return s.postList(ctx, constants.PostTypeNotice, number, search)
}

func (s *source) postList(ctx context.Context, postType string, number int, search string) (publicpage.Page, error) {
	const pageSize = 12
	posts, total, err := s.posts.ReadPublishedPosts(ctx, contentapp.PublicPostQuery{Type: postType, Search: search, Page: number, PageSize: pageSize})
	if err != nil {
		return publicpage.Page{}, err
	}
	if number > 1 && int64((number-1)*pageSize) >= total {
		return publicpage.Page{}, publicpage.ErrNotFound
	}
	basePath, kind, title, summary := "/blog", "blog", "博客", "最新文章和动态"
	if postType == constants.PostTypeNotice {
		basePath, kind, title, summary = "/notice", "notice-list", "公告", "购买、交付与售后服务公告"
	}
	pagePath := func(n int, filter string) string {
		q := url.Values{}
		if n > 1 {
			q.Set("page", strconv.Itoa(n))
		}
		if filter != "" {
			q.Set("search", filter)
		}
		if len(q) > 0 {
			return basePath + "?" + q.Encode()
		}
		return basePath
	}
	page := publicpage.Page{Kind: kind, Path: pagePath(number, ""), Title: title, Summary: summary, NoIndex: search != ""}
	if page.NoIndex {
		page.Path = basePath
	}
	for _, post := range posts {
		page.Entries = append(page.Entries, publicpage.Entry{Title: localized(post.Title), Path: "/blog/" + url.PathEscape(post.Slug), Summary: localized(post.Summary), PublishedAt: post.PublishedAt})
	}
	if number > 1 {
		page.Pagination = append(page.Pagination, publicpage.Link{Title: "上一页", Path: pagePath(number-1, search)})
	}
	if int64(number*pageSize) < total {
		page.Pagination = append(page.Pagination, publicpage.Link{Title: "下一页", Path: pagePath(number+1, search)})
	}
	return page, nil
}

func (s *source) catalogItems(ctx context.Context, category string) ([]publicpage.Item, error) {
	var items []publicpage.Item
	for number := 1; ; number++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		rows, total, err := s.catalog.ListAnonymousProductSlugs(ctx, category, number, 100)
		if err != nil {
			return nil, err
		}
		for _, slug := range rows {
			item, err := s.products.ReadAnonymousProduct(ctx, slug)
			if errors.Is(err, productcontract.ErrNotFound) || errors.Is(err, productcontract.ErrResellerProductNotListed) {
				continue
			}
			if err != nil {
				return nil, err
			}
			price := item.PriceAmount
			if item.EffectivePriceAmount != nil {
				price = *item.EffectivePriceAmount
			}
			availability := "https://schema.org/InStock"
			if item.IsSoldOut {
				availability = "https://schema.org/OutOfStock"
			}
			items = append(items, publicpage.Item{Title: localized(item.Title), Path: "/products/" + url.PathEscape(item.Slug), Price: price.String(), Availability: availability, Summary: localized(item.Description), Category: localized(item.Category.Name)})
		}
		if len(rows) == 0 || int64(number*100) >= total {
			break
		}
	}
	return items, nil
}

func isEditorial(slug string) bool {
	for _, candidate := range publiccontent.GPTRecharge().BlogSlugs {
		if strings.EqualFold(slug, candidate) {
			return true
		}
	}
	return false
}

// Products reuses the same anonymous projection as the home and category pages.
func (s *source) Products(ctx context.Context) (publicpage.Page, error) {
	items, err := s.catalogItems(ctx, "")
	return publicpage.Page{Kind: "products", Path: "/products", Title: "商品中心", Summary: "使用期限与质保期不同，购买前请分别确认。", Items: items}, err
}

func (s *source) Terms(ctx context.Context) (publicpage.Page, error) {
	return s.legal(ctx, "terms", "服务条款")
}

func (s *source) Privacy(ctx context.Context) (publicpage.Page, error) {
	return s.legal(ctx, "privacy", "隐私政策")
}

func (s *source) legal(ctx context.Context, key, title string) (publicpage.Page, error) {
	config, err := s.publicConfig(ctx, false)
	if err != nil {
		return publicpage.Page{}, err
	}
	body := metadataText(configMap(config["legal"])[key])
	if strings.TrimSpace(body) == "" {
		return publicpage.Page{}, publicpage.ErrNotFound
	}
	return publicpage.Page{Kind: "legal", Path: "/" + key, Title: title, Content: body}, nil
}
