package sitemaphttp

import (
	"errors"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHEADMatchesGETWithoutBodyIncludingFailures(t *testing.T) {
	for _, failure := range []bool{false, true} {
		gen := &fakeGenerator{xml: "<urlset>中文</urlset>", robots: "User-agent: *\nDisallow:\n"}
		if failure {
			gen.err = errors.New("unavailable")
		}
		r := gin.New()
		RegisterRoutes(r, NewHandler(gen, fakeBrand{url: "https://example.com"}))
		for _, path := range []string{"/robots.txt", "/sitemap.xml"} {
			get, head := httptest.NewRecorder(), httptest.NewRecorder()
			r.ServeHTTP(get, httptest.NewRequest("GET", path, nil))
			r.ServeHTTP(head, httptest.NewRequest("HEAD", path, nil))
			if get.Code != head.Code || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
				t.Fatalf("HEAD mismatch %s: GET %d / HEAD %d headers=%v body=%q", path, get.Code, head.Code, head.Header(), head.Body.String())
			}
			for _, key := range []string{"Content-Type", "Cache-Control"} {
				if get.Header().Get(key) != head.Header().Get(key) {
					t.Fatal(path, key)
				}
			}
		}
	}
}
