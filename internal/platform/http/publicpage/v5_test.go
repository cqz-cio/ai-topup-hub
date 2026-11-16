package publicpage

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func TestV5SemanticsUseOnlyVisibleQuestionsAndCanonicalBreadcrumbs(t *testing.T) {
	body := `<h3>普通标题</h3><p><strong>不是问题</strong>普通正文</p><h3>购买前常见问题</h3><p><strong>是否已开通？</strong> 需要核对<b>结果</b>。</p><p><strong>如何退款？</strong>联系人工客服。</p><h3>其他说明</h3><p><strong>不进入 FAQ</strong>仍然可见</p>`
	page := Page{Kind: "product", Path: "/products/one", Title: "商品一", Category: "分类", CategoryPath: "/categories/live"}
	var value struct {
		Graph []map[string]any `json:"@graph"`
	}
	if err := json.Unmarshal([]byte(documentSemantics(page, body, "https://www.qzweb.cloud")), &value); err != nil {
		t.Fatal(err)
	}
	if len(value.Graph) != 2 {
		t.Fatal(value)
	}
	crumbs := value.Graph[0]["itemListElement"].([]any)
	if len(crumbs) != 3 || crumbs[1].(map[string]any)["item"] != "https://www.qzweb.cloud/categories/live" {
		t.Fatal(crumbs)
	}
	faqs := value.Graph[1]["mainEntity"].([]any)
	if len(faqs) != 2 || faqs[0].(map[string]any)["name"] != "是否已开通？" {
		t.Fatal(faqs)
	}
	if strings.Contains(string(documentSemantics(page, "<p>没有 FAQ</p>", "https://www.qzweb.cloud")), "FAQPage") {
		t.Fatal("empty FAQ emitted")
	}
	for _, kind := range []string{"article", "notice", "legal", "category"} {
		page.Kind = kind
		page.CategoryPath = ""
		raw := string(documentSemantics(page, "", "https://www.qzweb.cloud"))
		if !strings.Contains(raw, "BreadcrumbList") {
			t.Fatal(kind)
		}
		if kind == "notice" && !strings.Contains(raw, "/notice") {
			t.Fatal(raw)
		}
	}
}

func TestV5DescriptionNeverFallsBackToTitle(t *testing.T) {
	if got := descriptionFor(Page{Title: "服务条款", Content: "<p>下单前阅读适用条件。</p>"}); got != "下单前阅读适用条件。" {
		t.Fatal(got)
	}
	if got := descriptionFor(Page{Title: "服务条款", Summary: "服务条款"}); got != "" {
		t.Fatal(got)
	}
}

func TestV5StylesReadCurrentManifestAndFailSafely(t *testing.T) {
	assets := fstest.MapFS{
		"storefront-assets.json": {Data: []byte(`{"version":1,"styles":["/assets/index-one.css"]}`)},
		"assets/index-one.css":   {Data: []byte("old")},
		"assets/index-two.css":   {Data: []byte("new")},
		"index.html":             {Data: []byte(`<link rel="stylesheet" href="/assets/index-one.css">`)},
	}
	h := New(&sourceStub{}).WithAssets(assets)
	if got := h.stylesheets(); len(got) != 1 || got[0] != "/assets/index-one.css" {
		t.Fatal(got)
	}
	assets["storefront-assets.json"].Data = []byte(`{"version":1,"styles":["/assets/index-two.css"]}`)
	if h.stylesheets()[0] != "/assets/index-two.css" {
		t.Fatal("cached startup manifest")
	}
	for _, raw := range []string{`invalid`, `{"version":1,"styles":["https://evil.test/style.css"]}`, `{"version":1,"styles":["/assets/../secret.css"]}`, `{"version":1,"styles":["/assets/gone-three.css"]}`} {
		assets["storefront-assets.json"].Data = []byte(raw)
		got := h.stylesheets()
		if len(got) != 2 || got[0] != "/assets/index-one.css" {
			t.Fatal(got)
		}
	}
	delete(assets, "index.html")
	delete(assets, "storefront-assets.json")
	if got := h.stylesheets(); len(got) != 1 || got[0] != "/storefront-entry.css" {
		t.Fatal(got)
	}
}

func TestV5RenderedCategoryLinkDimensionsAndFooter(t *testing.T) {
	s := &sourceStub{page: Page{Kind: "product", Path: "/products/one", Title: "商品", Category: "分类", CategoryPath: "/categories/live", Images: []string{"/media/seo-share/catalog-d7d4c32af9652177616a-1200x630.jpg"}}}
	body := request(t, s, "GET", s.page.Path, "Googlebot").Body.String()
	for _, expected := range []string{`分类：<a href="/categories/live">分类</a>`, `width="1200" height="630"`, `<footer`, `href="/notice"`} {
		if !strings.Contains(body, expected) {
			t.Fatal(expected)
		}
	}
	s.page.CategoryPath = ""
	body = request(t, s, "GET", s.page.Path, "browser").Body.String()
	if strings.Contains(body, `href=""`) || !strings.Contains(body, `分类：分类`) {
		t.Fatal(body)
	}
	footer := body[strings.Index(body, "<footer"):]
	for _, path := range []string{"/cart", "/checkout", "/me"} {
		if strings.Contains(footer, `href="`+path) {
			t.Fatal("private link in footer")
		}
	}
}
