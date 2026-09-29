package compliancehttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册后台合规声明路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/compliance/status", handler.GetComplianceStatus)
	admin.POST("/compliance/acknowledge", handler.AcknowledgeCompliance)
}
