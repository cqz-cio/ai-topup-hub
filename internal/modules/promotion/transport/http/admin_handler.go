package promotionhttp

import (
	"errors"

	promotionapp "github.com/dujiao-next/internal/modules/promotion/application"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	promotiondomain "github.com/dujiao-next/internal/modules/promotion/domain"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// AdminService 是管理端 HTTP 层所需的最小活动价服务接口。
type AdminService interface {
	Create(input promotionapp.CreatePromotionInput) (*promotiondomain.Promotion, error)
	Update(id uint, input promotionapp.UpdatePromotionInput) (*promotiondomain.Promotion, error)
	Delete(id uint) error
	List(filter promotioncontract.ListFilter) ([]promotiondomain.Promotion, int64, error)
}

// AdminHandler 处理活动价管理端请求。
type AdminHandler struct {
	service AdminService
}

func NewAdminHandler(service AdminService) *AdminHandler {
	if service == nil {
		panic("promotion admin handler: service is nil")
	}
	return &AdminHandler{service: service}
}

// CreatePromotionRequest 创建活动价请求
type CreatePromotionRequest struct {
	Name       string  `json:"name" binding:"required"`
	Type       string  `json:"type" binding:"required"`
	ScopeRefID uint    `json:"scope_ref_id" binding:"required"`
	Value      float64 `json:"value" binding:"required"`
	MinAmount  float64 `json:"min_amount"`
	StartsAt   string  `json:"starts_at"`
	EndsAt     string  `json:"ends_at"`
	IsActive   *bool   `json:"is_active"`
}

func buildCreatePromotionInputFromRequest(req CreatePromotionRequest) (promotionapp.CreatePromotionInput, error) {
	startsAt, err := ginutil.ParseTimeNullable(req.StartsAt)
	if err != nil {
		return promotionapp.CreatePromotionInput{}, err
	}
	endsAt, err := ginutil.ParseTimeNullable(req.EndsAt)
	if err != nil {
		return promotionapp.CreatePromotionInput{}, err
	}
	return promotionapp.CreatePromotionInput{
		Name:       req.Name,
		Type:       req.Type,
		ScopeRefID: req.ScopeRefID,
		Value:      money.FromDecimal(decimal.NewFromFloat(req.Value)),
		MinAmount:  money.FromDecimal(decimal.NewFromFloat(req.MinAmount)),
		StartsAt:   startsAt,
		EndsAt:     endsAt,
		IsActive:   req.IsActive,
	}, nil
}

func buildUpdatePromotionInputFromRequest(req CreatePromotionRequest) (promotionapp.UpdatePromotionInput, error) {
	input, err := buildCreatePromotionInputFromRequest(req)
	if err != nil {
		return promotionapp.UpdatePromotionInput{}, err
	}
	return promotionapp.UpdatePromotionInput(input), nil
}

// CreatePromotion 创建活动价
func (h *AdminHandler) CreatePromotion(c *gin.Context) {
	var req CreatePromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	input, err := buildCreatePromotionInputFromRequest(req)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	created, err := h.service.Create(input)
	if err != nil {
		switch {
		case errors.Is(err, promotioncontract.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.promotion_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.promotion_create_failed", err)
		}
		return
	}

	response.Success(c, created)
}

// UpdatePromotion 更新活动价
func (h *AdminHandler) UpdatePromotion(c *gin.Context) {
	promotionID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var req CreatePromotionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}

	input, err := buildUpdatePromotionInputFromRequest(req)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	updated, err := h.service.Update(promotionID, input)
	if err != nil {
		switch {
		case errors.Is(err, promotioncontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.promotion_not_found", nil)
		case errors.Is(err, promotioncontract.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.promotion_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.promotion_update_failed", err)
		}
		return
	}

	response.Success(c, updated)
}

// DeletePromotion 删除活动价
func (h *AdminHandler) DeletePromotion(c *gin.Context) {
	promotionID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	if err := h.service.Delete(promotionID); err != nil {
		switch {
		case errors.Is(err, promotioncontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.promotion_not_found", nil)
		case errors.Is(err, promotioncontract.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.promotion_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.promotion_delete_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"deleted": true,
	})
}

// GetAdminPromotions 获取活动价列表
func (h *AdminHandler) GetAdminPromotions(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	id, err := ginutil.ParseQueryUint(c.Query("id"), true)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	scopeRefID, _ := ginutil.ParseQueryUint(c.Query("scope_ref_id"), false)

	isActive, err := ginutil.ParseQueryBoolPtr(c, "is_active")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	promotions, total, err := h.service.List(promotioncontract.ListFilter{
		ID:         id,
		Name:       c.Query("name"),
		ScopeRefID: scopeRefID,
		IsActive:   isActive,
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.promotion_fetch_failed", err)
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, promotions, pagination)
}
