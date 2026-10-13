package middleware

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dujiao-next/internal/platform/http/response"
	"github.com/gin-gonic/gin"
)

// BodyLimit runs before JSON parsing or key extraction. Reading at most limit+1
// also rejects chunked requests with no Content-Length without unbounded memory.
func BodyLimit(limit int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Body == nil || c.Request.Body == http.NoBody {
			return
		}
		if c.Request.ContentLength > limit {
			rejectLargeBody(c)
			return
		}
		original := c.Request.Body
		body, err := io.ReadAll(io.LimitReader(original, limit+1))
		_ = original.Close()
		if int64(len(body)) > limit {
			rejectLargeBody(c)
			return
		}
		if err != nil {
			response.ErrorWithHTTPStatus(c, http.StatusBadRequest, response.CodeBadRequest, "Invalid request body")
			c.Abort()
			return
		}
		c.Request.Body = io.NopCloser(bytes.NewReader(body))
	}
}

func rejectLargeBody(c *gin.Context) {
	response.ErrorWithHTTPStatus(c, http.StatusRequestEntityTooLarge, response.CodeBadRequest, "Request body too large")
	c.Abort()
}

// ConcurrencyLimit rejects excess work immediately instead of retaining an
// unbounded queue of goroutines waiting for bcrypt, database or SMTP resources.
func ConcurrencyLimit(limit int) gin.HandlerFunc {
	if limit < 1 {
		limit = 16
	}
	slots := make(chan struct{}, limit)
	return func(c *gin.Context) {
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			c.Next()
		default:
			c.Header("Retry-After", "1")
			response.ErrorWithHTTPStatus(c, http.StatusServiceUnavailable, response.CodeInternal, "Service busy; please retry later")
			c.Abort()
		}
	}
}

func MutationsOnly(handler gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		switch c.Request.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			c.Next()
		default:
			handler(c)
		}
	}
}

// Account limits are independent of the source IP and store a digest instead
// of raw addresses. Apply an IP limit first to bound arbitrary account keys.
func KeyByJSONField(field string) RateLimitKeyFunc {
	return func(c *gin.Context) string {
		value := strings.ToLower(strings.TrimSpace(readJSONField(c, field)))
		if value == "" {
			return "missing|" + c.ClientIP()
		}
		return fmt.Sprintf("%x", sha256.Sum256([]byte(value)))
	}
}

func KeyByUserID(c *gin.Context) string {
	if id, ok := c.Get("user_id"); ok {
		return fmt.Sprint(id)
	}
	return c.ClientIP()
}
