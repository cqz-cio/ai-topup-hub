package giftcardhttp

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	userdomain "github.com/dujiao-next/internal/modules/identity/user/domain"

	giftcardapp "github.com/dujiao-next/internal/modules/giftcard/application"
	giftcardcontract "github.com/dujiao-next/internal/modules/giftcard/contract"
	giftcarddomain "github.com/dujiao-next/internal/modules/giftcard/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

// AdminService 是后台礼品卡管理端口。
type AdminService interface {
	Generate(input giftcardapp.GenerateInput) (*giftcarddomain.GiftCardBatch, int, error)
	List(input giftcardapp.ListInput) ([]giftcarddomain.GiftCard, int64, error)
	ResolveRedeemedUsers(cards []giftcarddomain.GiftCard) (map[uint]userdomain.User, error)
	Update(id uint, input giftcardapp.UpdateInput) (*giftcarddomain.GiftCard, error)
	Delete(id uint) error
	BatchUpdateStatus(ids []uint, status string) (int64, error)
	Export(ids []uint, format string) ([]byte, string, error)
}

// AdminHandler 处理后台礼品卡请求。
type AdminHandler struct {
	cards AdminService
}

func NewAdminHandler(cards AdminService) *AdminHandler {
	if cards == nil {
		panic("giftcard admin handler: cards is nil")
	}
	return &AdminHandler{cards: cards}
}

type generateRequest struct {
	Name      string `json:"name" binding:"required"`
	Quantity  int    `json:"quantity" binding:"required"`
	Amount    string `json:"amount" binding:"required"`
	ExpiresAt string `json:"expires_at"`
}

type updateRequest struct {
	Name      *string `json:"name"`
	Status    *string `json:"status"`
	ExpiresAt *string `json:"expires_at"`
}

type batchUpdateStatusRequest struct {
	IDs    []uint `json:"ids" binding:"required"`
	Status string `json:"status" binding:"required"`
}

type exportRequest struct {
	IDs    []uint `json:"ids" binding:"required"`
	Format string `json:"format" binding:"required"`
}

type adminGiftCardUser struct {
	ID          uint   `json:"id"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
}

type adminGiftCardItem struct {
	giftcarddomain.GiftCard
	IsExpired    bool               `json:"is_expired"`
	RedeemedUser *adminGiftCardUser `json:"redeemed_user,omitempty"`
}

// Generate 管理端生成礼品卡。
func (h *AdminHandler) Generate(c *gin.Context) {
	adminID, ok := ginutil.GetAdminID(c)
	if !ok {
		return
	}
	var req generateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := decimal.NewFromString(strings.TrimSpace(req.Amount))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	expiresAt, err := ginutil.ParseTimeNullable(strings.TrimSpace(req.ExpiresAt))
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	batch, created, err := h.cards.Generate(giftcardapp.GenerateInput{
		Name:      req.Name,
		Quantity:  req.Quantity,
		Amount:    money.FromDecimal(amount),
		ExpiresAt: expiresAt,
		CreatedBy: &adminID,
	})
	if err != nil {
		switch {
		case errors.Is(err, giftcardcontract.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.gift_card_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.gift_card_create_failed", err)
		}
		return
	}
	response.Success(c, gin.H{
		"batch":   batch,
		"created": created,
	})
}

// List 获取礼品卡列表。
func (h *AdminHandler) List(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	status := strings.TrimSpace(strings.ToLower(c.Query("status")))
	code := strings.TrimSpace(c.Query("code"))
	batchNo := strings.TrimSpace(c.Query("batch_no"))

	var redeemedUserID uint
	if rawUserID := strings.TrimSpace(c.Query("redeemed_user_id")); rawUserID != "" {
		parsed, err := ginutil.ParseQueryUint(rawUserID, true)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		redeemedUserID = parsed
	}

	createdFrom, createdTo, err := ginutil.ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	redeemedFrom, redeemedTo, err := ginutil.ParseQueryTimeRange(c, "redeemed_from", "redeemed_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	expiresFrom, expiresTo, err := ginutil.ParseQueryTimeRange(c, "expires_from", "expires_to")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}

	cards, total, err := h.cards.List(giftcardapp.ListInput{
		Code:           code,
		Status:         status,
		BatchNo:        batchNo,
		RedeemedUserID: redeemedUserID,
		CreatedFrom:    createdFrom,
		CreatedTo:      createdTo,
		RedeemedFrom:   redeemedFrom,
		RedeemedTo:     redeemedTo,
		ExpiresFrom:    expiresFrom,
		ExpiresTo:      expiresTo,
		Page:           page,
		PageSize:       pageSize,
	})
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.gift_card_fetch_failed", err)
		return
	}

	userMap, err := h.cards.ResolveRedeemedUsers(cards)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.gift_card_fetch_failed", err)
		return
	}

	now := time.Now()
	items := make([]adminGiftCardItem, 0, len(cards))
	for _, card := range cards {
		item := adminGiftCardItem{
			GiftCard:  card,
			IsExpired: card.ExpiresAt != nil && card.ExpiresAt.Before(now),
		}
		if card.RedeemedUserID != nil {
			if user, ok := userMap[*card.RedeemedUserID]; ok {
				item.RedeemedUser = &adminGiftCardUser{
					ID:          user.ID,
					Email:       user.Email,
					DisplayName: user.DisplayName,
				}
			}
		}
		items = append(items, item)
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}

// Update 更新礼品卡。
func (h *AdminHandler) Update(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	var (
		expiresAt      *time.Time
		clearExpiresAt bool
	)
	if req.ExpiresAt != nil {
		if strings.TrimSpace(*req.ExpiresAt) == "" {
			clearExpiresAt = true
		} else {
			parsed, err := ginutil.ParseTimeNullable(strings.TrimSpace(*req.ExpiresAt))
			if err != nil {
				ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
				return
			}
			expiresAt = parsed
		}
	}
	card, err := h.cards.Update(id, giftcardapp.UpdateInput{
		Name:           req.Name,
		Status:         req.Status,
		ExpiresAt:      expiresAt,
		ClearExpiresAt: clearExpiresAt,
	})
	if err != nil {
		switch {
		case errors.Is(err, giftcardcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.gift_card_not_found", nil)
		case errors.Is(err, giftcardcontract.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.gift_card_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.gift_card_update_failed", err)
		}
		return
	}
	response.Success(c, card)
}

// Delete 删除礼品卡。
func (h *AdminHandler) Delete(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	if err := h.cards.Delete(id); err != nil {
		switch {
		case errors.Is(err, giftcardcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.gift_card_not_found", nil)
		case errors.Is(err, giftcardcontract.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.gift_card_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.gift_card_delete_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// BatchUpdateStatus 批量更新礼品卡状态。
func (h *AdminHandler) BatchUpdateStatus(c *gin.Context) {
	var req batchUpdateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	affected, err := h.cards.BatchUpdateStatus(req.IDs, req.Status)
	if err != nil {
		switch {
		case errors.Is(err, giftcardcontract.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.gift_card_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.gift_card_update_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"affected": affected})
}

// Export 导出礼品卡。
func (h *AdminHandler) Export(c *gin.Context) {
	var req exportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	content, contentType, err := h.cards.Export(req.IDs, req.Format)
	if err != nil {
		switch {
		case errors.Is(err, giftcardcontract.ErrNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.gift_card_not_found", nil)
		case errors.Is(err, giftcardcontract.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.gift_card_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.gift_card_fetch_failed", err)
		}
		return
	}
	filename := fmt.Sprintf("gift_cards_%s.%s", time.Now().Format("20060102_150405"), strings.ToLower(strings.TrimSpace(req.Format)))
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	c.Data(http.StatusOK, contentType, content)
}
