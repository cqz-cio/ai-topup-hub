package publicpage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/net/html"
)

type sourceStub struct {
	page             Page
	err, siteErr     error
	returnPolicyPath string
}

func (s *sourceStub) Site(context.Context) (Site, error) {
	return Site{Name: "青栀商城", URL: "http://127.0.0.1:4170", Currency: "CNY", ReturnPolicyPath: s.returnPolicyPath}, s.siteErr
}
func (s *sourceStub) Product(context.Context, string) (Page, error)  { return s.page, s.err }
func (s *sourceStub) Post(context.Context, string) (Page, error)     { return s.page, s.err }
func (s *sourceStub) Category(context.Context, string) (Page, error) { return s.page, s.err }

func request(t *testing.T, source *sourceStub, method, path, agent string) *httptest.ResponseRecorder {
	t.Helper()
	r := gin.New()
	New(source).Register(r, fstest.MapFS{"products/icon.svg": {Data: []byte("<svg></svg>")}})
	req := httptest.NewRequest(method, "https://www.qzweb.cloud"+path, nil)
	req.Header.Set("User-Agent", agent)
	req.Header.Set("Authorization", "Bearer never-render-this-token")
	req.Header.Set("Cookie", "order_lookup=never-render-this-cookie")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestVisibleFullDocumentAndStructuredDataWithoutJavaScript(t *testing.T) {
	published := time.Date(2026, 11, 5, 1, 0, 0, 0, time.UTC)
	for _, kind := range []string{"product", "article"} {
		t.Run(kind, func(t *testing.T) {
			path := "/products/item"
			if kind == "article" {
				path = "/blog/guide"
			}
			s := &sourceStub{page: Page{Kind: kind, Path: path, Title: "完整标题", Summary: "摘要 &amp; 介绍", Content: "<h2>购买条件</h2><p>第一段完整正文</p><table><tr><td>表格内容</td></tr></table><p>最后一段完整正文</p><a href=\"/blog/help\">说明链接</a>", Price: "138.00", Availability: "https://schema.org/InStock", PublishedAt: &published, Images: []string{"/uploads/cover.webp"}}}
			w := request(t, s, "GET", path+"?tracking=ignored", "plain-http-client")
			if w.Code != 200 {
				t.Fatalf("status=%d", w.Code)
			}
			body := w.Body.String()
			for _, want := range []string{"<main id=\"public-document\">", "<h1>完整标题</h1>", "第一段完整正文", "最后一段完整正文", "<table>", "表格内容", "href=\"/blog/help\"", "https://www.qzweb.cloud" + path, "https://www.qzweb.cloud/uploads/cover.webp", "/storefront-entry.js", "/storefront-entry.css"} {
				if !strings.Contains(body, want) {
					t.Errorf("missing %q", want)
				}
			}
			for _, forbidden := range []string{"127.0.0.1:4170", "tracking=ignored", "never-render-this", "&amp;amp;", "<noscript", `href="/blog/help" rel="nofollow"`} {
				if strings.Contains(body, forbidden) {
					t.Errorf("unexpected %q", forbidden)
				}
			}
			doc, err := html.Parse(strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			var schema map[string]any
			var walk func(*html.Node)
			walk = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "script" {
					for _, a := range n.Attr {
						if a.Key == "type" && a.Val == "application/ld+json" {
							if err := json.Unmarshal([]byte(n.FirstChild.Data), &schema); err != nil {
								t.Error(err)
							}
						}
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					walk(c)
				}
			}
			walk(doc)
			if schema["url"] != "https://www.qzweb.cloud"+path {
				t.Fatalf("schema=%v", schema)
			}
			if kind == "product" {
				offer := schema["offers"].(map[string]any)
				if offer["price"] != "138.00" || offer["priceCurrency"] != "CNY" {
					t.Fatal(offer)
				}
			} else if schema["datePublished"] != published.Format(time.RFC3339) {
				t.Fatal(schema)
			}
			if got := request(t, s, "GET", path+"?tracking=ignored", "Googlebot").Body.String(); got != body {
				t.Fatal("crawler received different content")
			}
			if w.Header().Get("Cache-Control") != "private, no-store" {
				t.Fatal("live document can be cached")
			}
			head := request(t, s, "HEAD", path, "")
			if head.Code != 200 || head.Body.Len() != 0 || head.Header().Get("Content-Type") != w.Header().Get("Content-Type") || head.Header().Get("Content-Length") != w.Header().Get("Content-Length") {
				t.Fatal("HEAD contract broken")
			}
		})
	}
}

func TestUntrustedRichTextCannotExecuteOrEscapeStructuredData(t *testing.T) {
	s := &sourceStub{page: Page{Kind: "article", Path: "/blog/xss", Title: `</script><script>alert('title')</script>`, Content: `<p onclick="alert('event')">保留正文</p><script>alert('body')</script><iframe src="https://bad.invalid/"></iframe><img src="/uploads/ok.png" onerror="alert('image')"><a href="javascript:alert('url')">危险链接文字</a>`, Images: []string{"javascript:alert(1)"}}}
	body := request(t, s, "GET", "/blog/xss", "").Body.String()
	for _, bad := range []string{"<script>alert", "onclick=", "onerror=", "javascript:", "<iframe", "alert('body')"} {
		if strings.Contains(body, bad) {
			t.Errorf("unsafe HTML: %q", bad)
		}
	}
	if !strings.Contains(body, "保留正文") || !strings.Contains(body, "危险链接文字") || !strings.Contains(body, `\u003c/script\u003e`) {
		t.Fatal("safe content or JSON escaping missing")
	}
}

func TestMissingDraftAndUnavailableDocumentsKeepRealHTTPStatus(t *testing.T) {
	for _, tc := range []struct {
		name         string
		err, siteErr error
		status       int
	}{{"missing-or-unpublished", ErrNotFound, nil, 404}, {"content-failure", errors.New("private DB detail"), nil, 503}, {"settings-failure", nil, errors.New("private config detail"), 503}} {
		t.Run(tc.name, func(t *testing.T) {
			for _, method := range []string{"GET", "HEAD"} {
				w := request(t, &sourceStub{err: tc.err, siteErr: tc.siteErr}, method, "/blog/unavailable", "")
				if w.Code != tc.status || w.Header().Get("X-Robots-Tag") != "noindex" || w.Header().Get("Cache-Control") != "private, no-store" {
					t.Fatalf("%d %v", w.Code, w.Header())
				}
				if strings.Contains(w.Body.String(), "private") || strings.Contains(w.Body.String(), "storefront-entry") {
					t.Fatal("error leaks details or loads a SPA success fallback")
				}
				if method == "HEAD" && w.Body.Len() != 0 {
					t.Fatal("HEAD has body")
				}
			}
		})
	}
}

func TestEmbeddedProductImageIsNotShadowedByHTMLRoute(t *testing.T) {
	w := request(t, &sourceStub{err: ErrNotFound}, http.MethodGet, "/products/icon.svg", "")
	if w.Code != 200 || w.Body.String() != "<svg></svg>" || !strings.Contains(w.Header().Get("Content-Type"), "image/svg+xml") {
		t.Fatalf("%d %v %s", w.Code, w.Header(), w.Body)
	}
}

func TestProductMetadataDoesNotRenameVisibleProduct(t *testing.T) {
	s := &sourceStub{page: Page{Kind: "product", Path: "/products/item", Title: "原商品名", Summary: "商品卡片摘要", SEOTitle: "Plus 兑换码与购买条件", SEODescription: "<b>专属摘要</b> &amp; 条件", Content: "<p>完整条件</p>", Price: "130.00"}}
	body := request(t, s, "GET", "/products/item", "").Body.String()
	for _, want := range []string{`<title data-public-page-seo>Plus 兑换码与购买条件 - 青栀商城</title>`, `<h1>原商品名</h1>`, `name="description" content="专属摘要 &amp; 条件"`, `"name":"原商品名"`, `"description":"专属摘要 \u0026 条件"`, `商品卡片摘要`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
	s.page.SEOTitle, s.page.SEODescription = "", ""
	body = request(t, s, "GET", "/products/item", "").Body.String()
	if !strings.Contains(body, `<title data-public-page-seo>原商品名 - 青栀商城</title>`) || !strings.Contains(body, `name="description" content="商品卡片摘要"`) {
		t.Fatal("empty metadata must fall back to the public product copy")
	}
}

func TestCategoryDocumentHasVisiblePricesLinksAndCollectionSchema(t *testing.T) {
	s := &sourceStub{page: Page{Kind: "category", Path: "/categories/gpt", Title: "GPT 充值", SEOTitle: "GPT充值与购买须知", Summary: "核对账号条件", Content: `<h2>购买前</h2><p>不是官方授权</p><a href="/blog/help">教程</a>`, Items: []Item{
		{Title: "Plus", Path: "/products/plus", Price: "130.00", Availability: "https://schema.org/InStock"},
		{Title: "Pro", Path: "/products/pro", Price: "700.00", Availability: "https://schema.org/OutOfStock"},
	}}}
	w := request(t, s, "GET", "/categories/gpt?source=test", "")
	body := w.Body.String()
	for _, want := range []string{`href="https://www.qzweb.cloud/categories/gpt"`, `href="/products/plus"`, `href="/products/pro"`, "130.00 CNY", "暂时缺货", "核对账号条件", "不是官方授权", `"@type":"CollectionPage"`, `"@type":"ItemList"`, `content="website"`} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %s", want)
		}
	}
	for _, agent := range []string{"Googlebot", "bingbot", "OAI-SearchBot"} {
		if got := request(t, s, "GET", "/categories/gpt?source=test", agent).Body.String(); got != body {
			t.Fatal("UA-dependent content")
		}
	}
	if head := request(t, s, "HEAD", "/categories/gpt", ""); head.Code != 200 || head.Body.Len() != 0 {
		t.Fatal("HEAD contract")
	}
	for _, tc := range []struct {
		err    error
		status int
	}{{ErrNotFound, 404}, {errors.New("private failure"), 503}} {
		w := request(t, &sourceStub{err: tc.err}, "GET", "/categories/gpt", "")
		if w.Code != tc.status || w.Header().Get("X-Robots-Tag") != "noindex" {
			t.Fatal("incorrect category error semantics")
		}
	}
}
