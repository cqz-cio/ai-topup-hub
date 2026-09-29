package adproxyhttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册后台广告代理路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/ads/render/:slotCode", handler.GetAdRender)
	admin.POST("/ads/impression", handler.PostAdImpression)
}
