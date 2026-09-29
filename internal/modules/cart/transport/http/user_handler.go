package carthttp

import (
	"errors"

	"github.com/dujiao-next/internal/constants"
	cartapp "github.com/dujiao-next/internal/modules/cart/application"
	cartcontract "github.com/dujiao-next/internal/modules/cart/contract"
	cartpresenter "github.com/dujiao-next/internal/modules/cart/transport/presenter"
	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"
	promotioncontract "github.com/dujiao-next/internal/modules/promotion/contract"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

// Service 购物车应用端口。
type Service interface {
	ListByUser(userID uint) ([]cartapp.ItemDetail, error)
	UpsertItem(input cartapp.UpsertItemInput) error
	RemoveItem(userID, productID, skuID uint) error
}

// UserHandler 处理用户购物车 HTTP 请求。
type UserHandler struct {
	carts Service
}

func NewUserHandler(carts Service) *UserHandler {
	if carts == nil {
		panic("cart user handler: carts is nil")
	}
	return &UserHandler{carts: carts}
}

// CartItemRequest 购物车项请求。
type CartItemRequest struct {
	ProductID       uint   `json:"product_id" binding:"required"`
	SKUID           uint   `json:"sku_id"`
	Quantity        int    `json:"quantity" binding:"required"`
	FulfillmentType string `json:"fulfillment_type"`
}

// GetCart 获取购物车。
func (h *UserHandler) GetCart(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}

	items, err := h.carts.ListByUser(uid)
	if err != nil {
		switch {
		case errors.Is(err, cartcontract.ErrInvalidItem):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		case errors.Is(err, cartcontract.ErrProductUnavailable):
			ginutil.RespondError(c, response.CodeBadRequest, "error.product_not_available", nil)
		case errors.Is(err, cartcontract.ErrFulfillmentInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.fulfillment_invalid", nil)
		case errors.Is(err, promotioncontract.ErrInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.promotion_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		}
		return
	}

	respItems := make([]cartpresenter.CartItemResp, 0, len(items))
	for _, item := range items {
		if item.Product == nil {
			continue
		}
		productFT := item.Product.FulfillmentType
		if productFT == constants.FulfillmentTypeUpstream {
			productFT = constants.FulfillmentTypeManual
		}
		cartFT := item.FulfillmentType
		if cartFT == constants.FulfillmentTypeUpstream {
			cartFT = constants.FulfillmentTypeManual
		}
		product := cartpresenter.CartProductResp{
			Slug:                item.Product.Slug,
			Title:               item.Product.TitleJSON,
			PriceAmount:         item.Product.PriceAmount,
			Images:              item.Product.Images,
			Tags:                item.Product.Tags,
			PurchaseType:        item.Product.PurchaseType,
			MinPurchaseQuantity: item.Product.MinPurchaseQuantity,
			MaxPurchaseQuantity: item.Product.MaxPurchaseQuantity,
			FulfillmentType:     productFT,
			IsActive:            item.Product.IsActive,
		}
		respItems = append(respItems, cartpresenter.CartItemResp{
			ProductID:       item.ProductID,
			SKUID:           item.SKUID,
			Quantity:        item.Quantity,
			FulfillmentType: cartFT,
			UnitPrice:       item.UnitPrice,
			OriginalPrice:   item.OriginalPrice,
			Currency:        item.Currency,
			Product:         product,
		})
	}

	response.Success(c, gin.H{"items": respItems})
}

// UpsertCartItem 添加/更新购物车项。
func (h *UserHandler) UpsertCartItem(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	var req CartItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	if req.Quantity <= 0 {
		if err := h.carts.RemoveItem(uid, req.ProductID, req.SKUID); err != nil {
			ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
			return
		}
		response.Success(c, gin.H{"updated": true})
		return
	}
	if err := h.carts.UpsertItem(cartapp.UpsertItemInput{
		UserID:          uid,
		ProductID:       req.ProductID,
		SKUID:           req.SKUID,
		Quantity:        req.Quantity,
		FulfillmentType: req.FulfillmentType,
	}); err != nil {
		respondCartItemUpdateError(c, err)
		return
	}
	response.Success(c, gin.H{"updated": true})
}

// DeleteCartItem 删除购物车项。
func (h *UserHandler) DeleteCartItem(c *gin.Context) {
	uid, ok := ginutil.GetUserID(c)
	if !ok {
		return
	}
	productID, err := ginutil.ParseParamUint(c, "product_id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	skuID, err := ginutil.ParseQueryUint(c.DefaultQuery("sku_id", "0"), false)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	if err := h.carts.RemoveItem(uid, productID, skuID); err != nil {
		switch {
		case errors.Is(err, cartcontract.ErrInvalidItem):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
		}
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func respondCartItemUpdateError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, cartcontract.ErrSKURequired),
		errors.Is(err, cartcontract.ErrSKUInvalid),
		errors.Is(err, cartcontract.ErrInvalidItem):
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
	case errors.Is(err, productdomain.ErrMaxPurchaseExceeded):
		ginutil.RespondError(c, response.CodeBadRequest, "error.product_max_purchase_exceeded", nil)
	case errors.Is(err, productdomain.ErrMinPurchaseNotMet):
		ginutil.RespondError(c, response.CodeBadRequest, "error.product_min_purchase_not_met", nil)
	case errors.Is(err, cartcontract.ErrProductUnavailable):
		ginutil.RespondError(c, response.CodeBadRequest, "error.product_not_available", nil)
	case errors.Is(err, cartcontract.ErrManualStockInsufficient):
		ginutil.RespondError(c, response.CodeBadRequest, "error.manual_stock_insufficient", nil)
	case errors.Is(err, cartcontract.ErrFulfillmentInvalid):
		ginutil.RespondError(c, response.CodeBadRequest, "error.fulfillment_invalid", nil)
	default:
		ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
	}
}
