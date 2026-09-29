package broadcasthttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/telegram-bot/broadcasts", handler.List)
	admin.GET("/telegram-bot/broadcasts/:id", handler.Get)
	admin.POST("/telegram-bot/broadcasts", handler.Create)
	admin.GET("/telegram-bot/users", handler.ListUsers)
}
