package paymentcallbackhttp

import (
	"context"
	"net/http"
	"strings"

	paymentdomain "github.com/dujiao-next/internal/modules/payment/domain"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

const maxCallbackBodyBytes = 1 << 20

// WechatWebhookInput is the transport-owned input for a WeChat callback.
type WechatWebhookInput struct {
	ChannelID uint
	Headers   map[string]string
	Body      []byte
	Context   context.Context
}

// Service is the application boundary used by synchronous payment callbacks.
type Service interface {
	HandleSyncCallback(channel *paymentdomain.PaymentChannel, form map[string][]string, body []byte) (*paymentdomain.Payment, error)
	HandleWechatWebhook(input WechatWebhookInput) (*paymentdomain.Payment, string, error)
}

// PaymentLookup contains only the payment reads required to locate callback targets.
type PaymentLookup interface {
	GetByGatewayOrderNo(gatewayOrderNo string) (*paymentdomain.Payment, error)
	GetLatestByProviderRef(providerRef string) (*paymentdomain.Payment, error)
}

// ChannelLookup contains only the channel read required to validate a callback provider.
type ChannelLookup interface {
	GetByID(id uint) (*paymentdomain.PaymentChannel, error)
}

// ExceptionAlerter queues operational callback alerts without coupling HTTP to notifications.
type ExceptionAlerter interface {
	EnqueuePaymentExceptionAlert(method, path, clientIP string, data jsonmap.JSON) error
}

// Handler dispatches the shared synchronous callback endpoint to its provider protocol.
type Handler struct {
	service  Service
	payments PaymentLookup
	channels ChannelLookup
	alerts   ExceptionAlerter
}

func NewHandler(service Service, payments PaymentLookup, channels ChannelLookup, alerts ExceptionAlerter) *Handler {
	if service == nil || payments == nil || channels == nil {
		panic("payment callback handler: required dependency is nil")
	}
	return &Handler{service: service, payments: payments, channels: channels, alerts: alerts}
}

// PaymentCallback preserves the historical provider detection order on the shared endpoint.
func (h *Handler) PaymentCallback(c *gin.Context) {
	if c.Request.Body != nil {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxCallbackBodyBytes)
	}
	ginutil.RequestLog(c).Infow("payment_callback_received",
		"method", c.Request.Method,
		"client_ip", c.ClientIP(),
		"content_type", strings.TrimSpace(c.GetHeader("Content-Type")),
	)
	for _, handle := range []func(*gin.Context) bool{
		h.handleWechatCallback,
		h.handleOkpayCallback,
		h.handleAlipayCallback,
		h.handleEpayCallback,
		h.handleTokenPayCallback,
		h.handleEpusdtCallback,
		h.handleBepusdtCallback,
	} {
		if handle(c) {
			return
		}
	}

	ginutil.RequestLog(c).Warnw("payment_callback_unrecognized",
		"method", c.Request.Method,
		"client_ip", c.ClientIP(),
		"content_type", strings.TrimSpace(c.GetHeader("Content-Type")),
	)
	h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
		"alert_type":  "callback_unrecognized",
		"alert_level": "warning",
		"message":     "支付回调请求无法匹配已支持的回调格式",
	})
	c.AbortWithStatus(http.StatusNotFound)
}

func (h *Handler) enqueuePaymentExceptionAlert(c *gin.Context, data jsonmap.JSON) {
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

func getFirstValue(form map[string][]string, key string) string {
	if values, ok := form[key]; ok && len(values) > 0 {
		return values[0]
	}
	return ""
}
