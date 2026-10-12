package memberlevelhttp

import (
	"errors"
	"regexp"

	memberlevelcontract "github.com/dujiao-next/internal/modules/memberlevel/contract"
	memberleveldomain "github.com/dujiao-next/internal/modules/memberlevel/domain"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"
	"github.com/gin-gonic/gin"
	"github.com/shopspring/decimal"
)

type UserPriceAdminService interface {
	GetUserPrices(userID uint) ([]memberleveldomain.UserPrice, error)
	ReplaceUserPrices(userID uint, prices []memberleveldomain.UserPrice) error
}

type UserPriceAdminHandler struct{ service UserPriceAdminService }

func NewUserPriceAdminHandler(service UserPriceAdminService) *UserPriceAdminHandler {
	return &UserPriceAdminHandler{service: service}
}

func RegisterUserPriceAdminRoutes(admin gin.IRoutes, handler *UserPriceAdminHandler) {
	admin.GET("/users/:id/prices", handler.Get)
	admin.PUT("/users/:id/prices", handler.Replace)
}

func (h *UserPriceAdminHandler) Get(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil || userID == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	prices, err := h.service.GetUserPrices(userID)
	if err != nil {
		ginutil.RespondError(c, response.CodeInternal, "error.member_level_price_fetch_failed", err)
		return
	}
	if prices == nil {
		prices = []memberleveldomain.UserPrice{}
	}
	response.Success(c, prices)
}

var userPricePattern = regexp.MustCompile(`^[0-9]{1,10}(\.[0-9]{1,2})?$`)

func (h *UserPriceAdminHandler) Replace(c *gin.Context) {
	userID, err := ginutil.ParseParamUint(c, "id")
	if err != nil || userID == 0 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	var req struct {
		Prices *[]struct {
			ProductID   uint   `json:"product_id" binding:"required"`
			SKUID       uint   `json:"sku_id"`
			PriceAmount string `json:"price_amount" binding:"required"`
		} `json:"prices" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if len(*req.Prices) > 5000 {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	prices := make([]memberleveldomain.UserPrice, 0, len(*req.Prices))
	for _, row := range *req.Prices {
		if !userPricePattern.MatchString(row.PriceAmount) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
			return
		}
		amount, err := decimal.NewFromString(row.PriceAmount)
		if err != nil {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
			return
		}
		prices = append(prices, memberleveldomain.UserPrice{UserID: userID, ProductID: row.ProductID, SKUID: row.SKUID, PriceAmount: money.FromDecimal(amount)})
	}
	if err := h.service.ReplaceUserPrices(userID, prices); err != nil {
		if errors.Is(err, memberlevelcontract.ErrUserNotFound) {
			ginutil.RespondError(c, response.CodeNotFound, "error.user_not_found", nil)
		} else if errors.Is(err, memberlevelcontract.ErrPriceInvalid) {
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		} else {
			ginutil.RespondError(c, response.CodeInternal, "error.member_level_price_save_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"saved": true})
}
