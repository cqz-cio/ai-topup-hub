package sitemaphttp

import (
	"context"
	"strings"

	"github.com/dujiao-next/internal/logger"

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
		c.String(503, "sitemap service unavailable")
		return
	}

	baseURL := h.resolveBaseURL(c)
	xmlStr, err := h.sitemap.Generate(c.Request.Context(), baseURL)
	if err != nil {
		logger.Errorw("sitemap_generate_failed", "error", err)
		c.String(500, "internal error")
		return
	}

	c.Header("Cache-Control", "public, max-age=300")
	c.Data(200, "application/xml; charset=utf-8", []byte(xmlStr))
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
	c.Data(200, "text/plain; charset=utf-8", []byte(body))
}

// resolveBaseURL 站点 URL 优先取后台 brand.site_url，否则回落到当前请求 Host。
func (h *Handler) resolveBaseURL(c *gin.Context) string {
	if h != nil && h.brand != nil {
		if siteURL, err := h.brand.GetSiteURL(); err == nil {
			if u := strings.TrimRight(strings.TrimSpace(siteURL), "/"); u != "" {
				return u
			}
		}
	}

	scheme := "https"
	if c.Request.TLS == nil && c.GetHeader("X-Forwarded-Proto") == "" {
		scheme = "http"
	}
	if proto := c.GetHeader("X-Forwarded-Proto"); proto != "" {
		scheme = proto
	}
	host := c.Request.Host
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}
	return scheme + "://" + host
}
