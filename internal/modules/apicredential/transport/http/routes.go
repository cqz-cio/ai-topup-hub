package apicredentialhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	admin.GET("/api-credentials", handler.GetApiCredentials)
	admin.GET("/api-credentials/:id", handler.GetApiCredential)
	admin.POST("/api-credentials/:id/approve", handler.ApproveApiCredential)
	admin.POST("/api-credentials/:id/reject", handler.RejectApiCredential)
	admin.PUT("/api-credentials/:id/status", handler.UpdateApiCredentialStatus)
	admin.DELETE("/api-credentials/:id", handler.DeleteApiCredential)
}

func RegisterUserRoutes(user gin.IRoutes, handler *UserHandler) {
	user.GET("/api-credential", handler.GetMyApiCredential)
	user.POST("/api-credential/apply", handler.ApplyApiCredential)
	user.POST("/api-credential/regenerate", handler.RegenerateMyApiCredential)
	user.PUT("/api-credential/status", handler.UpdateMyApiCredentialStatus)
}
