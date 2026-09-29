package web

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// 保留路径，不能与 admin path 冲突或互为前缀。
var reservedPaths = []string{"/api", "/uploads", "/health"}

// isReservedPath 报告请求路径是否落在后端保留前缀之下。
func isReservedPath(p string) bool {
	for _, r := range reservedPaths {
		if p == r || strings.HasPrefix(p, r+"/") {
			return true
		}
	}
	return false
}

// adminPathSegment 后台路径每一段允许的字符集。
//
// 之所以用白名单而不是「禁掉几个危险字符」：这个值会被直接拼进 Gin 的路由模式
// （prefix + "/*filepath"），而 Gin 把 ':' 和 '*' 当作动态参数与 catch-all。
//   - "/:tenant"     → 注册成 /:tenant/*filepath，会把任意一级用户站路径吞成后台 SPA
//   - "/*admin"      → 违反 Gin 的 catch-all 规则，注册时直接 panic
//   - "/admin/:id"   → 本该是一个字面秘密路径，结果变成动态参数路由
//
// 字符集取 RFC 3986 的 unreserved（字母、数字、'-'、'.'、'_'、'~'）再加 '@'：
// 这些字符在 URL 路径里无需编码、语义稳定，且都不是 Gin 的路由元字符。
// 之所以不缩到更窄的 [A-Za-z0-9_-]，是因为 '.' '~' '@' 在旧版本里是合法的，
// 收得过紧会让 /ops.admin、/admin~private、/console@company 这类存量配置升级后直接起不来。
//
// 同时挡掉了空白、控制字符、反斜杠和百分号编码带来的歧义。
var adminPathSegment = regexp.MustCompile(`^[A-Za-z0-9._~@-]+$`)

// ValidateAdminPath 校验 web.admin_path 配置项的合法性。
// 规则：
//   - 必须以 "/" 开头
//   - 不能为 "/"（会与 user SPA 兜底冲突）
//   - 不能以 "/" 结尾
//   - 每一段只能由字母、数字、'-'、'_' 组成（不允许 Gin 的 ':'、'*' 等路由元字符）
//   - 不能等于、不能是、也不能拥有 "/api"、"/uploads"、"/health" 任一作为前缀
func ValidateAdminPath(p string) error {
	if !strings.HasPrefix(p, "/") {
		return fmt.Errorf("web.admin_path must start with '/', got %q", p)
	}
	if p == "/" {
		return fmt.Errorf("web.admin_path cannot be '/' (conflicts with user SPA fallback)")
	}
	if strings.HasSuffix(p, "/") {
		return fmt.Errorf("web.admin_path cannot end with '/', got %q", p)
	}

	// 逐段校验。TrimPrefix 后按 "/" 切分，连续斜杠会切出空段并在这里被拒。
	for _, seg := range strings.Split(strings.TrimPrefix(p, "/"), "/") {
		if !adminPathSegment.MatchString(seg) {
			return fmt.Errorf(
				"web.admin_path segment %q is invalid in %q: each segment must be a literal path made of letters, digits, '-', '.', '_', '~' or '@'",
				seg, p)
		}
		// "." 与 ".." 单独成段会被路径规范化吃掉，导致实际挂载位置与配置不符
		if seg == "." || seg == ".." {
			return fmt.Errorf("web.admin_path cannot contain %q as a path segment, got %q", seg, p)
		}
	}

	for _, r := range reservedPaths {
		if p == r || strings.HasPrefix(p, r+"/") || strings.HasPrefix(r, p+"/") {
			return fmt.Errorf("web.admin_path %q conflicts with reserved path %q", p, r)
		}
	}
	return nil
}

// 占位符：admin/index.html 中的 __DJ_ADMIN_BASE__ 启动时被替换为实际 admin path
const adminBasePlaceholder = "__DJ_ADMIN_BASE__"

const (
	// SPA 入口必须每次回源校验。带内容 hash 的 chunk 文件名只记录在 index.html 里，
	// 入口一旦被长期缓存，升级后的浏览器会继续加载上一版 chunk，与新后端形成契约错配
	// （例如旧前端仍用查询参数传游客订单凭证，而新后端只认 Authorization 头）。
	// 不设该头时，Cloudflare 这类 CDN 会按自己的 Browser Cache TTL 补一个长缓存。
	indexCacheControl = "no-cache"

	// assets/ 下的产物由 Vite 保证带内容 hash，内容变化必然改名，可以安全长期强缓存。
	hashedAssetCacheControl = "public, max-age=31536000, immutable"
)

// serveIndex 返回 SPA 入口，并禁止浏览器与中间 CDN 长期缓存。
func serveIndex(c *gin.Context, body []byte) {
	c.Header("Cache-Control", indexCacheControl)
	c.Data(http.StatusOK, "text/html; charset=utf-8", body)
}

// serveAsset 返回真实静态文件。assets/ 下是内容寻址的构建产物，可长期强缓存；
// 其余（favicon.ico、robots.txt 等固定文件名）必须回源校验，否则替换后不会生效。
func serveAsset(c *gin.Context, fileServer http.Handler, fp string) {
	if strings.HasPrefix(fp, "assets/") {
		c.Header("Cache-Control", hashedAssetCacheControl)
	} else {
		c.Header("Cache-Control", indexCacheControl)
	}
	fileServer.ServeHTTP(c.Writer, c.Request)
}

// RegisterAdmin 在 prefix 前缀下挂载 admin SPA。
//
// 启动时从 fsys 读取 index.html，把 __DJ_ADMIN_BASE__ 一次性替换为 strings.Trim(prefix, "/")，
// 然后缓存。后续请求走该缓存，不做实时替换。
//
// 路由匹配规则：
//   - GET prefix + "/*filepath"
//   - filepath 为空 / "/" / "/index.html" → 返回缓存的 index.html
//   - filepath 在 fsys 中有真实文件 → 用 http.FileServer 服务该文件
//   - 否则（SPA history 深层路由）→ 返回缓存的 index.html
func RegisterAdmin(r *gin.Engine, prefix string, fsys fs.FS) error {
	if r == nil {
		return errors.New("nil gin engine")
	}
	if fsys == nil {
		return errors.New("nil admin filesystem")
	}

	raw, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return fmt.Errorf("read admin/index.html: %w", err)
	}
	// 注意：base href 必须是绝对路径（带前导 /），否则浏览器会基于当前 URL
	// 解析出 prefix/prefix/ 的双重前缀（例如访问 /admin/orders 时资源 URL 会
	// 错误解析为 /admin/orders/admin/assets/...），导致深层路由刷新后 404。
	cached := bytes.ReplaceAll(raw, []byte(adminBasePlaceholder), []byte("/"+strings.Trim(prefix, "/")))

	fileServer := http.StripPrefix(prefix, http.FileServer(http.FS(fsys)))

	r.GET(prefix+"/*filepath", func(c *gin.Context) {
		fp := strings.TrimPrefix(c.Param("filepath"), "/")
		if fp == "" || fp == "index.html" {
			serveIndex(c, cached)
			return
		}
		if hasFile(fsys, fp) {
			serveAsset(c, fileServer, fp)
			return
		}
		serveIndex(c, cached)
	})
	return nil
}

// RegisterUser 把 user SPA 挂载到 NoRoute 兜底位置。
//
// 由于 NoRoute 在所有显式路由（API、uploads、health、admin）之后才匹配，
// 这里不需要做路径前缀剥离；命中真实文件返回文件，否则 fallback 到 index.html。
func RegisterUser(r *gin.Engine, fsys fs.FS) error {
	if r == nil {
		return errors.New("nil gin engine")
	}
	if fsys == nil {
		return errors.New("nil user filesystem")
	}

	indexCached, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return fmt.Errorf("read user/index.html: %w", err)
	}

	fileServer := http.FileServer(http.FS(fsys))

	r.NoRoute(func(c *gin.Context) {
		// 后端保留前缀下的未命中请求必须保持 404，不能被 SPA 兜底成 200 HTML：
		// 否则调用了不存在的接口的客户端会拿到 index.html 当响应体去解析 JSON。
		// 这也让单二进制部署与「nginx 反代 + 独立前端」的分离部署行为保持一致。
		if isReservedPath(c.Request.URL.Path) {
			c.String(http.StatusNotFound, "404 page not found")
			return
		}

		fp := strings.TrimPrefix(c.Request.URL.Path, "/")
		if fp == "" || fp == "index.html" {
			serveIndex(c, indexCached)
			return
		}
		if hasFile(fsys, fp) {
			serveAsset(c, fileServer, fp)
			return
		}
		serveIndex(c, indexCached)
	})
	return nil
}

func hasFile(fsys fs.FS, name string) bool {
	name = path.Clean(name)
	if name == "." || name == "/" {
		return false
	}
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return !stat.IsDir()
}
