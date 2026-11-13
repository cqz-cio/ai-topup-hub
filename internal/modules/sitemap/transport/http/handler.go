package sitemaphttp

import (
	"context"
	"net/http"
	"strconv"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/platform/http/siteurl"

	"github.com/gin-gonic/gin"
)

// Generator 生成 sitemap.xml / robots.txt。
type Generator interface {
	Generate(ctx context.Context, baseURL string) (string, error)
	GenerateRobots(baseURL string) string
}

// SiteBrandReader 提供后台配置的站点品牌 URL。
type SiteBrandReader interface {
	GetSiteURL() (string, error)
}

// Handler 处理 SEO 静态资源请求。
type Handler struct {
	sitemap Generator
	brand   SiteBrandReader
}

func NewHandler(sitemap Generator, brand SiteBrandReader) *Handler {
	return &Handler{sitemap: sitemap, brand: brand}
}

// GetSitemap GET /sitemap.xml
func (h *Handler) GetSitemap(c *gin.Context) {
	if h == nil || h.sitemap == nil {
		writeDocument(c, 503, "text/plain; charset=utf-8", "sitemap service unavailable")
		return
	}

	baseURL := h.resolveBaseURL(c)
	xmlStr, err := h.sitemap.Generate(c.Request.Context(), baseURL)
	if err != nil {
		logger.Errorw("sitemap_generate_failed", "error", err)
		writeDocument(c, 500, "text/plain; charset=utf-8", "internal error")
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	writeDocument(c, 200, "application/xml; charset=utf-8", xmlStr)
}

// GetRobots GET /robots.txt
func (h *Handler) GetRobots(c *gin.Context) {
	baseURL := ""
	body := "User-agent: *\nDisallow:\n"
	if h != nil && h.sitemap != nil {
		baseURL = h.resolveBaseURL(c)
		body = h.sitemap.GenerateRobots(baseURL)
	}

	c.Header("Cache-Control", "public, max-age=3600")
	writeDocument(c, 200, "text/plain; charset=utf-8", body)
}

func writeDocument(c *gin.Context, status int, contentType, text string) {
	body := []byte(text)
	c.Header("Content-Length", strconv.Itoa(len(body)))
	if c.Request.Method == http.MethodHead {
		body = nil
	}
	c.Data(status, contentType, body)
}

// resolveBaseURL 与公开配置共用 URL 校验，避免将开发地址写入 sitemap。
func (h *Handler) resolveBaseURL(c *gin.Context) string {
	configured := ""
	if h != nil && h.brand != nil {
		if siteURL, err := h.brand.GetSiteURL(); err == nil {
			configured = siteURL
		}
	}
	return siteurl.Resolve(configured, c.Request)
}
