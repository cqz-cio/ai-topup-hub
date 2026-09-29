package cardsecrethttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("card secret admin routes: required dependency is nil")
	}
	admin.POST("/card-secrets/batch", handler.CreateCardSecretBatch)
	admin.POST("/card-secrets/import", handler.ImportCardSecretCSV)
	admin.GET("/card-secrets", handler.GetCardSecrets)
	admin.PUT("/card-secrets/:id", handler.UpdateCardSecret)
	admin.PATCH("/card-secrets/batch-status", handler.BatchUpdateCardSecretStatus)
	admin.POST("/card-secrets/batch-delete", handler.BatchDeleteCardSecrets)
	admin.POST("/card-secrets/export", handler.ExportCardSecrets)
	admin.POST("/card-secrets/export-available", handler.ExportAvailableCardSecrets)
	admin.GET("/card-secrets/stats", handler.GetCardSecretStats)
	admin.GET("/card-secrets/batches", handler.GetCardSecretBatches)
	admin.GET("/card-secrets/template", handler.GetCardSecretTemplate)
}
