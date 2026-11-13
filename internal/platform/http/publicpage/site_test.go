package publicpage

import (
	"context"
	"errors"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

type siteStub struct {
	sourceStub
	number int
	search string
}

func (s *siteStub) Home(context.Context) (Page, error)     { return s.page, s.err }
func (s *siteStub) About(context.Context) (Page, error)    { return s.page, s.err }
func (s *siteStub) Products(context.Context) (Page, error) { return s.page, s.err }
func (s *siteStub) Terms(context.Context) (Page, error)    { return s.page, s.err }
func (s *siteStub) Privacy(context.Context) (Page, error)  { return s.page, s.err }
func (s *siteStub) NoticeList(ctx context.Context, number int, search string) (Page, error) {
	return s.BlogList(ctx, number, search)
}
func (s *siteStub) BlogList(_ context.Context, number int, search string) (Page, error) {
	s.number, s.search = number, search
	return s.page, s.err
}

func TestSiteDocumentsServeSameCompleteHTMLForAllAgentsAndHEAD(t *testing.T) {
	for _, path := range []string{"/", "/about", "/blog", "/products", "/notice", "/terms", "/privacy"} {
		s := &siteStub{sourceStub: sourceStub{page: Page{Kind: "blog", Path: path, Title: "公开页面", Content: "<p>完整说明</p>", Entries: []Entry{{Title: "已发布标题", Path: "/blog/article", Summary: "<script>escaped summary</script>"}}, Pagination: []Link{{Title: "下一页", Path: "/blog?page=2"}}}}}
		r := gin.New()
		New(s).Register(r, nil)
		var original string
		for _, agent := range []string{"Mozilla/5.0", "Googlebot", "OAI-SearchBot", "GPTBot"} {
			request := httptest.NewRequest("GET", path, nil)
			request.Header.Set("User-Agent", agent)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, request)
			body := w.Body.String()
			if w.Code != 200 || !strings.Contains(body, "完整说明") || !strings.Contains(body, `href="/blog/article"`) || !strings.Contains(body, `href="/blog?page=2"`) || !strings.Contains(body, `"@type":"Organization"`) || strings.Contains(body, "<script>escaped") {
				t.Fatalf("bad document %s: %s", path, body)
			}
			if original != "" && original != body {
				t.Fatal("UA-dependent content")
			}
			original = body
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("HEAD", path, nil))
		if w.Code != 200 || w.Body.Len() != 0 || w.Header().Get("Content-Length") != strconv.Itoa(len(original)) {
			t.Fatal("HEAD parity failed")
		}
	}
}

func TestBlogQueryValidationFailureAndSearchIndexing(t *testing.T) {
	s := &siteStub{sourceStub: sourceStub{page: Page{Kind: "blog", Path: "/blog", Title: "博客", NoIndex: true}}}
	r := gin.New()
	New(s).Register(r, nil)
	for _, path := range []string{"/blog?page=0", "/blog?page=-1", "/blog?page=abc", "/blog?page=100001"} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
		if w.Code != 404 || w.Header().Get("X-Robots-Tag") != "noindex" {
			t.Fatal(path, w.Code)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/blog?page=2&search=Plus&utm_source=ignored", nil))
	if s.number != 2 || s.search != "Plus" || w.Code != 200 || w.Header().Get("X-Robots-Tag") != "noindex, follow" || strings.Contains(w.Body.String(), "utm_source") {
		t.Fatal("search handling")
	}
	s.err = errors.New("private database failure")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/about", nil))
	if w.Code != 503 || w.Header().Get("Retry-After") != "60" || strings.Contains(w.Body.String(), "database failure") {
		t.Fatal("temporary error exposed/indexable")
	}
}
