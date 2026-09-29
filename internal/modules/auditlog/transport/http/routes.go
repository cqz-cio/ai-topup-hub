package auditloghttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/authz/audit-logs", handler.ListAuthzAuditLogs)
	admin.GET("/user-login-logs", handler.GetUserLoginLogs)
}

func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	user.GET("/me/login-logs", handler.GetMyLoginLogs)
}
