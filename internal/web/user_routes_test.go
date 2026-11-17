package web

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestUserRouteAllowlistPreservesBusinessPagesAndRejectsUnknownURLs(t *testing.T) {
	r := gin.New()
	if err := RegisterUser(r, newUserFS()); err != nil {
		t.Fatal(err)
	}
	paths := map[string]int{
		"/cart": 200, "/checkout": 200, "/pay": 200, "/redeem": 200, "/me": 200,
		"/me/profile": 200, "/me/gift-cards": 200, "/reseller": 200, "/reseller/orders/123": 200,
		"/orders/123": 200, "/recharge-orders/123": 200, "/guest/orders": 200, "/guest/orders/123": 200,
		"/auth/login": 200, "/auth/register": 200, "/auth/forgot": 200, "/auth/google/callback": 200, "/auth/telegram/callback": 200,
		"/missing-page": 404, "/cart/extra": 404, "/auth/unknown": 404, "/me/unknown": 404,
		"/assets/missing.js": 404, "/products/a/b": 404, "/apiary": 404,
	}
	for path, status := range paths {
		get, head := httptest.NewRecorder(), httptest.NewRecorder()
		r.ServeHTTP(get, httptest.NewRequest("GET", path, nil))
		r.ServeHTTP(head, httptest.NewRequest("HEAD", path, nil))
		if get.Code != status || head.Code != status || head.Body.Len() != 0 || head.Header().Get("Content-Length") != strconv.Itoa(get.Body.Len()) {
			t.Fatal(path, get.Code, head.Code, head.Header())
		}
		if get.Header().Get("X-Robots-Tag") == "" {
			t.Fatal("indexable private/error page", path)
		}
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("POST", "/cart", nil))
	if w.Code != 404 {
		t.Fatal("unknown mutation must not serve SPA", w.Code)
	}
}

func TestPublicCartGETAndHEADRemainNonIndexableUncachedShells(t *testing.T) {
	r := gin.New()
	if err := RegisterUser(r, newUserFS()); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/cart", "/cart/", "/cart?utm=test", "/cart/?utm=test", "/cart?user_id=1&next=/checkout"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			t.Run(method+path, func(t *testing.T) {
				w := httptest.NewRecorder()
				// No Cookie/Authorization: public shell contains no per-user cart data.
				r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
				if w.Code != http.StatusOK || w.Header().Get("X-Robots-Tag") != "noindex, nofollow" || w.Header().Get("Cache-Control") != "no-cache" {
					t.Fatalf("cart policy changed: status=%d headers=%v", w.Code, w.Header())
				}
				if w.Header().Get("Set-Cookie") != "" {
					t.Fatal("anonymous shell unexpectedly establishes a session")
				}
				wantBody := "<html>user-spa</html>"
				if w.Header().Get("Content-Length") != strconv.Itoa(len(wantBody)) {
					t.Fatal("GET/HEAD content length mismatch")
				}
				if method == http.MethodHead {
					wantBody = ""
				}
				if w.Body.String() != wantBody {
					t.Fatal("cart should only serve the static shell")
				}
			})
		}
	}
}
