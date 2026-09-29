package dashboardhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/dashboard/overview", handler.GetOverview)
	admin.GET("/dashboard/trends", handler.GetTrends)
	admin.GET("/dashboard/rankings", handler.GetRankings)
	admin.GET("/dashboard/inventory-alerts", handler.GetInventoryAlerts)
}
