package fulfillmenthttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册后台交付路由。
func RegisterAdminRoutes(authorized gin.IRoutes, handler *AdminHandler) {
	if authorized == nil || handler == nil {
		panic("fulfillment admin routes: required dependency is nil")
	}
	authorized.GET("/orders/:id/fulfillment/download", handler.AdminDownloadFulfillment)
	authorized.POST("/fulfillments", handler.AdminCreateFulfillment)
}
