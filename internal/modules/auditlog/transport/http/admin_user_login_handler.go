package auditloghttp

import (
	"strings"

	"github.com/dujiao-next/internal/modules/auditlog/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// GetUserLoginLogs 获取用户登录日志列表
func (h *AdminHandler) GetUserLoginLogs(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	userIDRaw := c.Query("user_id")
	email := strings.TrimSpace(c.Query("email"))
	status := strings.TrimSpace(c.Query("status"))
	failReason := strings.TrimSpace(c.Query("fail_reason"))
	clientIP := strings.TrimSpace(c.Query("client_ip"))

	var userID uint
	if userIDRaw != "" {
		parsedUserID, err := ginutil.ParseQueryUint(userIDRaw, false)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		userID = parsedUserID
	}

	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	logs, total, err := h.userLoginLogs.ListForAdmin(contract.UserLoginFilter{
		Page:        page,
		PageSize:    pageSize,
		UserID:      userID,
		Email:       email,
		Status:      status,
		FailReason:  failReason,
		ClientIP:    clientIP,
		CreatedFrom: createdFrom,
		CreatedTo:   createdTo,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.user_login_log_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, logs, pagination)
}
