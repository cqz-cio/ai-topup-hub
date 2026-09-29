package middleware

import (
	"net/http"
	"strings"

	settingsapp "github.com/dujiao-next/internal/modules/settings/application"

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
func CallbackRouteMiddleware(
	settingService *settingsapp.Service,
	callbackHandler *paymentcallbacktransport.Handler,
	webhookHandler *paymenttransport.WebhookHandler,
	upstreamHandler *upstreamtransport.Handler,
) gin.HandlerFunc {
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

		// 匹配自定义回调路径
		switch path {
		case routes.PaymentCallback:
			if routes.PaymentCallback != "" && (method == http.MethodPost || method == http.MethodGet) {
				callbackHandler.PaymentCallback(c)
				c.Abort()
				return
			}
		case routes.DujiaoPayWebhook:
			if routes.DujiaoPayWebhook != "" && method == http.MethodPost && webhookHandler != nil {
				webhookHandler.DujiaoPayWebhook(c)
				c.Abort()
				return
			}
		case routes.PaypalWebhook:
			if routes.PaypalWebhook != "" && method == http.MethodPost && webhookHandler != nil {
				webhookHandler.PaypalWebhook(c)
				c.Abort()
				return
			}
		case routes.StripeWebhook:
			if routes.StripeWebhook != "" && method == http.MethodPost && webhookHandler != nil {
				webhookHandler.StripeWebhook(c)
				c.Abort()
				return
			}
		case routes.UpstreamCallback:
			if routes.UpstreamCallback != "" && method == http.MethodPost {
				upstreamHandler.HandleCallback(c)
				c.Abort()
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
