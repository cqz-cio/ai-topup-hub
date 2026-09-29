package publicconfighttp

import "github.com/gin-gonic/gin"

// RegisterPublicRoutes 注册公开站点配置路由。
func RegisterPublicRoutes(public gin.IRoutes, handler *Handler) {
	if public == nil || handler == nil {
		panic("public config routes: required dependency is nil")
	}
	public.GET("/config", handler.GetConfig)
}
