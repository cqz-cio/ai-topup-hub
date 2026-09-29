package paymenthttp

import (
	"context"
	"io"
	"net/http"
	"strings"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

const maxWebhookBodyBytes = 1 << 20

// WebhookCallbackInput webhook 回调输入。
type WebhookCallbackInput struct {
	ChannelID uint
	Headers   map[string]string
	Body      []byte
	Context   context.Context
}

// PaymentWebhookService webhook 处理端口。
type PaymentWebhookService interface {
	HandlePaypalWebhook(input WebhookCallbackInput) (*paymentdomain.Payment, string, error)
	HandleStripeWebhook(input WebhookCallbackInput) (*paymentdomain.Payment, string, error)
	HandleDujiaoPayWebhook(input WebhookCallbackInput) (*paymentdomain.Payment, string, error)
}

// ExceptionAlerter 支付异常告警入队端口。
type ExceptionAlerter interface {
	EnqueuePaymentExceptionAlert(method, path, clientIP string, data jsonmap.JSON) error
}

// PaypalWebhookQuery PayPal webhook 查询参数。
type PaypalWebhookQuery struct {
	ChannelID uint `form:"channel_id" binding:"required"`
}

// StripeWebhookQuery Stripe webhook 查询参数。
type StripeWebhookQuery struct {
	ChannelID uint `form:"channel_id"`
}

// DujiaoPayWebhookQuery DujiaoPay webhook 查询参数。
type DujiaoPayWebhookQuery struct {
	ChannelID uint `form:"channel_id"`
}

// WebhookHandler 处理支付 webhook HTTP。
type WebhookHandler struct {
	webhooks PaymentWebhookService
	alerts   ExceptionAlerter
}

func NewWebhookHandler(webhooks PaymentWebhookService, alerts ExceptionAlerter) *WebhookHandler {
	if webhooks == nil {
		panic("payment webhook handler: webhooks is nil")
	}
	return &WebhookHandler{webhooks: webhooks, alerts: alerts}
}

// PaypalWebhook PayPal webhook 回调。
func (h *WebhookHandler) PaypalWebhook(c *gin.Context) {
	log := ginutil.RequestLog(c)
	var query PaypalWebhookQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		log.Warnw("paypal_webhook_query_invalid", "error", err)
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	body, err := readWebhookBody(c)
	if err != nil {
		log.Warnw("paypal_webhook_body_read_failed", "channel_id", query.ChannelID, "error", err)
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	log.Infow("paypal_webhook_received",
		"channel_id", query.ChannelID,
		"client_ip", c.ClientIP(),
		"body_size", len(body),
		"paypal_transmission_id", strings.TrimSpace(c.GetHeader("Paypal-Transmission-Id")),
		"paypal_transmission_time", strings.TrimSpace(c.GetHeader("Paypal-Transmission-Time")),
		"paypal_auth_algo", strings.TrimSpace(c.GetHeader("Paypal-Auth-Algo")),
	)
	payment, eventType, err := h.webhooks.HandlePaypalWebhook(WebhookCallbackInput{
		ChannelID: query.ChannelID,
		Headers:   collectRequestHeaders(c),
		Body:      body,
		Context:   c.Request.Context(),
	})
	if err != nil {
		log.Warnw("paypal_webhook_handle_failed",
			"channel_id", query.ChannelID,
			"event_type", eventType,
			"error", err,
		)
		h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
			"alert_type":  "paypal_webhook_handle_failed",
			"alert_level": "error",
			"message":     strings.TrimSpace(err.Error()),
			"provider":    constants.PaymentChannelTypePaypal,
		})
		respondPaymentCallbackError(c, err)
		return
	}
	respondWebhookSuccess(c, log, "paypal_webhook", query.ChannelID, eventType, payment)
}

// StripeWebhook Stripe webhook 回调。
func (h *WebhookHandler) StripeWebhook(c *gin.Context) {
	log := ginutil.RequestLog(c)
	var query StripeWebhookQuery
	_ = c.ShouldBindQuery(&query)

	body, err := readWebhookBody(c)
	if err != nil {
		log.Warnw("stripe_webhook_body_read_failed", "channel_id", query.ChannelID, "error", err)
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	log.Infow("stripe_webhook_received",
		"channel_id", query.ChannelID,
		"client_ip", c.ClientIP(),
		"body_size", len(body),
	)

	payment, eventType, err := h.webhooks.HandleStripeWebhook(WebhookCallbackInput{
		ChannelID: query.ChannelID,
		Headers:   collectRequestHeaders(c),
		Body:      body,
		Context:   c.Request.Context(),
	})
	if err != nil {
		log.Warnw("stripe_webhook_handle_failed",
			"channel_id", query.ChannelID,
			"event_type", eventType,
			"error", err,
		)
		h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
			"alert_type":  "stripe_webhook_handle_failed",
			"alert_level": "error",
			"message":     strings.TrimSpace(err.Error()),
			"provider":    constants.PaymentChannelTypeStripe,
		})
		respondPaymentCallbackError(c, err)
		return
	}
	respondWebhookSuccess(c, log, "stripe_webhook", query.ChannelID, eventType, payment)
}

// DujiaoPayWebhook DujiaoPay webhook 回调。
func (h *WebhookHandler) DujiaoPayWebhook(c *gin.Context) {
	log := ginutil.RequestLog(c)
	var query DujiaoPayWebhookQuery
	_ = c.ShouldBindQuery(&query)

	body, err := readWebhookBody(c)
	if err != nil {
		log.Warnw("dujiaopay_webhook_body_read_failed", "channel_id", query.ChannelID, "error", err)
		ginutil.RespondError(c, response.CodeBadRequest, "error.bad_request", err)
		return
	}
	log.Infow("dujiaopay_webhook_received",
		"channel_id", query.ChannelID,
		"client_ip", c.ClientIP(),
		"body_size", len(body),
		"dujiaopay_webhook_id", strings.TrimSpace(c.GetHeader("DJP-Webhook-ID")),
		"dujiaopay_webhook_timestamp", strings.TrimSpace(c.GetHeader("DJP-Webhook-Timestamp")),
	)

	payment, eventType, err := h.webhooks.HandleDujiaoPayWebhook(WebhookCallbackInput{
		ChannelID: query.ChannelID,
		Headers:   collectRequestHeaders(c),
		Body:      body,
		Context:   c.Request.Context(),
	})
	if err != nil {
		log.Warnw("dujiaopay_webhook_handle_failed",
			"channel_id", query.ChannelID,
			"event_type", eventType,
			"error", err,
		)
		h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
			"alert_type":  "dujiaopay_webhook_handle_failed",
			"alert_level": "error",
			"message":     strings.TrimSpace(err.Error()),
			"provider":    constants.PaymentProviderDujiaoPay,
		})
		respondPaymentCallbackError(c, err)
		return
	}
	respondWebhookSuccess(c, log, "dujiaopay_webhook", query.ChannelID, eventType, payment)
}

func readWebhookBody(c *gin.Context) ([]byte, error) {
	if c == nil || c.Request == nil || c.Request.Body == nil {
		return nil, nil
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxWebhookBodyBytes)
	return io.ReadAll(c.Request.Body)
}

func collectRequestHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string)
	for key, values := range c.Request.Header {
		if len(values) == 0 {
			continue
		}
		headers[key] = values[0]
	}
	return headers
}

type webhookLogger interface {
	Infow(msg string, keysAndValues ...interface{})
}

func respondWebhookSuccess(c *gin.Context, log webhookLogger, prefix string, channelID uint, eventType string, payment *paymentdomain.Payment) {
	if payment == nil {
		log.Infow(prefix+"_accepted_no_payment",
			"channel_id", channelID,
			"event_type", eventType,
		)
		response.Success(c, gin.H{
			"accepted":   true,
			"event_type": eventType,
			"updated":    false,
		})
		return
	}
	log.Infow(prefix+"_processed",
		"channel_id", channelID,
		"event_type", eventType,
		"payment_id", payment.ID,
		"status", payment.Status,
	)
	response.Success(c, gin.H{
		"accepted":   true,
		"event_type": eventType,
		"updated":    true,
		"payment_id": payment.ID,
		"status":     payment.Status,
	})
}

func (h *WebhookHandler) enqueuePaymentExceptionAlert(c *gin.Context, data jsonmap.JSON) {
	if h == nil || h.alerts == nil || c == nil || c.Request == nil {
		return
	}
	path := ""
	if c.Request.URL != nil {
		path = strings.TrimSpace(c.Request.URL.Path)
	}
	if err := h.alerts.EnqueuePaymentExceptionAlert(
		strings.TrimSpace(c.Request.Method),
		path,
		strings.TrimSpace(c.ClientIP()),
		data,
	); err != nil {
		ginutil.RequestLog(c).Warnw("enqueue_payment_exception_alert_failed", "error", err)
	}
}

func respondPaymentCallbackError(c *gin.Context, err error) {
	respondWithMappedError(c, err, paymentCallbackErrorRules, response.CodeInternal, "error.payment_callback_failed")
}

var paymentCallbackErrorRules = concatMappedErrors(
	[]mappedError{
		{target: ErrPaymentInvalid, code: response.CodeBadRequest, key: "error.payment_invalid"},
		{target: ErrPaymentNotFound, code: response.CodeNotFound, key: "error.payment_not_found"},
		{target: ErrPaymentStatusInvalid, code: response.CodeBadRequest, key: "error.payment_status_invalid"},
		{target: ErrPaymentAmountMismatch, code: response.CodeBadRequest, key: "error.payment_amount_mismatch"},
		{target: ErrPaymentCurrencyMismatch, code: response.CodeBadRequest, key: "error.payment_currency_mismatch"},
		{target: ErrPaymentChannelNotFound, code: response.CodeNotFound, key: "error.payment_channel_not_found"},
	},
	paymentProviderGatewayErrorRules,
)
