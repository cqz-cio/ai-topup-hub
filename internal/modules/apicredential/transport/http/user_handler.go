package apicredentialhttp

import (
	"errors"

	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"

	"github.com/dujiao-next/internal/constants"
	apicredentialcontract "github.com/dujiao-next/internal/modules/apicredential/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// UserService 是用户中心 API 凭证接口实际使用的应用能力。
type UserService interface {
	GetByUserID(userID uint) (*apicredentialdomain.ApiCredential, error)
	Apply(userID uint) (*apicredentialdomain.ApiCredential, error)
	RegenerateByUserID(userID uint) (string, error)
	SetActiveByUserID(userID uint, active bool) error
}

type UserHandler struct {
	service UserService
}

func NewUserHandler(service UserService) *UserHandler {
	return &UserHandler{service: service}
}

// GetMyApiCredential 查看自己的 API 凭证状态
func (h *UserHandler) GetMyApiCredential(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	cred, err := h.service.GetByUserID(userID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.api_credential_fetch_failed", err)
		return
	}

	if cred == nil {
		response.Success(c, gin.H{"status": "none"})
		return
	}

	result := gin.H{
		"id":         cred.ID,
		"status":     cred.Status,
		"is_active":  cred.IsActive,
		"created_at": cred.CreatedAt,
	}

	if cred.Status == constants.ApiCredentialStatusRejected {
		result["reject_reason"] = cred.RejectReason
	}

	if cred.Status == constants.ApiCredentialStatusApproved {
		result["api_key"] = cred.ApiKey
		result["approved_at"] = cred.ApprovedAt
		result["last_used_at"] = cred.LastUsedAt
		// Secret 末 4 位（掩码展示）
		if len(cred.ApiSecret) >= 4 {
			result["api_secret_tail"] = cred.ApiSecret[len(cred.ApiSecret)-4:]
		}
	}

	response.Success(c, result)
}

// ApplyApiCredential 申请 API 对接权限
func (h *UserHandler) ApplyApiCredential(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	cred, err := h.service.Apply(userID)
	if err != nil {
		switch {
		case errors.Is(err, apicredentialcontract.ErrExists):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, "API credential already exists", nil)
		case errors.Is(err, apicredentialcontract.ErrPendingExist):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, "Application is pending review", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.api_credential_apply_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"id":     cred.ID,
		"status": cred.Status,
	})
}

// RegenerateMyApiCredential 重新生成 Secret
func (h *UserHandler) RegenerateMyApiCredential(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	newSecret, err := h.service.RegenerateByUserID(userID)
	if err != nil {
		switch {
		case errors.Is(err, apicredentialcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.api_credential_not_found", nil)
		case errors.Is(err, apicredentialcontract.ErrNotApproved):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, "API credential is not approved", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.api_credential_regenerate_failed", err)
		}
		return
	}

	response.Success(c, gin.H{
		"api_secret": newSecret,
	})
}

// UpdateMyApiCredentialStatusRequest 更新凭证状态请求
type UpdateMyApiCredentialStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// UpdateMyApiCredentialStatus 启用/禁用自己的凭证
func (h *UserHandler) UpdateMyApiCredentialStatus(c *gin.Context) {
	userID, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	var req UpdateMyApiCredentialStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.service.SetActiveByUserID(userID, req.IsActive); err != nil {
		switch {
		case errors.Is(err, apicredentialcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.api_credential_not_found", nil)
		case errors.Is(err, apicredentialcontract.ErrNotApproved):
			ginutil.RespondErrorWithMsg(c, response.CodeBadRequest, "API credential is not approved", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.api_credential_update_failed", err)
		}
		return
	}

	response.Success(c, gin.H{"updated": true})
}
