package uploadhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.POST("/upload", handler.UploadFile)
}
