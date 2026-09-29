package httpserver

import (
	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/app/httpserver/middleware"
	upstreamtransport "github.com/dujiao-next/internal/modules/upstreamapi/transport/http"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func registerUpstreamRoutes(
	apiV1 *gin.RouterGroup,
	c *container.Container,
	upstreamHandler *upstreamtransport.Handler,
	redisClient *redis.Client,
	upstreamAPIRule middleware.RateLimitRule,
	callbackRule middleware.RateLimitRule,
) {
	// 上游 API（本站作为 B 站点，暴露给下游 A 调用）
	upstreamAPI := apiV1.Group("/upstream")
	upstreamAPI.Use(middleware.RateLimitMiddleware(redisClient, upstreamAPIRule, middleware.KeyByUpstreamApiKey))
	upstreamAPI.Use(middleware.UpstreamAPIAuthMiddleware(c.ApiCredentialRepo))
	upstreamtransport.RegisterAuthenticatedRoutes(upstreamAPI, upstreamHandler)

	// 上游回调接收（本站作为 A 站点，接收 B 的回调）
	apiV1.POST("/upstream/callback", middleware.RateLimitMiddleware(redisClient, callbackRule, middleware.KeyByIP), upstreamHandler.HandleCallback)
}
