package auditloghttp

import (
	"strings"

	"github.com/dujiao-next/internal/modules/auditlog/contract"
	"github.com/dujiao-next/internal/modules/auditlog/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type AuthzLogReader interface {
	ListForAdmin(filter contract.AuthzFilter) ([]domain.AuthzAuditLog, int64, error)
}

type UserLoginLogReader interface {
	ListForAdmin(filter contract.UserLoginFilter) ([]domain.UserLoginLog, int64, error)
}

type AdminHandler struct {
	authzLogs     AuthzLogReader
	userLoginLogs UserLoginLogReader
}

func NewAdminHandler(authzLogs AuthzLogReader, userLoginLogs UserLoginLogReader) *AdminHandler {
	return &AdminHandler{authzLogs: authzLogs, userLoginLogs: userLoginLogs}
}

// ListAuthzAuditLogs 获取权限审计日志列表
func (h *AdminHandler) ListAuthzAuditLogs(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	operatorAdminIDRaw := c.Query("operator_admin_id")
	targetAdminIDRaw := c.Query("target_admin_id")
	action := strings.TrimSpace(c.Query("action"))
	role := strings.TrimSpace(c.Query("role"))
	object := strings.TrimSpace(c.Query("object"))
	method := strings.TrimSpace(c.Query("method"))

	var operatorAdminID uint
	if operatorAdminIDRaw != "" {
		parsedOperatorAdminID, err := ginutil.ParseQueryUint(operatorAdminIDRaw, false)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		operatorAdminID = parsedOperatorAdminID
	}

	var targetAdminID uint
	if targetAdminIDRaw != "" {
		parsedTargetAdminID, err := ginutil.ParseQueryUint(targetAdminIDRaw, false)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		targetAdminID = parsedTargetAdminID
	}

	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	items, total, err := h.authzLogs.ListForAdmin(contract.AuthzFilter{
		Page:            page,
		PageSize:        pageSize,
		OperatorAdminID: operatorAdminID,
		TargetAdminID:   targetAdminID,
		Action:          action,
		Role:            role,
		Object:          object,
		Method:          method,
		CreatedFrom:     createdFrom,
		CreatedTo:       createdTo,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.config_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}
