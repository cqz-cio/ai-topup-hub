package rechargehttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(authorized gin.IRoutes, h *Handler) {
	authorized.GET("/orders/:id/auto-recharge", h.adminStatus)
	authorized.POST("/orders/:id/auto-recharge/advance", h.advance)
	authorized.GET("/orders/:id/auto-recharge/verification", h.verification)
	authorized.POST("/orders/:id/auto-recharge/finalize", h.finalize)
}
func RegisterUserRoutes(user gin.IRoutes, h *Handler) {
	user.GET("/orders/:order_no/auto-recharge", h.userStatus)
}

// Code and Session are POST body fields, never URL/query parameters.
func RegisterRedemptionRoutes(public gin.IRoutes, h *Handler) {
	public.POST("/lookup", h.lookupCode)
	public.POST("/account", h.checkAccount)
	public.POST("/confirm", h.confirmCode)
}
