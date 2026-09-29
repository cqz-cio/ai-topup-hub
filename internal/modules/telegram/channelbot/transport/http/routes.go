package telegramhttp

import "github.com/gin-gonic/gin"

// RegisterChannelBotRoutes 注册渠道 Telegram Bot 配置与心跳路由。
func RegisterChannelBotRoutes(channel gin.IRoutes, handler *ChannelBotHandler) {
	channel.GET("/telegram/config", handler.GetBotConfig)
	channel.POST("/telegram/heartbeat", handler.ReportHeartbeat)
}
