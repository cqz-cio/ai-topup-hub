package promotionhttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册活动价管理端路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("promotion admin routes: required dependency is nil")
	}
	admin.POST("/promotions", handler.CreatePromotion)
	admin.GET("/promotions", handler.GetAdminPromotions)
	admin.PUT("/promotions/:id", handler.UpdatePromotion)
	admin.DELETE("/promotions/:id", handler.DeletePromotion)
}
