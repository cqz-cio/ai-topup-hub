package sitemaphttp

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册站点根路径下的 SEO 资源路由。
func RegisterRoutes(engine gin.IRoutes, handler *Handler) {
	engine.GET("/sitemap.xml", handler.GetSitemap)
	engine.GET("/robots.txt", handler.GetRobots)
}
