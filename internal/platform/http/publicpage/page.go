// Package publicpage serves live, anonymous HTML documents for public detail URLs.
// It has no access to domain models, credentials, orders or delivery data.
package publicpage

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"html"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dujiao-next/internal/platform/http/siteurl"
	"github.com/gin-gonic/gin"
	"github.com/microcosm-cc/bluemonday"
)

var ErrNotFound = errors.New("public document not found")

type Site struct{ Name, URL, Currency, Logo, Description, ReturnPolicyPath string }
type Link struct{ Title, Path string }
type Fact struct{ Name, Value string }
type Item struct{ Title, Path, Price, Availability, Summary, Category string }
type Entry struct {
	Title, Path, Summary string
	PublishedAt          *time.Time
}
type Page struct {
	Kind, Path, Title, Summary, Content string
	SEOTitle, SEODescription            string
	Price, Category, Availability       string
	Images                              []string
	Facts                               []Fact
	Links                               []Link
	Items                               []Item
	PublishedAt                         *time.Time
	Entries                             []Entry
	Pagination                          []Link
	NoIndex, Editorial                  bool
}

type Source interface {
	Site(context.Context) (Site, error)
	Product(context.Context, string) (Page, error)
	Post(context.Context, string) (Page, error)
	Category(context.Context, string) (Page, error)
}

type SitePages interface {
	Home(context.Context) (Page, error)
	About(context.Context) (Page, error)
	BlogList(context.Context, int, string) (Page, error)
}

type Handler struct {
	source Source
	policy *bluemonday.Policy
}

func New(source Source) *Handler {
	// Keep editorial links between our pages crawlable, while retaining nofollow
	// on fully qualified external links and the sanitizer's URL safety rules.
	policy := bluemonday.UGCPolicy().RequireNoFollowOnLinks(false).RequireNoFollowOnFullyQualifiedLinks(true)
	return &Handler{source: source, policy: policy}
}

// Register also preserves /products/* image files in embedded/fullstack mode.
// The split frontend gateway performs the equivalent file check before proxying.
func (h *Handler) Register(routes gin.IRoutes, assets fs.FS) {
	if source, ok := h.source.(SitePages); ok {
		for path, load := range map[string]func(context.Context) (Page, error){"/": source.Home, "/about": source.About} {
			handler := func(c *gin.Context) {
				h.serve(c, func(ctx context.Context, _ string) (Page, error) { return load(ctx) })
			}
			routes.GET(path, handler)
			routes.HEAD(path, handler)
		}
		list := func(c *gin.Context) {
			h.serve(c, func(ctx context.Context, _ string) (Page, error) {
				page := 1
				if raw := c.Query("page"); raw != "" {
					value, err := strconv.Atoi(raw)
					if err != nil || value < 1 || value > 100000 {
						return Page{}, ErrNotFound
					}
					page = value
				}
				search := strings.TrimSpace(c.Query("search"))
				if len([]rune(search)) > 200 {
					return Page{}, ErrNotFound
				}
				return source.BlogList(ctx, page, search)
			})
		}
		routes.GET("/blog", list)
		routes.HEAD("/blog", list)
	}
	for _, kind := range []string{"products", "blog", "categories"} {
		load := h.source.Product
		if kind == "blog" {
			load = h.source.Post
		}
		if kind == "categories" {
			load = h.source.Category
		}
		handler := func(c *gin.Context) {
			if assets != nil {
				if info, err := fs.Stat(assets, strings.TrimPrefix(c.Request.URL.Path, "/")); err == nil && !info.IsDir() {
					c.Header("Cache-Control", "no-cache")
					http.FileServer(http.FS(assets)).ServeHTTP(c.Writer, c.Request)
					return
				}
			}
			h.serve(c, load)
		}
		routes.GET("/"+kind+"/:slug", handler)
		routes.HEAD("/"+kind+"/:slug", handler)
	}
}

func (h *Handler) serve(c *gin.Context, load func(context.Context, string) (Page, error)) {
	c.Header("Cache-Control", "private, no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Content-Language", "zh-CN")
	ctx := c.Request.Context()
	page, err := load(ctx, c.Param("slug"))
	if err != nil {
		h.failure(c, err)
		return
	}
	site, err := h.source.Site(ctx)
	if err != nil {
		h.failure(c, err)
		return
	}
	if site.Name == "" {
		site.Name = "商城"
	}
	origin := siteurl.Resolve(site.URL, c.Request)
	canonical := origin + page.Path
	images := make([]string, 0, len(page.Images))
	for _, raw := range page.Images {
		if image := absoluteImage(origin, raw); image != "" {
			images = append(images, image)
		}
	}
	data := document{
		Page: page, Site: site, Canonical: canonical, Images: images,
		Body:        template.HTML(h.policy.Sanitize(page.Content)), // Only sanitized UGC is trusted.
		Summary:     template.HTML(h.policy.Sanitize(page.Summary)),
		Description: plainText(page.Summary),
	}
	if description := plainText(page.SEODescription); description != "" {
		data.Description = description
	}
	if data.Description == "" {
		data.Description = page.Title
	}
	data.MetadataTitle = strings.TrimSpace(page.SEOTitle)
	if data.MetadataTitle == "" {
		data.MetadataTitle = page.Title
	}
	if site.Name != "" && data.MetadataTitle != site.Name && page.Kind != "home" {
		data.MetadataTitle += " - " + site.Name
	}
	schema := map[string]any{"@context": "https://schema.org", "@type": "BlogPosting", "headline": page.Title, "url": canonical, "mainEntityOfPage": canonical, "description": data.Description, "inLanguage": "zh-CN"}
	if page.Kind == "notice" {
		schema["@type"] = "Article"
	}
	if page.Kind == "home" || page.Kind == "about" || page.Kind == "blog" {
		kind := map[string]string{"home": "WebPage", "about": "AboutPage", "blog": "CollectionPage"}[page.Kind]
		schema = map[string]any{"@context": "https://schema.org", "@type": kind, "name": page.Title, "url": canonical, "description": data.Description, "inLanguage": "zh-CN", "isPartOf": map[string]string{"@id": origin + "/#website"}, "about": map[string]string{"@id": origin + "/#organization"}}
		if page.Kind == "blog" {
			items := make([]map[string]any, 0, len(page.Entries))
			for i, entry := range page.Entries {
				items = append(items, map[string]any{"@type": "ListItem", "position": i + 1, "name": entry.Title, "url": origin + entry.Path})
			}
			schema["mainEntity"] = map[string]any{"@type": "ItemList", "itemListElement": items}
		}
	}
	if page.Kind == "category" {
		items := make([]map[string]any, 0, len(page.Items))
		for i, item := range page.Items {
			items = append(items, map[string]any{"@type": "ListItem", "position": i + 1, "name": item.Title, "url": origin + item.Path})
		}
		schema = map[string]any{"@context": "https://schema.org", "@type": "CollectionPage", "name": page.Title, "url": canonical, "description": data.Description, "mainEntity": map[string]any{"@type": "ItemList", "itemListElement": items}}
	}
	if page.Kind == "product" {
		offer := map[string]any{"@type": "Offer", "@id": canonical + "#offer", "price": page.Price, "priceCurrency": site.Currency, "availability": page.Availability, "url": canonical, "seller": map[string]string{"@id": origin + "/#organization"}}
		if site.ReturnPolicyPath != "" {
			offer["hasMerchantReturnPolicy"] = map[string]string{"@id": origin + site.ReturnPolicyPath + "#merchant-return-policy"}
		}
		schema = map[string]any{"@context": "https://schema.org", "@type": "Product", "@id": canonical + "#product", "name": page.Title, "url": canonical, "description": data.Description, "offers": offer}
		if page.Category != "" {
			schema["category"] = page.Category
		}
	} else if page.PublishedAt != nil {
		schema["datePublished"] = page.PublishedAt.Format(time.RFC3339)
	}
	if len(images) > 0 {
		schema["image"] = images
	}
	if page.Kind == "article" || page.Kind == "notice" {
		schema["publisher"] = map[string]string{"@id": origin + "/#organization"}
		if page.Editorial {
			schema["author"] = map[string]string{"@type": "Organization", "@id": origin + "/#organization", "name": site.Name, "url": origin + "/about"}
		}
	}
	raw, err := json.Marshal(schema) // Escapes <, > and &; content cannot terminate the script.
	if err != nil {
		h.failure(c, err)
		return
	}
	data.Schema = template.JS(raw)
	identity := map[string]any{"@type": "Organization", "@id": origin + "/#organization", "name": site.Name, "url": origin + "/", "contactPoint": map[string]any{"@type": "ContactPoint", "contactType": "customer support", "url": origin + "/about"}}
	if site.ReturnPolicyPath != "" {
		identity["hasMerchantReturnPolicy"] = map[string]string{"@type": "MerchantReturnPolicy", "@id": origin + site.ReturnPolicyPath + "#merchant-return-policy", "merchantReturnLink": origin + site.ReturnPolicyPath}
	}
	if logo := absoluteImage(origin, site.Logo); logo != "" {
		identity["logo"] = logo
	}
	if site.Description != "" {
		identity["description"] = site.Description
	}
	identityRaw, err := json.Marshal(map[string]any{"@context": "https://schema.org", "@graph": []any{identity, map[string]any{"@type": "WebSite", "@id": origin + "/#website", "url": origin + "/", "name": site.Name, "publisher": map[string]string{"@id": origin + "/#organization"}}}})
	if err != nil {
		h.failure(c, err)
		return
	}
	data.Identity = template.JS(identityRaw)
	if page.NoIndex {
		c.Header("X-Robots-Tag", "noindex, follow")
	}
	var out bytes.Buffer
	if err := documentTemplate.Execute(&out, data); err != nil {
		h.failure(c, err)
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", headBody(c, out.Bytes()))
}

func (h *Handler) failure(c *gin.Context, err error) {
	status, title := http.StatusServiceUnavailable, "内容暂时无法加载，请稍后重试"
	if errors.Is(err, ErrNotFound) {
		status, title = http.StatusNotFound, "内容不存在或尚未公开"
	} else {
		c.Header("Retry-After", "60")
	}
	c.Header("X-Robots-Tag", "noindex")
	c.Data(status, "text/html; charset=utf-8", headBody(c, []byte(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="robots" content="noindex"><title>`+title+`</title></head><body><h1>`+title+`</h1><a href="/">返回首页</a></body></html>`)))
}

func headBody(c *gin.Context, body []byte) []byte {
	c.Header("Content-Length", strconv.Itoa(len(body)))
	if c.Request.Method == http.MethodHead {
		return nil
	}
	return body
}

func absoluteImage(origin, raw string) string {
	base, _ := url.Parse(origin + "/")
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || raw == "" {
		return ""
	}
	u = base.ResolveReference(u)
	if u.User != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return u.String()
}

func plainText(raw string) string {
	return strings.Join(strings.Fields(html.UnescapeString(bluemonday.StrictPolicy().Sanitize(raw))), " ")
}
