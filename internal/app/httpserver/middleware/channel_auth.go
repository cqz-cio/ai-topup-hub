package middleware

import (
	"io"
	"net/http"
	"time"

	"github.com/dujiao-next/internal/app/container"
	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/logger"
	channelclientapp "github.com/dujiao-next/internal/modules/channelclient/application"
	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/dujiao-next/internal/upstream"

	"github.com/gin-gonic/gin"
)

const (
	channelClientIDKey = "channel_client_id"
	channelKeyCtxKey   = "channel_key"
	channelTypeCtxKey  = "channel_type"

	channelHeaderKey       = "Dujiao-Next-Channel-Key"
	channelHeaderTimestamp = "Dujiao-Next-Channel-Timestamp"
	channelHeaderSignature = "Dujiao-Next-Channel-Signature"
)

// ChannelAPIAuthMiddleware 渠道 API 签名鉴权中间件
func ChannelAPIAuthMiddleware(container *container.Container) gin.HandlerFunc {
	return func(c *gin.Context) {
		channelKey := c.GetHeader(channelHeaderKey)
		timestampStr := c.GetHeader(channelHeaderTimestamp)
		signature := c.GetHeader(channelHeaderSignature)

		if channelKey == "" || timestampStr == "" || signature == "" {
			response.ChannelError(c, http.StatusUnauthorized, response.CodeUnauthorized, i18n.T(i18n.ResolveLocale(c), "error.unauthorized"), "channel_client_unauthorized")
			c.Abort()
			return
		}

		timestamp, err := upstream.ParseTimestamp(timestampStr)
		if err != nil {
			response.ChannelError(c, http.StatusUnauthorized, response.CodeUnauthorized, i18n.T(i18n.ResolveLocale(c), "error.unauthorized"), "channel_client_unauthorized")
			c.Abort()
			return
		}

		// 读取 body 用于签名验证（限制最大 10MB 防止内存耗尽）
		var body []byte
		if c.Request.Body != nil {
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)
			body, err = io.ReadAll(c.Request.Body)
			if err != nil {
				response.ChannelError(c, http.StatusBadRequest, response.CodeBadRequest, i18n.T(i18n.ResolveLocale(c), "error.bad_request"), "validation_error")
				c.Abort()
				return
			}
			// 重置 body 供后续 handler 读取
			c.Request.Body = io.NopCloser(&bodyReader{data: body})
		}

		method := c.Request.Method
		path := c.Request.URL.Path

		client, err := container.ChannelClientService.VerifyChannelSignature(
			channelKey, signature, timestamp, method, path, body,
		)
		if err != nil {
			switch err {
			case channelclientapp.ErrTimestampExpired:
				response.ChannelError(c, http.StatusUnauthorized, response.CodeUnauthorized, i18n.T(i18n.ResolveLocale(c), "error.unauthorized"), "channel_client_unauthorized")
			case channelclientapp.ErrNotFound:
				response.ChannelError(c, http.StatusUnauthorized, response.CodeUnauthorized, i18n.T(i18n.ResolveLocale(c), "error.unauthorized"), "channel_client_unauthorized")
			case channelclientapp.ErrDisabled:
				response.ChannelError(c, http.StatusForbidden, response.CodeForbidden, i18n.T(i18n.ResolveLocale(c), "error.forbidden"), "channel_client_disabled")
			case channelclientapp.ErrSignatureInvalid:
				response.ChannelError(c, http.StatusUnauthorized, response.CodeUnauthorized, i18n.T(i18n.ResolveLocale(c), "error.unauthorized"), "channel_client_unauthorized")
			default:
				logger.Errorw("channel_auth_error", "error", err)
				response.ChannelError(c, http.StatusInternalServerError, response.CodeInternal, i18n.T(i18n.ResolveLocale(c), "error.internal_error"), "internal_error")
			}
			c.Abort()
			return
		}

		// 异步更新 last_used_at
		now := time.Now()
		go func() {
			if updateErr := container.ChannelClientService.MarkUsed(client.ID, now); updateErr != nil {
				logger.Warnw("channel_auth_update_last_used_failed", "error", updateErr)
			}
		}()

		// 设置 context keys
		c.Set(channelClientIDKey, client.ID)
		c.Set(channelKeyCtxKey, client.ChannelKey)
		c.Set(channelTypeCtxKey, client.ChannelType)

		c.Next()
	}
}
