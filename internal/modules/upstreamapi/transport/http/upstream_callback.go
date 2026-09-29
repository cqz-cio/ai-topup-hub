package upstreamhttp

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dujiao-next/internal/logger"
	procurementcontract "github.com/dujiao-next/internal/modules/procurement/contract"
	"github.com/dujiao-next/internal/shared/jsonmap"
	upstreamadapter "github.com/dujiao-next/internal/upstream"

	"github.com/gin-gonic/gin"
)

type callbackPayload struct {
	Event             string `json:"event"`
	OrderID           uint   `json:"order_id"`
	OrderNo           string `json:"order_no"`
	DownstreamOrderNo string `json:"downstream_order_no"`
	Status            string `json:"status"`
	Fulfillment       *struct {
		Type         string       `json:"type"`
		Status       string       `json:"status"`
		Payload      string       `json:"payload"`
		DeliveryData jsonmap.JSON `json:"delivery_data"`
		DeliveredAt  *time.Time   `json:"delivered_at"`
	} `json:"fulfillment,omitempty"`
	Timestamp int64 `json:"timestamp"`
}

// HandleCallback POST /api/v1/upstream/callback (A 站点接收 B 站回调)
func (h *Handler) HandleCallback(c *gin.Context) {
	// ---- 签名验证 ----
	apiKey := c.GetHeader(upstreamadapter.HeaderApiKey)
	timestampStr := c.GetHeader(upstreamadapter.HeaderTimestamp)
	signature := c.GetHeader(upstreamadapter.HeaderSignature)

	if apiKey == "" || timestampStr == "" || signature == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "missing authentication headers"})
		return
	}

	timestamp, err := upstreamadapter.ParseTimestamp(timestampStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "invalid timestamp"})
		return
	}

	if !upstreamadapter.IsTimestampValid(timestamp) {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "timestamp expired"})
		return
	}

	// 根据 api_key 查找对应的站点连接
	conn, err := h.Connections.GetByApiKey(apiKey)
	if err != nil {
		logger.Errorw("upstream_callback_lookup_connection_failed", "api_key", apiKey, "error", err)
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "internal error"})
		return
	}
	if conn == nil || conn.Status != "active" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "invalid api key"})
		return
	}

	// 读取 body 用于签名验证
	var body []byte
	if c.Request.Body != nil {
		body, err = io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20))
		if err != nil {
			c.JSON(http.StatusOK, gin.H{"ok": false, "message": "failed to read request body"})
			return
		}
		c.Request.Body = io.NopCloser(&bodyBuf{data: body})
	}

	// 解密 api_secret 并验证签名；解密失败视为配置错误，直接拒绝
	apiSecret := conn.ApiSecret
	if h.ConnectionSecrets != nil {
		decrypted, decErr := h.ConnectionSecrets.DecryptSecret(apiSecret)
		if decErr != nil {
			logger.Errorw("upstream_callback_secret_decrypt_failed", "connection_id", conn.ID, "error", decErr)
			c.JSON(http.StatusOK, gin.H{"ok": false, "message": "internal error"})
			return
		}
		apiSecret = decrypted
	}

	if !upstreamadapter.Verify(apiSecret, "POST", "/api/v1/upstream/callback", signature, timestamp, body) {
		logger.Warnw("upstream_callback_signature_invalid", "api_key", apiKey)
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "signature verification failed"})
		return
	}

	// ---- 解析 payload ----
	var payload callbackPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "invalid request body"})
		return
	}

	if payload.DownstreamOrderNo == "" || payload.Status == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "missing required fields"})
		return
	}

	if h.Procurements == nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "service not available"})
		return
	}

	// 根据 downstream_order_no（即本站的 local_order_no）查找对应的采购单
	procOrder, err := h.Procurements.GetByLocalOrderNo(payload.DownstreamOrderNo)
	if err != nil || procOrder == nil {
		logger.Warnw("upstream_callback_procurement_not_found",
			"downstream_order_no", payload.DownstreamOrderNo,
			"upstream_order_id", payload.OrderID,
		)
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "procurement order not found"})
		return
	}

	// 归属校验：采购单必须属于本次认证的连接，且上游订单号需与登记值一致，
	// 防止任意已认证连接凭本地订单号提交他人订单的状态。
	// upstream_order_id 为 0 时说明下单响应尚未落库（回调抢跑），此时只校验连接归属。
	if procOrder.ConnectionID != conn.ID ||
		(procOrder.UpstreamOrderID != 0 && payload.OrderID != procOrder.UpstreamOrderID) {
		logger.Warnw("upstream_callback_ownership_mismatch",
			"api_key", apiKey,
			"connection_id", conn.ID,
			"procurement_connection_id", procOrder.ConnectionID,
			"downstream_order_no", payload.DownstreamOrderNo,
			"payload_order_id", payload.OrderID,
			"procurement_upstream_order_id", procOrder.UpstreamOrderID,
		)
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "procurement order not found"})
		return
	}

	// 转换状态并处理回调
	var uf *procurementcontract.Fulfillment
	if payload.Fulfillment != nil {
		uf = &procurementcontract.Fulfillment{
			Type:         payload.Fulfillment.Type,
			Status:       payload.Fulfillment.Status,
			Payload:      payload.Fulfillment.Payload,
			DeliveryData: payload.Fulfillment.DeliveryData,
			DeliveredAt:  payload.Fulfillment.DeliveredAt,
		}
	}

	upstreamStatus := mapCallbackStatus(payload.Status)
	if err := h.Procurements.HandleUpstreamCallback(procOrder.ID, upstreamStatus, uf); err != nil {
		logger.Warnw("upstream_callback_handle_failed",
			"procurement_order_id", procOrder.ID,
			"upstream_status", upstreamStatus,
			"error", err,
		)
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": "callback processing failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "message": "received"})
}

// bodyBuf 用于重置 body
type bodyBuf struct {
	data   []byte
	offset int
}

func (b *bodyBuf) Read(p []byte) (n int, err error) {
	if b.offset >= len(b.data) {
		return 0, io.EOF
	}
	n = copy(p, b.data[b.offset:])
	b.offset += n
	return n, nil
}

// mapCallbackStatus 将上游订单状态映射为回调处理状态
func mapCallbackStatus(status string) string {
	normalized := strings.ToLower(strings.TrimSpace(status))
	switch normalized {
	case "delivered", "completed", "fulfilled":
		return "delivered"
	case "canceled", "cancelled":
		return "canceled"
	default:
		return normalized
	}
}

// mapOrderErrorToResponse 将订单创建错误映射为上游 API 错误响应
// validateCallbackURL 验证回调 URL 的安全性（防止 SSRF）
func validateCallbackURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid url format")
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("callback url must use http or https")
	}
	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("callback url must have a host")
	}
	// 禁止 localhost 和回环地址
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "0.0.0.0" {
		return fmt.Errorf("callback url must not point to localhost")
	}
	// 检查是否是内网 IP
	ip := net.ParseIP(host)
	if ip != nil {
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
			return fmt.Errorf("callback url must not point to private network")
		}
	}
	return nil
}
