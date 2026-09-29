package paymentcallbackhttp

import "github.com/gin-gonic/gin"

// RegisterRoutes registers the shared synchronous payment callback endpoint.
func RegisterRoutes(apiV1 *gin.RouterGroup, handler *Handler) {
	apiV1.POST("/payments/callback", handler.PaymentCallback)
	apiV1.GET("/payments/callback", handler.PaymentCallback)
}
