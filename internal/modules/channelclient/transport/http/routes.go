package channelclienthttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册后台渠道客户端路由。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/channel-clients", handler.ListChannelClients)
	admin.POST("/channel-clients", handler.CreateChannelClient)
	admin.GET("/channel-clients/:id", handler.GetChannelClient)
	admin.PUT("/channel-clients/:id", handler.UpdateChannelClient)
	admin.PUT("/channel-clients/:id/status", handler.UpdateChannelClientStatus)
	admin.POST("/channel-clients/:id/reset-secret", handler.ResetChannelClientSecret)
	admin.DELETE("/channel-clients/:id", handler.DeleteChannelClient)
}
