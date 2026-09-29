package paymentcallbackhttp

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

var errAlipayCallbackPaymentNotFound = errors.New("alipay callback payment not found")

func (h *Handler) handleAlipayCallback(c *gin.Context) bool {
	log := ginutil.RequestLog(c)
	form, err := parseCallbackForm(c)
	if err != nil {
		log.Warnw("alipay_callback_form_parse_failed", "error", err)
		return false
	}
	if !isAlipayCallbackForm(form) {
		log.Debugw("alipay_callback_not_matched")
		return false
	}
	log.Infow("alipay_callback_received",
		"client_ip", c.ClientIP(),
		"out_trade_no", strings.TrimSpace(getFirstValue(form, "out_trade_no")),
		"trade_no", strings.TrimSpace(getFirstValue(form, "trade_no")),
		"trade_status", strings.TrimSpace(getFirstValue(form, "trade_status")),
	)

	payment, channel, err := h.findAlipayCallbackPayment(form)
	if err != nil || payment == nil || channel == nil {
		log.Warnw("alipay_callback_payment_not_found",
			"out_trade_no", strings.TrimSpace(getFirstValue(form, "out_trade_no")),
			"trade_no", strings.TrimSpace(getFirstValue(form, "trade_no")),
			"error", err,
		)
		c.String(http.StatusOK, constants.AlipayCallbackFail)
		return true
	}

	updated, err := h.service.HandleSyncCallback(channel, form, nil)
	if err != nil {
		log.Warnw("alipay_callback_handle_failed", "payment_id", payment.ID, "channel_id", channel.ID, "error", err)
		h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
			"alert_type": "alipay_callback_handle_failed", "alert_level": "error",
			"payment_id":   fmt.Sprintf("%d", payment.ID),
			"out_trade_no": strings.TrimSpace(getFirstValue(form, "out_trade_no")),
			"message":      strings.TrimSpace(err.Error()), "provider": constants.PaymentChannelTypeAlipay,
		})
		c.String(http.StatusOK, constants.AlipayCallbackFail)
		return true
	}
	log.Infow("alipay_callback_processed", "payment_id", payment.ID, "channel_id", channel.ID,
		"out_trade_no", strings.TrimSpace(getFirstValue(form, "out_trade_no")),
		"trade_no", strings.TrimSpace(getFirstValue(form, "trade_no")), "status", updated.Status)
	c.String(http.StatusOK, constants.AlipayCallbackSuccess)
	return true
}

func (h *Handler) handleEpayCallback(c *gin.Context) bool {
	log := ginutil.RequestLog(c)
	form, err := parseCallbackForm(c)
	if err != nil {
		log.Warnw("epay_callback_form_parse_failed", "error", err)
		return false
	}
	outTradeNo := strings.TrimSpace(getFirstValue(form, "out_trade_no"))
	pid := strings.TrimSpace(getFirstValue(form, "pid"))
	if pid == "" || outTradeNo == "" {
		log.Debugw("epay_callback_not_matched", "reason", "missing_pid_or_out_trade_no")
		return false
	}
	if strings.TrimSpace(getFirstValue(form, "trade_status")) == "" {
		log.Debugw("epay_callback_not_matched", "reason", "missing_trade_status")
		return false
	}
	log.Infow("epay_callback_received", "client_ip", c.ClientIP(), "out_trade_no", outTradeNo,
		"trade_no", strings.TrimSpace(getFirstValue(form, "trade_no")),
		"trade_status", strings.TrimSpace(getFirstValue(form, "trade_status")))

	payment, err := h.payments.GetByGatewayOrderNo(outTradeNo)
	if err != nil || payment == nil {
		log.Warnw("epay_callback_payment_not_found", "out_trade_no", outTradeNo, "error", err)
		c.String(http.StatusOK, constants.EpayCallbackFail)
		return true
	}
	channel, err := h.channels.GetByID(payment.ChannelID)
	if err != nil || channel == nil {
		log.Warnw("epay_callback_channel_not_found", "payment_id", payment.ID, "channel_id", payment.ChannelID, "error", err)
		c.String(http.StatusOK, constants.EpayCallbackFail)
		return true
	}
	if strings.ToLower(strings.TrimSpace(channel.ProviderType)) != constants.PaymentProviderEpay {
		log.Warnw("epay_callback_provider_invalid", "payment_id", payment.ID, "channel_id", channel.ID, "provider_type", channel.ProviderType)
		c.String(http.StatusOK, constants.EpayCallbackFail)
		return true
	}

	updated, err := h.service.HandleSyncCallback(channel, form, nil)
	if err != nil {
		log.Warnw("epay_callback_handle_failed", "payment_id", payment.ID, "channel_id", channel.ID, "out_trade_no", outTradeNo, "error", err)
		h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
			"alert_type": "epay_callback_handle_failed", "alert_level": "error",
			"payment_id": fmt.Sprintf("%d", payment.ID), "out_trade_no": outTradeNo,
			"message": strings.TrimSpace(err.Error()), "provider": constants.PaymentProviderEpay,
		})
		c.String(http.StatusOK, constants.EpayCallbackFail)
		return true
	}
	log.Infow("epay_callback_processed", "payment_id", payment.ID, "channel_id", channel.ID, "out_trade_no", outTradeNo, "status", updated.Status)
	c.String(http.StatusOK, constants.EpayCallbackSuccess)
	return true
}

func parseCallbackForm(c *gin.Context) (map[string][]string, error) {
	normalizeCallbackRequestQuery(c.Request)
	if err := c.Request.ParseForm(); err != nil {
		return nil, err
	}
	if len(c.Request.PostForm) > 0 {
		return c.Request.PostForm, nil
	}
	return c.Request.Form, nil
}

func normalizeCallbackRequestQuery(r *http.Request) {
	if r == nil || r.URL == nil || r.URL.RawQuery == "" {
		return
	}
	raw := r.URL.RawQuery
	normalized := strings.ReplaceAll(strings.ReplaceAll(raw, "&amp;", "&"), ";", "&")
	if normalized != raw {
		r.URL.RawQuery = normalized
	}
}

func isAlipayCallbackForm(form map[string][]string) bool {
	if strings.TrimSpace(getFirstValue(form, "sign")) == "" {
		return false
	}
	hasNotifyField := strings.TrimSpace(getFirstValue(form, "notify_id")) != "" ||
		strings.TrimSpace(getFirstValue(form, "notify_type")) != "" ||
		strings.TrimSpace(getFirstValue(form, "buyer_id")) != ""
	return hasNotifyField && (strings.TrimSpace(getFirstValue(form, "out_trade_no")) != "" ||
		strings.TrimSpace(getFirstValue(form, "trade_no")) != "")
}

func (h *Handler) findAlipayCallbackPayment(form map[string][]string) (*paymentdomain.Payment, *paymentdomain.PaymentChannel, error) {
	for _, candidate := range []struct {
		value string
		find  func(string) (*paymentdomain.Payment, error)
	}{
		{strings.TrimSpace(getFirstValue(form, "out_trade_no")), h.payments.GetByGatewayOrderNo},
		{strings.TrimSpace(getFirstValue(form, "trade_no")), h.payments.GetLatestByProviderRef},
	} {
		if candidate.value == "" {
			continue
		}
		payment, err := candidate.find(candidate.value)
		if err != nil || payment == nil {
			continue
		}
		channel, err := h.channels.GetByID(payment.ChannelID)
		if err == nil && channel != nil &&
			strings.EqualFold(strings.TrimSpace(channel.ProviderType), constants.PaymentProviderOfficial) &&
			strings.EqualFold(strings.TrimSpace(channel.ChannelType), constants.PaymentChannelTypeAlipay) {
			return payment, channel, nil
		}
	}
	return nil, nil, errAlipayCallbackPaymentNotFound
}
