package carthttp

import "github.com/gin-gonic/gin"

// RegisterUserRoutes 注册用户购物车路由。
func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	if user == nil || handler == nil {
		panic("cart user routes: required dependency is nil")
	}
	user.GET("/cart", handler.GetCart)
	user.POST("/cart/items", handler.UpsertCartItem)
	user.DELETE("/cart/items/:product_id", handler.DeleteCartItem)
}
