package channelhttp

import (
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

type channelOrderItemRequest struct {
	ProductID       uint   `json:"product_id"`
	SKUID           uint   `json:"sku_id"`
	Quantity        int    `json:"quantity"`
	FulfillmentType string `json:"fulfillment_type"`
}

type previewOrderRequest struct {
	ChannelUserID  string                    `json:"channel_user_id"`
	TelegramUserID string                    `json:"telegram_user_id"`
	Username       string                    `json:"username"`
	TelegramUser   string                    `json:"telegram_username"`
	FirstName      string                    `json:"first_name"`
	LastName       string                    `json:"last_name"`
	AvatarURL      string                    `json:"avatar_url"`
	Locale         string                    `json:"locale"`
	Items          []channelOrderItemRequest `json:"items"`
	CouponCode     string                    `json:"coupon_code"`
	AffiliateCode  string                    `json:"affiliate_code"`
	AffiliateKey   string                    `json:"affiliate_visitor_key"`
	ManualFormData map[string]jsonmap.JSON   `json:"manual_form_data"`
}

type createOrderRequest struct {
	ChannelUserID  string                    `json:"channel_user_id"`
	TelegramUserID string                    `json:"telegram_user_id"`
	Username       string                    `json:"username"`
	TelegramUser   string                    `json:"telegram_username"`
	FirstName      string                    `json:"first_name"`
	LastName       string                    `json:"last_name"`
	AvatarURL      string                    `json:"avatar_url"`
	Locale         string                    `json:"locale"`
	Items          []channelOrderItemRequest `json:"items"`
	ProductID      uint                      `json:"product_id"`
	SKUID          uint                      `json:"sku_id"`
	Quantity       int                       `json:"quantity"`
	CouponCode     string                    `json:"coupon_code"`
	AffiliateCode  string                    `json:"affiliate_code"`
	AffiliateKey   string                    `json:"affiliate_visitor_key"`
	ManualFormData map[string]jsonmap.JSON   `json:"manual_form_data"`
}

// PreviewOrder POST /api/v1/channel/orders/preview
func (h *Handler) PreviewOrder(c *gin.Context) {
	var req previewOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondChannelBindError(c, err)
		return
	}

	items, err := buildChannelOrderItems(req.Items, 0, 0, 0)
	if err != nil {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.provisionTelegramChannelUserID(telegramChannelIdentityInput(
		req.ChannelUserID,
		req.TelegramUserID,
		req.Username,
		req.TelegramUser,
		req.FirstName,
		req.LastName,
		req.AvatarURL,
	))
	if err != nil {
		logger.Errorw("channel_order_preview_resolve_user", "channel_user_id", channelUserIDValue(req.ChannelUserID, req.TelegramUserID), "error", err)
		respondChannelIdentityServiceError(c, err)
		return
	}

	preview, err := h.OrderService.PreviewOrder(CreateOrderInput{
		UserID:              userID,
		Items:               items,
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
	})
	if err != nil {
		logger.Errorw("channel_order_preview", "user_id", userID, "error", err)
		respondChannelOrderPreviewError(c, err)
		return
	}

	locale := channelLocaleValue(c, req.Locale)
	respondChannelSuccess(c, buildChannelOrderPreviewResponse(preview, locale))
}

// CreateOrder POST /api/v1/channel/orders
func (h *Handler) CreateOrder(c *gin.Context) {
	var req createOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondChannelBindError(c, err)
		return
	}

	items, err := buildChannelOrderItems(req.Items, req.ProductID, req.SKUID, req.Quantity)
	if err != nil {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.provisionTelegramChannelUserID(telegramChannelIdentityInput(
		req.ChannelUserID,
		req.TelegramUserID,
		req.Username,
		req.TelegramUser,
		req.FirstName,
		req.LastName,
		req.AvatarURL,
	))
	if err != nil {
		logger.Errorw("channel_order_resolve_user", "channel_user_id", channelUserIDValue(req.ChannelUserID, req.TelegramUserID), "error", err)
		respondChannelIdentityServiceError(c, err)
		return
	}

	order, err := h.OrderService.CreateOrder(CreateOrderInput{
		UserID:              userID,
		Items:               items,
		CouponCode:          req.CouponCode,
		AffiliateCode:       req.AffiliateCode,
		AffiliateVisitorKey: req.AffiliateKey,
		ClientIP:            c.ClientIP(),
		ManualFormData:      req.ManualFormData,
		SkipIPRiskControl:   true, // Bot 服务器 IP 共用，跳过 IP 维度风控避免误杀
	})
	if err != nil {
		logger.Errorw("channel_order_create", "user_id", userID, "error", err)
		respondChannelOrderCreateError(c, err)
		return
	}

	respondChannelSuccess(c, gin.H{
		"order_id":           order.ID,
		"order_no":           order.OrderNo,
		"status":             order.Status,
		"fulfillment_type":   channelOrderFulfillmentType(order),
		"currency":           order.Currency,
		"item_count":         len(order.Items),
		"original_amount":    order.OriginalAmount.StringFixed(2),
		"coupon_discount":    order.DiscountAmount.StringFixed(2),
		"promotion_discount": order.PromotionDiscountAmount.StringFixed(2),
		"wholesale_discount": order.WholesaleDiscountAmount.StringFixed(2),
		"total_amount":       order.TotalAmount.StringFixed(2),
		"wallet_paid_amount": order.WalletPaidAmount.StringFixed(2),
		"online_paid_amount": order.OnlinePaidAmount.StringFixed(2),
		"paid_amount":        channelOrderPaidAmount(order),
		"refunded_amount":    order.RefundedAmount.StringFixed(2),
		"expires_at":         order.ExpiresAt,
		"created_at":         order.CreatedAt,
	})
}

func buildChannelOrderItems(items []channelOrderItemRequest, legacyProductID, legacySKUID uint, legacyQuantity int) ([]CreateOrderItem, error) {
	if len(items) == 0 {
		if legacyProductID == 0 || legacyQuantity <= 0 {
			return nil, ErrInvalidOrderItem
		}
		items = []channelOrderItemRequest{{
			ProductID: legacyProductID,
			SKUID:     legacySKUID,
			Quantity:  legacyQuantity,
		}}
	}

	result := make([]CreateOrderItem, 0, len(items))
	for _, item := range items {
		if item.ProductID == 0 || item.Quantity <= 0 {
			return nil, ErrInvalidOrderItem
		}
		result = append(result, CreateOrderItem(item))
	}
	return result, nil
}

func buildChannelOrderPreviewResponse(preview *OrderPreview, locale string) gin.H {
	items := make([]gin.H, 0, len(preview.Items))
	for _, item := range preview.Items {
		items = append(items, gin.H{
			"product_id":           item.ProductID,
			"product_title":        resolveLocalizedJSON(item.TitleJSON, locale, "zh-CN"),
			"sku_id":               item.SKUID,
			"sku_name":             channelLocalizedValue(item.SKUSnapshotJSON["spec_values"], locale, "zh-CN"),
			"quantity":             item.Quantity,
			"original_unit_price":  item.OriginalUnitPrice.StringFixed(2),
			"unit_price":           item.UnitPrice.StringFixed(2),
			"original_total_price": item.OriginalTotalPrice.StringFixed(2),
			"subtotal":             item.TotalPrice.StringFixed(2),
			"coupon_discount":      item.CouponDiscount.StringFixed(2),
			"promotion_discount":   item.PromotionDiscount.StringFixed(2),
			"wholesale_discount":   item.WholesaleDiscount.StringFixed(2),
			"fulfillment_type":     item.FulfillmentType,
		})
	}
	return gin.H{
		"item_count":         len(items),
		"original_amount":    preview.OriginalAmount.StringFixed(2),
		"items":              items,
		"coupon_discount":    preview.DiscountAmount.StringFixed(2),
		"promotion_discount": preview.PromotionDiscountAmount.StringFixed(2),
		"wholesale_discount": preview.WholesaleDiscountAmount.StringFixed(2),
		"total_amount":       preview.TotalAmount.StringFixed(2),
		"currency":           preview.Currency,
		"valid":              true,
		"validation_errors":  []string{},
	}
}
