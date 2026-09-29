package siteconnectionhttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册后台站点对接连接路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/site-connections", handler.GetSiteConnections)
	admin.GET("/site-connections/:id", handler.GetSiteConnection)
	admin.POST("/site-connections", handler.CreateSiteConnection)
	admin.PUT("/site-connections/:id", handler.UpdateSiteConnection)
	admin.DELETE("/site-connections/:id", handler.DeleteSiteConnection)
	admin.POST("/site-connections/:id/ping", handler.PingSiteConnection)
	admin.PUT("/site-connections/:id/status", handler.UpdateSiteConnectionStatus)
	admin.POST("/site-connections/:id/reapply-markup", handler.ReapplyConnectionMarkup)
}
