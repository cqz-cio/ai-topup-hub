package publicpage

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type sharingSite struct{ siteStub }

func (*sharingSite) Site(context.Context) (Site, error) {
	return Site{Name: "π星卡社", URL: "https://www.qzweb.cloud", Icon: "/icon.png", DefaultImage: "/media/seo-share/qzweb-og-1200x630.jpg"}, nil
}

func TestPublicSharingAndSanitizedPolicyHTML(t *testing.T) {
	s := &sharingSite{siteStub{sourceStub: sourceStub{page: Page{Kind: "legal", Path: "/terms", Title: "服务条款", Images: []string{"/cover.svg"}, Content: `<h2>公开条款</h2><script>privateScript()</script><img src=x onerror="privateHandler()">`}}}}
	r := gin.New()
	New(s).Register(r, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/terms", nil))
	body := w.Body.String()
	for _, want := range []string{`name="twitter:card" content="summary_large_image"`, `name="twitter:image" content="https://www.qzweb.cloud/media/seo-share/qzweb-og-1200x630.jpg"`, `property="og:image" content="https://www.qzweb.cloud/media/seo-share/qzweb-og-1200x630.jpg"`, `rel="icon" href="https://www.qzweb.cloud/icon.png"`, `"@type":"WebPage"`, `<h2>公开条款</h2>`} {
		if !strings.Contains(body, want) {
			t.Fatal("missing", want, body)
		}
	}
	if w.Code != 200 || strings.Contains(body, "privateScript") || strings.Contains(body, "privateHandler") || strings.Contains(body, `"@type":"BlogPosting"`) {
		t.Fatal("invalid policy", body)
	}
}

func TestPublicTrailingSlashUsesPermanentRedirectAndPreservesQuery(t *testing.T) {
	r := gin.New()
	New(&siteStub{}).Register(r, nil)
	for _, path := range []string{"/products/", "/notice/", "/terms/", "/privacy/", "/products/test/", "/blog/test/", "/categories/test/"} {
		for _, method := range []string{"GET", "HEAD"} {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(method, path+"?page=2&x=1", nil))
			if w.Code != 301 || w.Header().Get("Location") != strings.TrimSuffix(path, "/")+"?page=2&x=1" {
				t.Fatal(path, method, w.Code, w.Header())
			}
			if method == "HEAD" && w.Body.Len() != 0 {
				t.Fatal("HEAD redirect body")
			}
		}
	}
}

func TestAnonymousHTMLCacheAndPrivateBypasses(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		for _, name := range []string{"anonymous", "cookie", "auth", "search", "empty-search", "noindex", "missing", "error"} {
			t.Run(method+"/"+name, func(t *testing.T) {
				s := &sourceStub{page: Page{Kind: "product", Path: "/products/item", Title: "公开商品"}}
				path := "/products/item"
				if name == "search" {
					path += "?search=gemini"
				}
				if name == "empty-search" {
					path += "?search="
				}
				if name == "noindex" {
					s.page.NoIndex = true
				}
				if name == "missing" {
					s.err = ErrNotFound
				}
				if name == "error" {
					s.siteErr = ErrNotFound
				}
				r := gin.New()
				New(s).Register(r, nil)
				req := httptest.NewRequest(method, path, nil)
				if name == "cookie" {
					req.Header.Set("Cookie", "session=example")
				}
				if name == "auth" {
					req.Header.Set("Authorization", "Bearer example")
				}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				want := "private, no-store"
				if name == "anonymous" {
					want = "public, max-age=60, s-maxage=120, stale-while-revalidate=300"
				}
				if w.Header().Get("Cache-Control") != want {
					t.Fatal(w.Code, w.Header())
				}
				if method == "HEAD" && w.Body.Len() != 0 {
					t.Fatal("HEAD body")
				}
			})
		}
	}
}

func TestCanonicalAliasRedirectsAndMissingAliases(t *testing.T) {
	for _, method := range []string{"GET", "HEAD"} {
		for _, kind := range []string{"products", "categories"} {
			s := &sourceStub{err: Redirect{Path: "/" + kind + "/readable"}}
			r := gin.New()
			New(s).Register(r, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(method, "/"+kind+"/old?utm=a%2Fb&x=1", nil))
			if w.Code != 301 || w.Header().Get("Location") != "/"+kind+"/readable?utm=a%2Fb&x=1" {
				t.Fatal(w.Code, w.Header())
			}
			s.err = ErrNotFound
			w = httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(method, "/"+kind+"/old", nil))
			if w.Code != 404 || w.Header().Get("Location") != "" || w.Header().Get("X-Robots-Tag") != "noindex" {
				t.Fatal(w.Code, w.Header())
			}
		}
	}
}

func TestShareCardDimensionsAndExplicitUncroppedImage(t *testing.T) {
	s := &sharingSite{siteStub{sourceStub: sourceStub{page: Page{Kind: "product", Path: "/products/item", Title: "商品 A", Images: []string{"/invalid.svg"}}}}}
	r := gin.New()
	New(s).Register(r, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/products/item", nil))
	for _, want := range []string{`property="og:image:width" content="1200"`, `property="og:image:height" content="630"`, `property="og:image:alt" content="商品 A"`, `name="twitter:image:alt" content="商品 A"`} {
		if !strings.Contains(w.Body.String(), want) {
			t.Fatal("missing", want)
		}
	}
	s.page.Images = []string{"/uncropped-long.png"}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/products/item", nil))
	if strings.Contains(w.Body.String(), `property="og:image:width"`) {
		t.Fatal("invented dimensions")
	}
}
