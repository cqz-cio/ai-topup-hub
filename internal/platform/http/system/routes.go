package systemhttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册后台系统信息路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/system/version/check", handler.CheckSystemUpdate)
	admin.GET("/system/update/capability", handler.GetUpdateCapability)
	admin.GET("/system/update/status", handler.GetUpdateStatus)
	admin.POST("/system/update/start", handler.StartUpdate)
	admin.POST("/system/update/rollback", handler.RollbackUpdate)
	admin.POST("/system/restart", handler.RestartService)
}

// RegisterFeatureRoutes exposes only non-sensitive availability flags to signed-in
// administrators, including custom roles. Business endpoints retain their RBAC.
func RegisterFeatureRoutes(authenticated gin.IRoutes, handler *AdminHandler) {
	authenticated.GET("/system/features", handler.GetFeatures)
}
