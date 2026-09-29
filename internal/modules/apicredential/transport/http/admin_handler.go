package apicredentialhttp

import (
	"errors"

	apicredentialdomain "github.com/dujiao-next/internal/modules/apicredential/domain"

	apicredentialcontract "github.com/dujiao-next/internal/modules/apicredential/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminService 是管理端 API 凭证接口实际使用的应用能力。
type AdminService interface {
	List(apicredentialcontract.ListFilter) ([]apicredentialdomain.ApiCredential, int64, error)
	GetByID(id uint) (*apicredentialdomain.ApiCredential, error)
	Approve(id uint) (*apicredentialdomain.ApiCredential, string, error)
	Reject(id uint, reason string) error
	SetActive(id uint, active bool) error
	Delete(id uint) error
}

type AdminHandler struct {
	service AdminService
}

func NewAdminHandler(service AdminService) *AdminHandler {
	return &AdminHandler{service: service}
}

// GetApiCredentials 获取 API 凭证列表
func (h *AdminHandler) GetApiCredentials(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)
	status := c.Query("status")
	search := c.Query("search")
	userID, _ := ginutil.ParseQueryUint(c.Query("user_id"), false)

	creds, total, err := h.service.List(apicredentialcontract.ListFilter{
		Status:   status,
		UserID:   userID,
		Search:   search,
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.api_credential_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, creds, pagination)
}

// GetApiCredential 获取 API 凭证详情
func (h *AdminHandler) GetApiCredential(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	cred, err := h.service.GetByID(id)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.api_credential_fetch_failed", err)
		return
	}
	if cred == nil {
		ginutil.RespondError(c, response.CodeNotFound, "error.api_credential_not_found", nil)
		return
	}

	response.Success(c, cred)
}

// ApproveApiCredential 审核通过 API 凭证
func (h *AdminHandler) ApproveApiCredential(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	cred, _, err := h.service.Approve(id)
	if err != nil {
		if errors.Is(err, apicredentialcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.api_credential_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.api_credential_approve_failed", err)
		return
	}

	response.Success(c, gin.H{
		"credential": cred,
		"approved":   true,
	})
}

// RejectApiCredentialRequest 拒绝凭证请求
type RejectApiCredentialRequest struct {
	Reason string `json:"reason" binding:"required"`
}

// RejectApiCredential 审核拒绝 API 凭证
func (h *AdminHandler) RejectApiCredential(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	var req RejectApiCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.service.Reject(id, req.Reason); err != nil {
		if errors.Is(err, apicredentialcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.api_credential_not_found", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.api_credential_reject_failed", err)
		return
	}

	response.Success(c, gin.H{"rejected": true})
}

// UpdateApiCredentialStatusRequest 更新凭证状态请求
type UpdateApiCredentialStatusRequest struct {
	IsActive bool `json:"is_active"`
}

// UpdateApiCredentialStatus 启用/禁用 API 凭证
func (h *AdminHandler) UpdateApiCredentialStatus(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	var req UpdateApiCredentialStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	if err := h.service.SetActive(id, req.IsActive); err != nil {
		if errors.Is(err, apicredentialcontract.ErrNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.api_credential_not_found", nil)
			return
		}
		if errors.Is(err, apicredentialcontract.ErrNotApproved) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.api_credential_not_approved", nil)
			return
		}
		ginutil.RespondError(c, response.CodeInternal, "error.api_credential_update_failed", err)
		return
	}

	response.Success(c, gin.H{"updated": true})
}

// DeleteApiCredential 删除 API 凭证
func (h *AdminHandler) DeleteApiCredential(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	if err := h.service.Delete(id); err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.api_credential_delete_failed", err)
		return
	}

	response.Success(c, gin.H{"deleted": true})
}
