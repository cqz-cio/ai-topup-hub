package reconciliationhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("reconciliation admin routes: required dependency is nil")
	}
	admin.POST("/reconciliation/run", handler.Run)
	admin.GET("/reconciliation/jobs", handler.ListJobs)
	admin.GET("/reconciliation/jobs/:id", handler.GetJob)
	admin.PUT("/reconciliation/items/:id/resolve", handler.ResolveItem)
}
