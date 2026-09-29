package adminauthzhttp

import "github.com/gin-gonic/gin"

// RegisterAdminRoutes 注册后台权限（角色/策略/管理员账号）路由。
func RegisterAdminRoutes(authorized gin.IRoutes, handler *AdminHandler) {
	if authorized == nil || handler == nil {
		panic("admin authz routes: required dependency is nil")
	}
	authorized.GET("/authz/me", handler.GetAuthzMe)
	authorized.GET("/authz/roles", handler.ListAuthzRoles)
	authorized.GET("/authz/admins", handler.ListAuthzAdmins)
	authorized.POST("/authz/admins", handler.CreateAuthzAdmin)
	authorized.PUT("/authz/admins/:id", handler.UpdateAuthzAdmin)
	authorized.DELETE("/authz/admins/:id", handler.DeleteAuthzAdmin)
	authorized.POST("/authz/roles", handler.CreateAuthzRole)
	authorized.DELETE("/authz/roles/:role", handler.DeleteAuthzRole)
	authorized.GET("/authz/roles/:role/policies", handler.GetAuthzRolePolicies)
	authorized.POST("/authz/policies", handler.GrantAuthzPolicy)
	authorized.DELETE("/authz/policies", handler.RevokeAuthzPolicy)
	authorized.GET("/authz/admins/:id/roles", handler.GetAuthzAdminRoles)
	authorized.PUT("/authz/admins/:id/roles", handler.SetAuthzAdminRoles)
}
