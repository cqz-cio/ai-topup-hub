package notificationhttp

import "github.com/gin-gonic/gin"

func RegisterAdminRoutes(admin gin.IRoutes, handler *AdminHandler) {
	if admin == nil || handler == nil {
		panic("notification admin routes: required dependency is nil")
	}
	admin.GET("/settings/notification-center", handler.GetNotificationCenterSettings)
	admin.PUT("/settings/notification-center", handler.UpdateNotificationCenterSettings)
	admin.GET("/settings/notification-center/logs", handler.ListNotificationLogs)
	admin.POST("/settings/notification-center/test", handler.TestNotificationCenterSettings)
	admin.GET("/settings/notifications", handler.GetNotificationCenterSettings)
	admin.PUT("/settings/notifications", handler.UpdateNotificationCenterSettings)
	admin.GET("/settings/notifications/logs", handler.ListNotificationLogs)
	admin.POST("/settings/notifications/test", handler.TestNotificationCenterSettings)
}
