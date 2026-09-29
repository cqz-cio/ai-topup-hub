package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gin-gonic/gin"
)

func TestValidateAdminPath(t *testing.T) {
	cases := []struct {
		name      string
		input     string
		wantErr   bool
		errSubstr string
	}{
		{"valid default", "/admin", false, ""},
		{"valid custom", "/dj-mgmt-7x9k2", false, ""},
		{"valid with dashes", "/console-private", false, ""},

		{"empty", "", true, "must start with"},
		{"missing leading slash", "admin", true, "must start with"},
		{"only slash", "/", true, "cannot be"},
		{"trailing slash", "/admin/", true, "cannot end with"},
		{"trailing slash deep", "/foo/bar/", true, "cannot end with"},

		{"reserved api", "/api", true, "reserved"},
		{"reserved api prefix", "/api/v1", true, "reserved"},
		{"reserved uploads", "/uploads", true, "reserved"},
		{"reserved uploads prefix", "/uploads/x", true, "reserved"},
		{"reserved health", "/health", true, "reserved"},

		{"valid nested", "/ops/console", false, ""},
		{"valid underscores", "/dj_mgmt_7x9k2", false, ""},
		// 这几种在收紧校验之前就是合法的，不能因为一次安全加固把存量配置弄成起不来
		{"valid dot", "/ops.admin", false, ""},
		{"valid tilde", "/admin~private", false, ""},
		{"valid at sign", "/console@company", false, ""},

		// Gin 路由元字符：拼进 prefix + "/*filepath" 后会变成动态参数或 catch-all，
		// 前者把用户站路径吞成后台 SPA，后者直接让路由注册 panic
		{"gin param", "/:tenant", true, "invalid"},
		{"gin param nested", "/admin/:id", true, "invalid"},
		{"gin catch-all", "/*admin", true, "invalid"},
		{"gin catch-all nested", "/admin/*rest", true, "invalid"},

		{"double slash", "/admin//panel", true, "invalid"},
		{"space", "/my admin", true, "invalid"},
		{"tab", "/my\tadmin", true, "invalid"},
		{"backslash", "/admin\\panel", true, "invalid"},
		{"percent encoded", "/adm%2Fin", true, "invalid"},
		// "." / ".." 由字符集允许，但会被路径规范化吃掉，需要单独拒绝
		{"dot segment", "/../etc", true, "cannot contain"},
		{"single dot segment", "/./admin", true, "cannot contain"},
		{"trailing dot segment", "/admin/..", true, "cannot contain"},
		{"question mark", "/admin?x=1", true, "invalid"},
		{"hash", "/admin#top", true, "invalid"},
		{"control char", "/adm\x00in", true, "invalid"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateAdminPath(c.input)
			if c.wantErr {
				if err == nil {
					t.Fatalf("want error, got nil")
				}
				if c.errSubstr != "" && !strings.Contains(err.Error(), c.errSubstr) {
					t.Fatalf("error %q does not contain %q", err.Error(), c.errSubstr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func newAdminFS(indexHTML string) fstest.MapFS {
	return fstest.MapFS{
		"index.html":     &fstest.MapFile{Data: []byte(indexHTML)},
		"assets/app.js":  &fstest.MapFile{Data: []byte("console.log('app');")},
		"assets/app.css": &fstest.MapFile{Data: []byte("body{}")},
		"favicon.ico":    &fstest.MapFile{Data: []byte("\x00\x00")},
	}
}

func TestRegisterAdmin_PlaceholderReplacement(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	html := `<!doctype html><html><head><base href="__DJ_ADMIN_BASE__/"><title>x</title></head><body></body></html>`
	if err := RegisterAdmin(r, "/dj-mgmt-7x9k2", newAdminFS(html)); err != nil {
		t.Fatalf("RegisterAdmin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/dj-mgmt-7x9k2/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), `<base href="/dj-mgmt-7x9k2/">`) {
		t.Fatalf("placeholder not replaced or missing leading slash; body = %s", body)
	}
	if strings.Contains(string(body), "__DJ_ADMIN_BASE__") {
		t.Fatalf("placeholder still present in body")
	}
}

// TestRegisterAdmin_BaseHrefIsAbsolute 防止 base href 退化成相对路径而触发
// 浏览器双前缀解析 bug（详见 handler.go:67 注释）。
func TestRegisterAdmin_BaseHrefIsAbsolute(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name   string
		prefix string
		want   string
	}{
		{"default admin", "/admin", `<base href="/admin/">`},
		{"deep custom", "/dj-mgmt-7x9k2", `<base href="/dj-mgmt-7x9k2/">`},
		{"with multiple segments", "/console-private", `<base href="/console-private/">`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := gin.New()
			html := `<head><base href="__DJ_ADMIN_BASE__/"></head>`
			if err := RegisterAdmin(r, c.prefix, newAdminFS(html)); err != nil {
				t.Fatalf("RegisterAdmin: %v", err)
			}
			req := httptest.NewRequest(http.MethodGet, c.prefix+"/products/123/edit", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			body := w.Body.String()
			if !strings.Contains(body, c.want) {
				t.Fatalf("history-fallback body for %s missing %q\nbody = %s", c.prefix, c.want, body)
			}
		})
	}
}

func TestRegisterAdmin_ServesRealAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	if err := RegisterAdmin(r, "/admin", newAdminFS("<html></html>")); err != nil {
		t.Fatalf("RegisterAdmin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/assets/app.js", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "console.log('app');" {
		t.Fatalf("body = %q", body)
	}
}

func TestRegisterAdmin_HistoryFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	html := `<!doctype html><html><head><base href="__DJ_ADMIN_BASE__/"></head><body>spa</body></html>`
	if err := RegisterAdmin(r, "/admin", newAdminFS(html)); err != nil {
		t.Fatalf("RegisterAdmin: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/products/123/edit", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "spa") {
		t.Fatalf("not index.html: %s", body)
	}
	ctype := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ctype, "text/html") {
		t.Fatalf("content-type = %q, want text/html*", ctype)
	}
}

func TestRegisterAdmin_NilFS(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := RegisterAdmin(r, "/admin", nil); err == nil {
		t.Fatal("want error for nil fs.FS, got nil")
	}
}

func TestRegisterAdmin_MissingIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	fsys := fstest.MapFS{"assets/x.js": &fstest.MapFile{Data: []byte("x")}}
	if err := RegisterAdmin(r, "/admin", fsys); err == nil {
		t.Fatal("want error when index.html missing")
	}
}

func newUserFS() fstest.MapFS {
	return fstest.MapFS{
		"index.html":  &fstest.MapFile{Data: []byte("<html>user-spa</html>")},
		"assets/u.js": &fstest.MapFile{Data: []byte("console.log('u');")},
		"robots.txt":  &fstest.MapFile{Data: []byte("User-agent: *\n")},
	}
}

func TestRegisterUser_ServesIndex(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := RegisterUser(r, newUserFS()); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "user-spa") {
		t.Fatalf("body = %s", body)
	}
}

func TestRegisterUser_ServesAsset(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := RegisterUser(r, newUserFS()); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/assets/u.js", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if string(body) != "console.log('u');" {
		t.Fatalf("body = %q", body)
	}
}

func TestRegisterUser_HistoryFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := RegisterUser(r, newUserFS()); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/products/some-slug", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	body, _ := io.ReadAll(w.Body)
	if !strings.Contains(string(body), "user-spa") {
		t.Fatalf("did not fallback to index.html: %s", body)
	}
}

// TestRegisterUser_ReservedPathsStay404 保证未命中的后端接口不会被 SPA 兜底成
// 200 HTML —— 否则 API 客户端会拿 index.html 去解析 JSON。
func TestRegisterUser_ReservedPathsStay404(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := RegisterUser(r, newUserFS()); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	for _, p := range []string{
		"/api",
		"/api/v1/nonexistent",
		"/uploads/missing.png",
		"/health/subpath",
	} {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", w.Code)
			}
			if strings.Contains(w.Body.String(), "user-spa") {
				t.Fatalf("reserved path fell back to SPA index: %s", w.Body.String())
			}
		})
	}
}

// TestRegisterUser_NonReservedLookalikeStillServesSPA 确认保留前缀判断是按路径段
// 匹配的，不会误伤 /apiary 这类只是恰好同前缀的前端路由。
func TestRegisterUser_NonReservedLookalikeStillServesSPA(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if err := RegisterUser(r, newUserFS()); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	for _, p := range []string{"/apiary", "/uploads-guide", "/healthy"} {
		t.Run(p, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, p, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200", w.Code)
			}
			if !strings.Contains(w.Body.String(), "user-spa") {
				t.Fatalf("expected SPA fallback, got: %s", w.Body.String())
			}
		})
	}
}

func TestRegisterUser_DoesNotShadowExistingRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/ping", func(c *gin.Context) {
		c.String(http.StatusOK, "pong")
	})
	if err := RegisterUser(r, newUserFS()); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/ping", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if w.Body.String() != "pong" {
		t.Fatalf("body = %s, want pong", w.Body.String())
	}
}

// SPA 入口被长期缓存会让升级后的浏览器继续加载上一版 chunk，与新后端形成契约错配，
// 因此入口必须回源校验；只有 assets/ 下带内容 hash 的产物才允许强缓存。
func TestCacheControlHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name string
		path string
		want string
	}{
		{"user index", "/", indexCacheControl},
		{"user history fallback", "/products/1", indexCacheControl},
		{"user hashed asset", "/assets/u.js", hashedAssetCacheControl},
		{"user unhashed file", "/robots.txt", indexCacheControl},
		{"admin index", "/admin/", indexCacheControl},
		{"admin history fallback", "/admin/orders/1", indexCacheControl},
		{"admin hashed asset", "/admin/assets/app.js", hashedAssetCacheControl},
	}

	r := gin.New()
	if err := RegisterAdmin(r, "/admin", newAdminFS("<html>admin-spa</html>")); err != nil {
		t.Fatalf("RegisterAdmin: %v", err)
	}
	if err := RegisterUser(r, newUserFS()); err != nil {
		t.Fatalf("RegisterUser: %v", err)
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d", w.Code)
			}
			if got := w.Header().Get("Cache-Control"); got != tc.want {
				t.Fatalf("Cache-Control = %q, want %q", got, tc.want)
			}
		})
	}
}
