package couponhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.POST("/coupons", handler.CreateCoupon)
	admin.GET("/coupons", handler.GetAdminCoupons)
	admin.PUT("/coupons/:id", handler.UpdateCoupon)
	admin.DELETE("/coupons/:id", handler.DeleteCoupon)
}
