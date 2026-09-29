package compliancehttp

import (
	"errors"

	complianceapp "github.com/dujiao-next/internal/modules/compliance/application"
	compliancecontract "github.com/dujiao-next/internal/modules/compliance/contract"
	compliancedomain "github.com/dujiao-next/internal/modules/compliance/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// AdminService 是后台合规声明端口。
type AdminService interface {
	Status() (*compliancedomain.Status, error)
	Acknowledge(command complianceapp.AcknowledgeCommand) error
}

type acknowledgeRequest struct {
	Segment1 string `json:"segment1" binding:"required"`
	Segment2 string `json:"segment2" binding:"required"`
	Segment3 string `json:"segment3" binding:"required"`
}

type statusResponse struct {
	Acknowledged           bool   `json:"acknowledged"`
	AcknowledgedAt         string `json:"acknowledged_at,omitempty"`
	AcknowledgedByAdminID  uint   `json:"acknowledged_by_admin_id,omitempty"`
	AcknowledgedByUsername string `json:"acknowledged_by_username,omitempty"`
	Version                string `json:"version,omitempty"`
}

// AdminHandler 处理后台合规声明确认请求。
type AdminHandler struct {
	svc AdminService
}

func NewAdminHandler(svc AdminService) *AdminHandler {
	if svc == nil {
		panic("compliance admin handler: service is nil")
	}
	return &AdminHandler{svc: svc}
}

// GetComplianceStatus GET /admin/compliance/status
func (h *AdminHandler) GetComplianceStatus(c *gin.Context) {
	status, err := h.svc.Status()
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
		return
	}
	response.Success(c, statusResponse{
		Acknowledged:           status.Acknowledged,
		AcknowledgedAt:         status.AcknowledgedAt,
		AcknowledgedByAdminID:  status.AcknowledgedByAdminID,
		AcknowledgedByUsername: status.AcknowledgedByUsername,
		Version:                status.Version,
	})
}

// AcknowledgeCompliance POST /admin/compliance/acknowledge —— 仅超管
func (h *AdminHandler) AcknowledgeCompliance(c *gin.Context) {
	if !ginutil.IsSuperAdmin(c) {
		ginutil.RespondError(c, response.CodeForbidden, "compliance.error.super_admin_required", nil)
		return
	}
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	username := ""
	if v, exists := c.Get("username"); exists {
		username, _ = v.(string)
	}

	var req acknowledgeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	err := h.svc.Acknowledge(complianceapp.AcknowledgeCommand{
		Segment1:  req.Segment1,
		Segment2:  req.Segment2,
		Segment3:  req.Segment3,
		AdminID:   adminID,
		Username:  username,
		ClientIP:  c.ClientIP(),
		UserAgent: c.Request.UserAgent(),
	})
	if err != nil {
		switch {
		case errors.Is(err, compliancecontract.ErrTextMismatch):
			ginutil.RespondError(c, response.CodeBadRequest, "compliance.error.text_mismatch", nil)
			return
		case errors.Is(err, compliancecontract.ErrAlreadyAcknowledged):
			response.Success(c, gin.H{"already_acknowledged": true})
			return
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.internal", err)
			return
		}
	}
	response.Success(c, gin.H{"already_acknowledged": false})
}
