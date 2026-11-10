// Package publiccontent holds the same editorial copy used by the HTML renderer
// and the interactive storefront. It contains no product, price or private data.
package publiccontent

import (
	_ "embed"
	"encoding/json"
	"html"
	"slices"
	"strings"
)

//go:embed gpt-recharge.json
var rechargeJSON []byte

type Section struct{ Title, HTML string }
type Link struct{ Title, Path string }
type RechargeGuide struct {
	CategorySlug, Path, Title, SEOTitle, Summary string
	UpdatedAt                                    string
	Sections                                     []Section
	ProductCategories, BlogSlugs                 []string
	RelatedHeading                               string
	RelatedLinks                                 []Link
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
	if (kind != "product" || !slices.Contains(g.ProductCategories, category)) &&
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
