package paymentcallbackhttp

import (
	"github.com/dujiao-next/internal/constants"
	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"
	"github.com/gin-gonic/gin"
	"net/http"
	"strings"
)

func (h *Handler) handleXorpayCallback(c *gin.Context) bool {
	if c.Request.Method != http.MethodPost || c.ContentType() != "application/x-www-form-urlencoded" {
		return false
	}
	form, err := parseCallbackForm(c)
	if err != nil {
		return false
	}
	if getFirstValue(form, "aoid") == "" || getFirstValue(form, "pay_price") == "" {
		return false
	}
	// XorPay treats every 200 response as success, including a 'fail' body.
	// Only acknowledge after the verified callback transaction has committed.
	orderNo := getFirstValue(form, "order_id")
	if orderNo == "" || len(form["order_id"]) != 1 {
		c.String(http.StatusBadRequest, "invalid")
		return true
	}
	payment, err := h.payments.GetByGatewayOrderNo(orderNo)
	if err != nil {
		c.String(http.StatusInternalServerError, "retry")
		return true
	}
	if payment == nil {
		c.String(http.StatusNotFound, "not found")
		return true
	}
	channel, err := h.channels.GetByID(payment.ChannelID)
	if err != nil {
		c.String(http.StatusInternalServerError, "retry")
		return true
	}
	if channel == nil || !strings.EqualFold(channel.ProviderType, constants.PaymentProviderXorpay) {
		c.String(http.StatusBadRequest, "invalid")
		return true
	}
	if _, err = h.service.HandleSyncCallback(channel, form, nil); err != nil {
		ginutil.RequestLog(c).Warnw("xorpay_callback_rejected", "payment_id", payment.ID, "error", err)
		c.String(http.StatusInternalServerError, "retry")
		return true
	}
	c.String(http.StatusOK, "ok")
	return true
}
