package giftcardhttp

import "github.com/gin-gonic/gin"

func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	user.POST("/gift-cards/redeem", handler.Redeem)
}

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.POST("/gift-cards/generate", handler.Generate)
	admin.GET("/gift-cards", handler.List)
	admin.PUT("/gift-cards/:id", handler.Update)
	admin.DELETE("/gift-cards/:id", handler.Delete)
	admin.PATCH("/gift-cards/batch-status", handler.BatchUpdateStatus)
	admin.POST("/gift-cards/export", handler.Export)
}

func RegisterChannelRoutes(channel gin.IRoutes, handler *ChannelHandler) {
	channel.POST("/wallet/gift-card/redeem", handler.Redeem)
}
