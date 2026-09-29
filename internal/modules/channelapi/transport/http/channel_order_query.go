package channelhttp

import (
	"errors"
	"strings"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/gin-gonic/gin"
)

type orderListItem struct {
	OrderID          uint    `json:"order_id"`
	OrderNo          string  `json:"order_no"`
	Status           string  `json:"status"`
	Currency         string  `json:"currency"`
	TotalAmount      string  `json:"total_amount"`
	PaidAmount       string  `json:"paid_amount"`
	WalletPaidAmount string  `json:"wallet_paid_amount"`
	OnlinePaidAmount string  `json:"online_paid_amount"`
	ProductTitle     string  `json:"product_title"`
	ItemCount        int     `json:"item_count"`
	ExpiresAt        *string `json:"expires_at,omitempty"`
	CreatedAt        string  `json:"created_at"`
}

type cancelOrderRequest struct {
	ChannelUserID  string `json:"channel_user_id"`
	TelegramUserID string `json:"telegram_user_id"`
	Reason         string `json:"reason"`
}

// GetOrderStatus GET /api/v1/channel/orders/:id
func (h *Handler) GetOrderStatus(c *gin.Context) {
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	channelUserID := channelUserIDFromQuery(c)
	if channelUserID == "" {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.provisionTelegramChannelUserID(TelegramIdentityInput{ChannelUserID: channelUserID})
	if err != nil {
		logger.Errorw("channel_order_status_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityServiceError(c, err)
		return
	}

	order, err := h.OrderService.GetOrderByUser(orderID, userID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			respondChannelError(c, 404, response.CodeNotFound, "order_not_found", "error.order_not_found", nil)
			return
		}
		respondChannelError(c, 500, response.CodeInternal, "internal_error", "error.internal_error", err)
		return
	}
	if order == nil {
		respondChannelError(c, 404, response.CodeNotFound, "order_not_found", "error.order_not_found", nil)
		return
	}

	order.MaskUpstreamFulfillmentType()
	order.StripCostPrice()
	respondChannelSuccess(c, buildChannelOrderDetailResponse(order, channelLocaleValue(c, c.Query("locale"))))
}

// GetOrderByOrderNo GET /api/v1/channel/orders/by-order-no/:order_no
func (h *Handler) GetOrderByOrderNo(c *gin.Context) {
	orderNo := strings.TrimSpace(c.Param("order_no"))
	if orderNo == "" {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	channelUserID := channelUserIDFromQuery(c)
	if channelUserID == "" {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.provisionTelegramChannelUserID(TelegramIdentityInput{ChannelUserID: channelUserID})
	if err != nil {
		logger.Errorw("channel_order_by_no_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityServiceError(c, err)
		return
	}

	order, err := h.OrderService.GetOrderByUserOrderNo(orderNo, userID)
	if err != nil {
		if errors.Is(err, ErrOrderNotFound) {
			respondChannelError(c, 404, response.CodeNotFound, "order_not_found", "error.order_not_found", nil)
			return
		}
		respondChannelError(c, 500, response.CodeInternal, "internal_error", "error.order_fetch_failed", err)
		return
	}
	if order == nil {
		respondChannelError(c, 404, response.CodeNotFound, "order_not_found", "error.order_not_found", nil)
		return
	}

	order.MaskUpstreamFulfillmentType()
	order.StripCostPrice()
	respondChannelSuccess(c, buildChannelOrderDetailResponse(order, channelLocaleValue(c, c.Query("locale"))))
}

// CancelOrder POST /api/v1/channel/orders/:id/cancel
func (h *Handler) CancelOrder(c *gin.Context) {
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	var req cancelOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondChannelBindError(c, err)
		return
	}

	channelUserID := channelUserIDValue(req.ChannelUserID, req.TelegramUserID)
	if channelUserID == "" {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.provisionTelegramChannelUserID(TelegramIdentityInput{ChannelUserID: channelUserID})
	if err != nil {
		logger.Errorw("channel_order_cancel_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityServiceError(c, err)
		return
	}

	order, err := h.OrderService.CancelOrder(orderID, userID)
	if err != nil {
		logger.Errorw("channel_order_cancel", "order_id", orderID, "error", err)
		if errors.Is(err, ErrOrderNotFound) {
			respondChannelError(c, 404, response.CodeNotFound, "order_not_found", "error.order_not_found", nil)
			return
		}
		respondChannelError(c, 400, response.CodeBadRequest, "order_status_invalid", "error.order_status_invalid", err)
		return
	}

	respondChannelSuccess(c, gin.H{
		"order_id":     order.ID,
		"order_no":     order.OrderNo,
		"status":       order.Status,
		"cancelled_at": order.CanceledAt,
	})
}

// ListOrders GET /api/v1/channel/orders
func (h *Handler) ListOrders(c *gin.Context) {
	channelUserID := channelUserIDFromQuery(c)
	if channelUserID == "" {
		respondChannelError(c, 400, response.CodeBadRequest, "validation_error", "error.bad_request", nil)
		return
	}

	userID, err := h.provisionTelegramChannelUserID(TelegramIdentityInput{ChannelUserID: channelUserID})
	if err != nil {
		logger.Errorw("channel_order_list_resolve_user", "channel_user_id", channelUserID, "error", err)
		respondChannelIdentityServiceError(c, err)
		return
	}

	page, pageSize := ginutil.ParsePaginationWithBounds(c, "page", "page_size", 5, 20)
	status := c.Query("status")
	locale := channelLocaleValue(c, c.Query("locale"))

	orders, total, err := h.OrderService.ListOrdersByUser(OrderListFilter{
		Page:     page,
		PageSize: pageSize,
		UserID:   userID,
		Status:   status,
	})
	if err != nil {
		logger.Errorw("channel_order_list", "user_id", userID, "error", err)
		respondChannelError(c, 500, response.CodeInternal, "internal_error", "error.internal_error", err)
		return
	}

	items := make([]orderListItem, 0, len(orders))
	for _, order := range orders {
		productTitle := ""
		if len(order.Items) > 0 {
			productTitle = resolveLocalizedJSON(order.Items[0].TitleJSON, locale, "zh-CN")
		}
		items = append(items, orderListItem{
			OrderID:          order.ID,
			OrderNo:          order.OrderNo,
			Status:           order.Status,
			Currency:         order.Currency,
			TotalAmount:      order.TotalAmount.StringFixed(2),
			PaidAmount:       channelOrderPaidAmount(&order),
			WalletPaidAmount: order.WalletPaidAmount.StringFixed(2),
			OnlinePaidAmount: order.OnlinePaidAmount.StringFixed(2),
			ProductTitle:     productTitle,
			ItemCount:        len(order.Items),
			ExpiresAt:        formatChannelNullableTime(order.ExpiresAt),
			CreatedAt:        order.CreatedAt.Format(time.RFC3339),
		})
	}

	totalPages := (total + int64(pageSize) - 1) / int64(pageSize)
	respondChannelSuccess(c, gin.H{
		"items":       items,
		"page":        page,
		"page_size":   pageSize,
		"total":       total,
		"total_pages": totalPages,
	})
}

// joinLocalizedInstructions 拼接 items 的多语言交付说明（去重，按 locale 取值）。
func joinLocalizedInstructions(items []orderdomain.OrderItem, locale string) string {
	if len(items) == 0 {
		return ""
	}
	seen := make(map[string]struct{}, len(items))
	var parts []string
	for _, item := range items {
		text := strings.TrimSpace(resolveLocalizedJSON(item.InstructionsJSON, locale, "zh-CN"))
		if text == "" {
			continue
		}
		if _, ok := seen[text]; ok {
			continue
		}
		seen[text] = struct{}{}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n\n")
}

func buildChannelOrderDetailResponse(order *orderdomain.Order, locale string) gin.H {
	resp := gin.H{
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
		"updated_at":         order.UpdatedAt,
		"paid_at":            order.PaidAt,
		"cancelled_at":       order.CanceledAt,
	}

	orderPaid := order.PaidAt != nil
	items := make([]gin.H, 0, len(order.Items))
	for _, item := range order.Items {
		instructions := ""
		if orderPaid {
			instructions = resolveLocalizedJSON(item.InstructionsJSON, locale, "zh-CN")
		}
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
			"instructions":         instructions,
		})
	}
	resp["items"] = items

	children := make([]gin.H, 0, len(order.Children))
	for _, child := range order.Children {
		childInstructions := ""
		if orderPaid {
			childInstructions = joinLocalizedInstructions(child.Items, locale)
		}
		childResp := gin.H{
			"order_id": child.ID,
			"order_no": child.OrderNo,
			"status":   child.Status,
		}
		if child.Fulfillment != nil {
			childResp["fulfillment"] = gin.H{
				"status":       child.Fulfillment.Status,
				"type":         child.Fulfillment.Type,
				"payload":      child.Fulfillment.Payload,
				"delivered_at": child.Fulfillment.DeliveredAt,
				"instructions": childInstructions,
			}
		} else {
			childResp["fulfillment"] = nil
		}
		children = append(children, childResp)
	}
	resp["children"] = children

	parentInstructions := ""
	if orderPaid {
		parentInstructions = joinLocalizedInstructions(order.Items, locale)
	}
	if order.Fulfillment != nil {
		resp["fulfillment_status"] = order.Fulfillment.Status
		resp["fulfillment_result"] = order.Fulfillment.Payload
		resp["fulfillment_delivered_at"] = order.Fulfillment.DeliveredAt
		resp["fulfillment_instructions"] = parentInstructions
	} else {
		resp["fulfillment_status"] = ""
		resp["fulfillment_result"] = nil
		resp["fulfillment_delivered_at"] = nil
		resp["fulfillment_instructions"] = ""
	}

	return resp
}

func channelOrderFulfillmentType(order *orderdomain.Order) string {
	if order == nil {
		return ""
	}
	if order.Fulfillment != nil {
		if fulfillmentType := strings.TrimSpace(order.Fulfillment.Type); fulfillmentType != "" {
			return fulfillmentType
		}
	}
	for _, item := range order.Items {
		if fulfillmentType := strings.TrimSpace(item.FulfillmentType); fulfillmentType != "" {
			return fulfillmentType
		}
	}
	for _, child := range order.Children {
		if child.Fulfillment != nil {
			if fulfillmentType := strings.TrimSpace(child.Fulfillment.Type); fulfillmentType != "" {
				return fulfillmentType
			}
		}
		for _, item := range child.Items {
			if fulfillmentType := strings.TrimSpace(item.FulfillmentType); fulfillmentType != "" {
				return fulfillmentType
			}
		}
	}
	return ""
}
