package middleware

import (
	"net/http"
	"strings"

	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
	"github.com/dujiao-next/internal/platform/http/response"

	"github.com/dujiao-next/internal/constants"
	paymenttransport "github.com/dujiao-next/internal/modules/payment/transport/http"
	paymentcallbacktransport "github.com/dujiao-next/internal/modules/payment/transport/http/callback"
	upstreamtransport "github.com/dujiao-next/internal/modules/upstreamapi/transport/http"

	"github.com/gin-gonic/gin"
)

// defaultCallbackPaths 默认回调路径集合，用于在配置自定义路由后屏蔽默认路径
var defaultCallbackPaths = map[string]bool{
	constants.DefaultPaymentCallbackPath:  true,
	constants.DefaultDujiaoPayWebhookPath: true,
	constants.DefaultPaypalWebhookPath:    true,
	constants.DefaultStripeWebhookPath:    true,
	constants.DefaultUpstreamCallbackPath: true,
}

// CallbackRouteMiddleware 动态回调路由中间件。
// 当管理员配置了自定义回调路由时：
//   - 匹配自定义路径 → 分发到对应 handler
//   - 匹配默认回调路径 → 返回 404（隐藏默认路径）
//   - 未配置自定义路由 → 放行，默认路由正常工作
type callbackRouteSettings interface {
	GetCallbackRoutesCached() *settingsintegration.CallbackRoutesSetting
}

func CallbackRouteMiddleware(
	settingService callbackRouteSettings,
	callbackHandler *paymentcallbacktransport.Handler,
	webhookHandler *paymenttransport.WebhookHandler,
	upstreamHandler *upstreamtransport.Handler,
	rateLimit gin.HandlerFunc,
) gin.HandlerFunc {
	if rateLimit == nil {
		panic("callback rate guard is required")
	}
	bodyLimit := BodyLimit(4 << 20)
	slots := make(chan struct{}, 16)
	return func(c *gin.Context) {
		path := strings.TrimRight(c.Request.URL.Path, "/")
		method := c.Request.Method

		// 快速过滤：只处理以 /api/ 开头的请求
		if !strings.HasPrefix(path, "/api/") {
			c.Next()
			return
		}

		// 快速过滤：回调只用 POST 和 GET
		if method != http.MethodPost && method != http.MethodGet {
			c.Next()
			return
		}

		routes := settingService.GetCallbackRoutesCached()
		if routes == nil {
			// 未配置自定义路由，放行使用默认路由
			c.Next()
			return
		}

		dispatch := func(handler gin.HandlerFunc) {
			select {
			case slots <- struct{}{}:
				defer func() { <-slots }()
			default:
				c.Header("Retry-After", "1")
				response.ErrorWithHTTPStatus(c, 503, response.CodeInternal, "Service busy; please retry later")
				c.Abort()
				return
			}
			rateLimit(c)
			if c.IsAborted() {
				return
			}
			bodyLimit(c)
			if c.IsAborted() {
				return
			}
			handler(c)
			c.Abort()
		}

		// 匹配自定义回调路径
		switch path {
		case routes.PaymentCallback:
			if routes.PaymentCallback != "" && (method == http.MethodPost || method == http.MethodGet) {
				dispatch(callbackHandler.PaymentCallback)
				return
			}
		case routes.DujiaoPayWebhook:
			if routes.DujiaoPayWebhook != "" && method == http.MethodPost && webhookHandler != nil {
				dispatch(webhookHandler.DujiaoPayWebhook)
				return
			}
		case routes.PaypalWebhook:
			if routes.PaypalWebhook != "" && method == http.MethodPost && webhookHandler != nil {
				dispatch(webhookHandler.PaypalWebhook)
				return
			}
		case routes.StripeWebhook:
			if routes.StripeWebhook != "" && method == http.MethodPost && webhookHandler != nil {
				dispatch(webhookHandler.StripeWebhook)
				return
			}
		case routes.UpstreamCallback:
			if routes.UpstreamCallback != "" && method == http.MethodPost {
				dispatch(upstreamHandler.HandleCallback)
				return
			}
		}

		// 屏蔽默认回调路径（当对应的自定义路由已配置时）
		if defaultCallbackPaths[path] {
			shouldBlock := false
			switch path {
			case constants.DefaultPaymentCallbackPath:
				shouldBlock = routes.PaymentCallback != ""
			case constants.DefaultDujiaoPayWebhookPath:
				shouldBlock = routes.DujiaoPayWebhook != ""
			case constants.DefaultPaypalWebhookPath:
				shouldBlock = routes.PaypalWebhook != ""
			case constants.DefaultStripeWebhookPath:
				shouldBlock = routes.StripeWebhook != ""
			case constants.DefaultUpstreamCallbackPath:
				shouldBlock = routes.UpstreamCallback != ""
			}
			if shouldBlock {
				c.AbortWithStatus(http.StatusNotFound)
				return
			}
		}

		c.Next()
	}
}
