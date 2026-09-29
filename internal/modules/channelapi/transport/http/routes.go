package channelhttp

import "github.com/gin-gonic/gin"

func RegisterRoutes(channel gin.IRoutes, handler *Handler) {
	if channel == nil || handler == nil {
		panic("channel routes: required dependency is nil")
	}
	channel.POST("/identities/telegram/resolve", handler.ResolveTelegramIdentity)
	channel.POST("/identities/telegram/provision", handler.ProvisionTelegramIdentity)
	channel.POST("/identities/telegram/bind", handler.BindTelegramIdentity)
	channel.GET("/me", handler.GetCurrentIdentity)
	channel.GET("/catalog/categories", handler.GetCategories)
	channel.GET("/catalog/products", handler.GetProducts)
	channel.GET("/catalog/products/:id", handler.GetProductDetail)
	channel.POST("/orders/preview", handler.PreviewOrder)
	channel.POST("/orders", handler.CreateOrder)
	channel.GET("/orders", handler.ListOrders)
	channel.GET("/orders/by-order-no/:order_no", handler.GetOrderByOrderNo)
	channel.GET("/orders/:id", handler.GetOrderStatus)
	channel.POST("/orders/:id/cancel", handler.CancelOrder)
	channel.GET("/payment-channels", handler.GetPaymentChannels)
	channel.GET("/payment-methods", handler.GetPaymentChannels)
	channel.GET("/payments/latest", handler.GetLatestPayment)
	channel.GET("/payments/:id", handler.GetPaymentDetail)
	channel.POST("/payments", handler.CreatePayment)
}
