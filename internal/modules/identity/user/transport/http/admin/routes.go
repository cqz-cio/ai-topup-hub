package adminuserhttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册后台用户管理路由。
func RegisterAdminRoutes(authorized gin.IRoutes, handler *AdminHandler) {
	if authorized == nil || handler == nil {
		panic("admin user routes: required dependency is nil")
	}
	authorized.GET("/users", handler.GetAdminUsers)
	authorized.PUT("/users/batch-status", handler.BatchUpdateUserStatus)
	authorized.DELETE("/users/:id/oauth/telegram", handler.UnbindAdminUserTelegram)
	authorized.DELETE("/users/:id/oauth/google", handler.UnbindAdminUserGoogle)
	authorized.GET("/users/:id", handler.GetAdminUser)
	authorized.PUT("/users/:id", handler.UpdateAdminUser)
	authorized.GET("/users/:id/coupon-usages", handler.GetAdminUserCouponUsages)
}
