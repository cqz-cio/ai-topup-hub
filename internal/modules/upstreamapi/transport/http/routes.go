package upstreamhttp

import "github.com/gin-gonic/gin"

func RegisterAuthenticatedRoutes(upstream gin.IRoutes, handler *Handler) {
	if upstream == nil || handler == nil {
		panic("upstream authenticated routes: required dependency is nil")
	}
	upstream.POST("/ping", handler.Ping)
	upstream.GET("/categories", handler.ListCategories)
	upstream.GET("/products", handler.ListProducts)
	upstream.GET("/products/:id", handler.GetProduct)
	upstream.POST("/orders", handler.CreateOrder)
	upstream.GET("/orders/:id", handler.GetOrder)
	upstream.POST("/orders/:id/cancel", handler.CancelOrder)
}
