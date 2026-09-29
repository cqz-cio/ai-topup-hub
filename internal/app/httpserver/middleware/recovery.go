package middleware

import (
	"errors"
	"fmt"
	"net/http"
	"runtime/debug"
	"syscall"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/logger"
	"github.com/dujiao-next/internal/platform/http/response"
)

// RecoveryMiddleware 捕获 panic,结构化记录日志并返回 i18n 的 500 错误体,
// 确保前端收到统一的响应格式;同时把 request_id / method / path / stack 等
// 字段以 JSON 形式写入 zap 日志,运维可通过 grep / Loki / ELK 等做告警。
//
// 注意:对于客户端断开类 panic(EPIPE / ECONNRESET / http.ErrAbortHandler),
// 仅记录 warn 日志,不尝试写响应体——这些 panic 是网络层噪音,不是应用 bug,
// 向死 socket 写入注定失败。与 gin.Recovery 的默认行为保持一致。
func RecoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			r := recover()
			if r == nil {
				return
			}

			requestID := getRequestID(c)
			path := c.Request.URL.Path
			method := c.Request.Method
			clientIP := c.ClientIP()

			// 客户端断开类 panic 单独处理:warn 日志 + Abort,不写响应体。
			if isBrokenPipe(r) {
				logger.Z().Warn("client_disconnected_panic",
					zap.String("request_id", requestID),
					zap.String("method", method),
					zap.String("path", path),
					zap.String("client_ip", clientIP),
					zap.String("reason", fmt.Sprintf("%v", r)),
				)
				if e, ok := r.(error); ok {
					_ = c.Error(e)
				}
				c.Abort()
				return
			}

			stack := debug.Stack()
			err := normalizePanic(r)

			logger.Z().Error("panic_recovered",
				zap.String("request_id", requestID),
				zap.String("method", method),
				zap.String("path", path),
				zap.String("client_ip", clientIP),
				zap.Error(err),
				zap.ByteString("stack", stack),
			)

			if !c.Writer.Written() {
				msg := i18n.T(i18n.ResolveLocale(c), "error.internal_server_error")
				if msg == "" {
					msg = "Internal Server Error"
				}
				response.ErrorWithHTTPStatus(c, http.StatusInternalServerError, http.StatusInternalServerError, msg)
			}
			c.Abort()
		}()
		c.Next()
	}
}

// isBrokenPipe 判断 panic 值是否为客户端断开类网络错误。
func isBrokenPipe(r interface{}) bool {
	e, ok := r.(error)
	if !ok {
		return false
	}
	if errors.Is(e, http.ErrAbortHandler) {
		return true
	}
	// errors.Is 会沿着 wrap chain / *net.OpError 链查找 syscall 错误
	if errors.Is(e, syscall.EPIPE) || errors.Is(e, syscall.ECONNRESET) {
		return true
	}
	return false
}

func normalizePanic(r interface{}) error {
	switch v := r.(type) {
	case error:
		return v
	case string:
		return fmt.Errorf("panic: %s", v)
	default:
		return fmt.Errorf("panic: %v (type=%T)", v, v)
	}
}
