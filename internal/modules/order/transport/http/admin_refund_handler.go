package orderhttp

import (
	"errors"
	"strings"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	walletdomain "github.com/dujiao-next/internal/modules/wallet/domain"

	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
)

var (
	ErrOrderFetchFailed           = errors.New("order fetch failed")
	ErrOrderRefundExpired         = errors.New("order refund expired")
	ErrWalletInvalidAmount        = errors.New("wallet invalid amount")
	ErrWalletRefundExceeded       = errors.New("wallet refund exceeded")
	ErrWalletNotSupportedForGuest = errors.New("wallet not supported for guest")
)

// AdminRefundListQuery 管理端退款列表查询。
type AdminRefundListQuery struct {
	Page           int
	PageSize       int
	UserID         string
	UserKeyword    string
	OrderNo        string
	GuestEmail     string
	ProductKeyword string
	ProductName    string
	CreatedFrom    string
	CreatedTo      string
}

// AdminRefundItem 管理端退款列表/详情返回项。
type AdminRefundItem struct {
	orderdomain.OrderRefundRecord
	OrderNo         string                  `json:"order_no,omitempty"`
	GuestLocale     string                  `json:"guest_locale,omitempty"`
	Items           []orderdomain.OrderItem `json:"items,omitempty"`
	UserEmail       string                  `json:"user_email,omitempty"`
	UserDisplayName string                  `json:"user_display_name,omitempty"`
	RefundTypeLabel string                  `json:"refund_type_label"`
}

// AdminRefundToWalletInput 管理端退款到余额输入。
type AdminRefundToWalletInput struct {
	OrderID uint
	Amount  money.Amount
	Remark  string
}

// AdminManualRefundInput 管理端手动退款输入。
type AdminManualRefundInput struct {
	OrderID            uint
	Amount             money.Amount
	Remark             string
	PaymentFeeRefunded bool
}

type AdminUpdateRefundPaymentFeeInput struct {
	RefundRecordID     uint
	PaymentFeeRefunded bool
}

// AdminRefundReader 管理端退款只读端口。
type AdminRefundReader interface {
	ListAdminRefundItems(query AdminRefundListQuery) ([]AdminRefundItem, int64, error)
	GetAdminRefundItem(id uint) (*AdminRefundItem, error)
}

// AdminRefundWriter 管理端退款写端口（金额解析 + 手动退款）。
type AdminRefundWriter interface {
	ParseRefundAmount(raw string) (money.Amount, error)
	AdminManualRefund(input AdminManualRefundInput) (*orderdomain.Order, *orderdomain.OrderRefundRecord, error)
	UpdatePaymentFeeRefunded(input AdminUpdateRefundPaymentFeeInput) (*orderdomain.OrderRefundRecord, error)
}

// AdminWalletRefunder 管理端退款到余额端口。
type AdminWalletRefunder interface {
	AdminRefundToWallet(input AdminRefundToWalletInput) (*orderdomain.Order, *walletdomain.Transaction, *orderdomain.OrderRefundRecord, error)
}

// OrderByIDLookup 按 ID 查询订单（退款邮件优先父订单）。
type OrderByIDLookup interface {
	GetByID(id uint) (*orderdomain.Order, error)
}

// OrderStatusEmailEnqueuer 订单状态邮件入队端口。
type OrderStatusEmailEnqueuer interface {
	EnqueueOrderStatusEmail(orderID uint, status string, refundRecordID uint) error
}

// AdminRefundHandler 处理后台退款 HTTP（只读 + 写退款）。
type AdminRefundHandler struct {
	refunds AdminRefundReader
	writes  AdminRefundWriter
	wallet  AdminWalletRefunder
	orders  OrderByIDLookup
	emails  OrderStatusEmailEnqueuer
}

func NewAdminRefundHandler(
	refunds AdminRefundReader,
	writes AdminRefundWriter,
	wallet AdminWalletRefunder,
	orders OrderByIDLookup,
	emails OrderStatusEmailEnqueuer,
) *AdminRefundHandler {
	if refunds == nil {
		panic("order admin refund handler: refunds is nil")
	}
	return &AdminRefundHandler{
		refunds: refunds,
		writes:  writes,
		wallet:  wallet,
		orders:  orders,
		emails:  emails,
	}
}

// AdminRefundOrderToWalletRequest 管理端订单退款到余额请求
type AdminRefundOrderToWalletRequest struct {
	Amount string `json:"amount" binding:"required"`
	Remark string `json:"remark"`
}

// AdminManualRefundOrderRequest 管理端手动退款请求（不处理钱包/支付渠道）
type AdminManualRefundOrderRequest struct {
	Amount             string `json:"amount" binding:"required"`
	Remark             string `json:"remark"`
	PaymentFeeRefunded bool   `json:"payment_fee_refunded"`
}

type AdminUpdateRefundPaymentFeeRequest struct {
	PaymentFeeRefunded *bool `json:"payment_fee_refunded" binding:"required"`
}

// GetAdminOrderRefunds 获取管理端退款记录列表
func (h *AdminRefundHandler) GetAdminOrderRefunds(c *gin.Context) {
	page, pageSize := ginutil.ParsePagination(c)

	items, total, err := h.refunds.ListAdminRefundItems(AdminRefundListQuery{
		Page:           page,
		PageSize:       pageSize,
		UserID:         c.Query("user_id"),
		UserKeyword:    c.Query("user_keyword"),
		OrderNo:        c.Query("order_no"),
		GuestEmail:     c.Query("guest_email"),
		ProductKeyword: c.Query("product_keyword"),
		ProductName:    c.Query("product_name"),
		CreatedFrom:    c.Query("created_from"),
		CreatedTo:      c.Query("created_to"),
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderFetchFailed):
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		default:
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		}
		return
	}

	pagination := response.BuildPagination(page, pageSize, total)
	response.SuccessWithPage(c, items, pagination)
}

// GetAdminOrderRefund 获取管理端退款记录详情
func (h *AdminRefundHandler) GetAdminOrderRefund(c *gin.Context) {
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}

	item, err := h.refunds.GetAdminRefundItem(id)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		}
		return
	}
	response.Success(c, item)
}

// AdminRefundOrderToWallet 管理端订单退款到余额
func (h *AdminRefundHandler) AdminRefundOrderToWallet(c *gin.Context) {
	if h.writes == nil || h.wallet == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", nil)
		return
	}
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	var req AdminRefundOrderToWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := h.writes.ParseRefundAmount(req.Amount)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	order, txn, refundRecord, err := h.wallet.AdminRefundToWallet(AdminRefundToWalletInput{
		OrderID: orderID,
		Amount:  amount,
		Remark:  req.Remark,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		case errors.Is(err, ErrOrderStatusInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		case errors.Is(err, ErrOrderRefundExpired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_refund_expired", nil)
		case errors.Is(err, ErrWalletInvalidAmount), errors.Is(err, ErrWalletRefundExceeded), errors.Is(err, ErrWalletNotSupportedForGuest):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
		}
		return
	}
	h.enqueueOrderRefundStatusEmail(order, refundRecord)

	response.Success(c, gin.H{
		"order":       order,
		"transaction": txn,
	})
}

// AdminManualRefundOrder 管理端手动退款（不处理钱包/支付渠道）
func (h *AdminRefundHandler) AdminManualRefundOrder(c *gin.Context) {
	if h.writes == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", nil)
		return
	}
	orderID, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.order_item_invalid", nil)
		return
	}
	var req AdminManualRefundOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	amount, err := h.writes.ParseRefundAmount(req.Amount)
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	order, refundRecord, err := h.writes.AdminManualRefund(AdminManualRefundInput{
		OrderID:            orderID,
		Amount:             amount,
		Remark:             req.Remark,
		PaymentFeeRefunded: req.PaymentFeeRefunded,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		case errors.Is(err, ErrOrderStatusInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		case errors.Is(err, ErrOrderRefundExpired):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_refund_expired", nil)
		case errors.Is(err, ErrWalletInvalidAmount), errors.Is(err, ErrWalletRefundExceeded):
			ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
		}
		return
	}
	h.enqueueOrderRefundStatusEmail(order, refundRecord)

	response.Success(c, gin.H{
		"order":         order,
		"refund_record": refundRecord,
	})
}

// UpdateAdminOrderRefundPaymentFee corrects whether a recorded manual refund
// also returned its original payment fee.
func (h *AdminRefundHandler) UpdateAdminOrderRefundPaymentFee(c *gin.Context) {
	if h.writes == nil {
		ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", nil)
		return
	}
	id, err := ginutil.ParseParamUint(c, "id")
	if err != nil {
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", nil)
		return
	}
	var req AdminUpdateRefundPaymentFeeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		ginutil.RespondBindError(c, err)
		return
	}
	_, err = h.writes.UpdatePaymentFeeRefunded(AdminUpdateRefundPaymentFeeInput{
		RefundRecordID:     id,
		PaymentFeeRefunded: *req.PaymentFeeRefunded,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		case errors.Is(err, ErrOrderStatusInvalid):
			ginutil.RespondError(c, response.CodeBadRequest, "error.order_status_invalid", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_update_failed", err)
		}
		return
	}
	item, err := h.refunds.GetAdminRefundItem(id)
	if err != nil {
		switch {
		case errors.Is(err, ErrOrderNotFound):
			ginutil.RespondError(c, response.CodeNotFound, "error.order_not_found", nil)
		default:
			ginutil.RespondError(c, response.CodeInternal, "error.order_fetch_failed", err)
		}
		return
	}
	response.Success(c, item)
}

// enqueueOrderRefundStatusEmail 异步发送退款后的订单状态邮件（优先父订单维度）。
func (h *AdminRefundHandler) enqueueOrderRefundStatusEmail(order *orderdomain.Order, refundRecord *orderdomain.OrderRefundRecord) {
	if h == nil || order == nil || h.emails == nil {
		return
	}
	targetOrder := order
	if order.ParentID != nil && *order.ParentID > 0 && h.orders != nil {
		parentOrder, err := h.orders.GetByID(*order.ParentID)
		if err != nil {
			logger.Warnw("admin_order_refund_load_parent_failed",
				"order_id", order.ID,
				"parent_id", *order.ParentID,
				"error", err,
			)
		} else if parentOrder != nil {
			targetOrder = parentOrder
		}
	}
	if targetOrder.ID == 0 {
		return
	}
	status := strings.TrimSpace(targetOrder.Status)
	if status == "" {
		return
	}
	if err := h.emails.EnqueueOrderStatusEmail(targetOrder.ID, status, resolveRefundRecordID(refundRecord)); err != nil {
		logger.Warnw("admin_order_refund_enqueue_status_email_failed",
			"order_id", targetOrder.ID,
			"status", status,
			"error", err,
		)
	}
}

func resolveRefundRecordID(record *orderdomain.OrderRefundRecord) uint {
	if record == nil {
		return 0
	}
	return record.ID
}
