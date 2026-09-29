package middleware

import (
	complianceapp "github.com/dujiao-next/internal/modules/compliance/application"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// PaymentComplianceRequired 拦截支付/财务路由：未确认合规声明则阻断。
// - 已确认 → 放行
// - 未确认 + 超管 → 业务码 403 + msg "compliance_required"（前端据此弹窗）
// - 未确认 + 非超管 → 业务码 403 + msg "compliance_required_by_super_admin"（前端跳转提示页）
func PaymentComplianceRequired(cs *complianceapp.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		if cs != nil && cs.IsAcknowledged() {
			c.Next()
			return
		}
		if ginutil.IsSuperAdmin(c) {
			response.Error(c, response.CodeForbidden, "compliance_required")
		} else {
			response.Error(c, response.CodeForbidden, "compliance_required_by_super_admin")
		}
		c.Abort()
	}
}
