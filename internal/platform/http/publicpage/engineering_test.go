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
	return Site{Name: "π星卡社", URL: "https://www.qzweb.cloud", Icon: "/icon.png", DefaultImage: "/logo.webp"}, nil
}

func TestPublicSharingAndSanitizedPolicyHTML(t *testing.T) {
	s := &sharingSite{siteStub{sourceStub: sourceStub{page: Page{Kind: "legal", Path: "/terms", Title: "服务条款", Images: []string{"/cover.svg"}, Content: `<h2>公开条款</h2><script>privateScript()</script><img src=x onerror="privateHandler()">`}}}}
	r := gin.New()
	New(s).Register(r, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/terms", nil))
	body := w.Body.String()
	for _, want := range []string{`name="twitter:card" content="summary"`, `name="twitter:image" content="https://www.qzweb.cloud/logo.webp"`, `property="og:image" content="https://www.qzweb.cloud/logo.webp"`, `rel="icon" href="https://www.qzweb.cloud/icon.png"`, `"@type":"WebPage"`, `<h2>公开条款</h2>`} {
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
