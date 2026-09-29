package categoryhttp

import "github.com/gin-gonic/gin"

func RegisterPublicRoutes(public gin.IRoutes, handler *PublicHandler) {
	if public == nil || handler == nil {
		panic("category public routes: required dependency is nil")
	}
	public.GET("/categories", handler.List)
}

// RegisterAdminRoutes 注册商品分类后台端点。
func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminCategoryHandler) {
	if admin == nil || handler == nil {
		panic("category admin routes: required dependency is nil")
	}
	admin.GET("/categories", handler.GetAdminCategories)
	admin.POST("/categories", handler.CreateCategory)
	admin.PUT("/categories/:id", handler.UpdateCategory)
	admin.PATCH("/categories/:id/active", handler.PatchCategoryActive)
	admin.DELETE("/categories/:id", handler.DeleteCategory)
}
