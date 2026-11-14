// Package publiccontent holds the same editorial copy used by the HTML renderer
// and the interactive storefront. It contains no product, price or private data.
package publiccontent

import (
	"context"
	_ "embed"
	"encoding/json"
	"html"
	"regexp"
	"slices"
	"strings"
)

//go:embed gpt-recharge.json
var rechargeJSON []byte

type Section struct{ Title, HTML string }
type Link struct{ Title, Path string }
type RechargeGuide struct {
	CatalogIdentities                            map[string]string
	CategorySlug, Path, Title, SEOTitle, Summary string
	UpdatedAt                                    string
	Sections                                     []Section
	ProductCategories, BlogSlugs                 []string
	RelatedHeading                               string
	RelatedLinks                                 []Link
}

type catalogLinksKey struct{}

func WithCatalogLinks(ctx context.Context, links map[string]string) context.Context {
	return context.WithValue(ctx, catalogLinksKey{}, links)
}

func CatalogLinks(ctx context.Context) map[string]string {
	links, _ := ctx.Value(catalogLinksKey{}).(map[string]string)
	return links
}

// Resolve binds editorial references to permanent identities, never to a name
// that a different product could already own. Missing targets are not linked.
func (g RechargeGuide) Resolve(links map[string]string) RechargeGuide {
	g.Path = links[g.Path]
	g.CategorySlug = strings.TrimPrefix(g.Path, "/categories/")
	categories := []string{}
	for _, slug := range g.ProductCategories {
		if path := links["/categories/"+slug]; path != "" {
			categories = append(categories, strings.TrimPrefix(path, "/categories/"))
		}
	}
	g.ProductCategories = categories
	for i := range g.Sections {
		g.Sections[i].HTML = g.RewriteLinks(g.Sections[i].HTML, links)
	}
	related := []Link{}
	for _, link := range g.RelatedLinks {
		if _, fixed := g.CatalogIdentities[link.Path]; fixed {
			link.Path = links[link.Path]
		}
		if link.Path != "" {
			related = append(related, link)
		}
	}
	g.RelatedLinks = related
	return g
}

var catalogHref = regexp.MustCompile(`href="([^"]+)"`)

func (g RechargeGuide) RewriteLinks(content string, links map[string]string) string {
	references := map[string]string{}
	for path, identity := range g.CatalogIdentities {
		references[path], references[identity] = path, path
	}
	return catalogHref.ReplaceAllStringFunc(content, func(attribute string) string {
		path := strings.TrimSuffix(strings.TrimPrefix(attribute, `href="`), `"`)
		reference, fixed := references[path]
		if !fixed {
			return attribute
		}
		if target := links[reference]; target != "" {
			return `href="` + html.EscapeString(target) + `"`
		}
		return ""
	})
}

func GPTRecharge() RechargeGuide {
	var guide RechargeGuide
	if err := json.Unmarshal(rechargeJSON, &guide); err != nil {
		panic(err)
	}
	return guide
}

func (g RechargeGuide) HTML() string {
	var result strings.Builder
	for _, section := range g.Sections {
		result.WriteString("<h2>" + html.EscapeString(section.Title) + "</h2>" + section.HTML)
	}
	return result.String()
}

// AppendRelated preserves existing editorial content and avoids duplicate links.
// Callers must exclude reseller sites so main-site copy cannot cross tenants.
func (g RechargeGuide) AppendRelated(content, kind, slug, category string) string {
	permanentProduct := false
	for _, identity := range g.CatalogIdentities {
		permanentProduct = permanentProduct || identity == "/products/"+slug
	}
	if (kind != "product" || (!slices.Contains(g.ProductCategories, category) && !permanentProduct)) &&
		(kind != "article" || !slices.Contains(g.BlogSlugs, slug)) {
		return content
	}
	var links strings.Builder
	for _, link := range g.RelatedLinks {
		if link.Path == "/blog/"+slug || strings.Contains(content, link.Path) {
			continue
		}
		links.WriteString(`<li><a href="` + html.EscapeString(link.Path) + `">` + html.EscapeString(link.Title) + `</a></li>`)
	}
	if links.Len() == 0 {
		return content
	}
	return content + "<h2>" + html.EscapeString(g.RelatedHeading) + "</h2><ul>" + links.String() + "</ul>"
}
