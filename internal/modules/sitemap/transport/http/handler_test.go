package sitemaphttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type fakeGenerator struct {
	xml    string
	robots string
	err    error
	base   string
}

func TestSEOEndpointsRejectDevelopmentSiteURL(t *testing.T) {
	for _, path := range []string{"/sitemap.xml", "/robots.txt"} {
		t.Run(path, func(t *testing.T) {
			gen := &fakeGenerator{xml: "<urlset/>", robots: "User-agent: *\n"}
			handler := NewHandler(gen, fakeBrand{url: "http://127.0.0.1:4170"})
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest("GET", "http://www.qzweb.cloud"+path, nil)
			c.Request.Header.Set("X-Forwarded-Proto", "https")
			c.Request.Header.Set("X-Forwarded-Host", "untrusted.example")
			if path == "/sitemap.xml" {
				handler.GetSitemap(c)
			} else {
				handler.GetRobots(c)
			}
			if w.Code != 200 || gen.base != "https://www.qzweb.cloud" {
				t.Fatalf("status=%d origin=%q", w.Code, gen.base)
			}
			if w.Header().Get("Cache-Control") == "" {
				t.Fatal("missing cache control")
			}
			if path == "/robots.txt" && !strings.HasPrefix(w.Header().Get("Content-Type"), "text/plain") {
				t.Fatal("robots must be plain text")
			}
		})
	}
}

func TestSitemapFailureDoesNotReturnSuccessfulDocument(t *testing.T) {
	handler := NewHandler(&fakeGenerator{err: errors.New("catalog unavailable")}, fakeBrand{})
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "https://www.qzweb.cloud/sitemap.xml", nil)
	handler.GetSitemap(c)
	if w.Code != 500 || w.Header().Get("Cache-Control") != "" {
		t.Fatalf("failure was advertised as cacheable: status=%d headers=%v", w.Code, w.Header())
	}
}

func (f *fakeGenerator) Generate(_ context.Context, baseURL string) (string, error) {
	f.base = baseURL
	return f.xml, f.err
}

func (f *fakeGenerator) GenerateRobots(baseURL string) string {
	f.base = baseURL
	return f.robots
}

type fakeBrand struct {
	url string
	err error
}

func (f fakeBrand) GetSiteURL() (string, error) {
	return f.url, f.err
}

func TestGetSitemapUsesConfiguredSiteURL(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gen := &fakeGenerator{xml: "<urlset/>"}
	handler := NewHandler(gen, fakeBrand{url: "https://shop.example/"})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	c.Request.Host = "ignored.example"

	handler.GetSitemap(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
	if gen.base != "https://shop.example" {
		t.Fatalf("base URL want https://shop.example got %q", gen.base)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "application/xml") {
		t.Fatalf("unexpected content type %q", w.Header().Get("Content-Type"))
	}
}

func TestGetSitemapFallsBackToRequestHost(t *testing.T) {
	gin.SetMode(gin.TestMode)
	gen := &fakeGenerator{xml: "<urlset/>"}
	handler := NewHandler(gen, fakeBrand{url: ""})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)
	req.Host = "store.example"
	req.Header.Set("X-Forwarded-Proto", "https")
	c.Request = req

	handler.GetSitemap(c)

	if gen.base != "https://store.example" {
		t.Fatalf("base URL want https://store.example got %q", gen.base)
	}
}

func TestGetSitemapUnavailableWithoutGenerator(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/sitemap.xml", nil)

	handler.GetSitemap(c)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status want 503 got %d", w.Code)
	}
}

func TestGetRobotsFallsBackWhenGeneratorMissing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := NewHandler(nil, nil)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/robots.txt", nil)

	handler.GetRobots(c)

	if w.Code != http.StatusOK {
		t.Fatalf("status want 200 got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "User-agent: *") {
		t.Fatalf("unexpected robots body %q", w.Body.String())
	}
}
