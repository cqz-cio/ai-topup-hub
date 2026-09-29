package ginutil

import (
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// GetAdminID 从上下文读取管理员 ID。
func GetAdminID(c *gin.Context) (uint, bool) {
	return GetContextUintWithKeys(c, "admin_id", "error.admin_id_invalid", "error.admin_id_type_invalid")
}

// GetUserID 从上下文读取用户 ID。
func GetUserID(c *gin.Context) (uint, bool) {
	return GetContextUintWithKeys(c, "user_id", "error.user_id_invalid", "error.user_id_type_invalid")
}

// IsSuperAdmin 从上下文读取超级管理员标记（由 admin JWT 中间件注入）
func IsSuperAdmin(c *gin.Context) bool {
	v, ok := c.Get("admin_is_super")
	if !ok {
		return false
	}
	b, _ := v.(bool)
	return b
}

// GetContextUintWithKeys 从上下文读取 uint 值并统一处理错误响应。
func GetContextUintWithKeys(c *gin.Context, key, invalidKey, typeInvalidKey string) (uint, bool) {
	value, exists := c.Get(key)
	if !exists {
		RespondError(c, response.CodeUnauthorized, "error.unauthorized", nil)
		return 0, false
	}

	switch v := value.(type) {
	case uint:
		return v, true
	case int:
		if v < 0 {
			RespondError(c, response.CodeBadRequest, invalidKey, nil)
			return 0, false
		}
		return uint(v), true
	case float64:
		if v < 0 {
			RespondError(c, response.CodeBadRequest, invalidKey, nil)
			return 0, false
		}
		return uint(v), true
	default:
		RespondError(c, response.CodeInternal, typeInvalidKey, nil)
		return 0, false
	}
}
