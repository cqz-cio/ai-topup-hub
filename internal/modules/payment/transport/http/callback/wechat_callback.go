package paymentcallbackhttp

import (
	"encoding/json"
	"net/http"
	"strings"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

const (
	wechatCallbackRespCodeSuccess = "SUCCESS"
	wechatCallbackRespCodeFail    = "FAIL"
	wechatCallbackRespMsgSuccess  = "成功"
	wechatCallbackRespMsgFail     = "失败"
)

type wechatCallbackQuery struct {
	ChannelID uint `form:"channel_id"`
}

func (h *Handler) handleWechatCallback(c *gin.Context) bool {
	log := ginutil.RequestLog(c)
	body, ok := readCallbackBody(c)
	if !ok || !isWechatCallbackRequest(c, body) {
		log.Debugw("wechat_callback_not_matched")
		return false
	}
	var query wechatCallbackQuery
	_ = c.ShouldBindQuery(&query)
	log.Infow("wechat_callback_received", "channel_id", query.ChannelID, "client_ip", c.ClientIP(), "body_size", len(body),
		"wechatpay_timestamp", strings.TrimSpace(c.GetHeader("Wechatpay-Timestamp")),
		"wechatpay_serial", strings.TrimSpace(c.GetHeader("Wechatpay-Serial")))

	payment, _, err := h.service.HandleWechatWebhook(WechatWebhookInput{
		ChannelID: query.ChannelID, Headers: collectRequestHeaders(c), Body: body, Context: c.Request.Context(),
	})
	if err != nil {
		log.Warnw("wechat_callback_handle_failed", "channel_id", query.ChannelID, "error", err)
		h.enqueuePaymentExceptionAlert(c, jsonmap.JSON{
			"alert_type": "wechat_callback_handle_failed", "alert_level": "error",
			"message": strings.TrimSpace(err.Error()), "provider": constants.PaymentChannelTypeWechat,
		})
		respondWechatCallback(c, false)
		return true
	}
	if payment == nil {
		log.Infow("wechat_callback_accepted_no_payment", "channel_id", query.ChannelID)
		respondWechatCallback(c, true)
		return true
	}
	log.Infow("wechat_callback_processed", "channel_id", query.ChannelID, "payment_id", payment.ID, "status", payment.Status)
	respondWechatCallback(c, true)
	return true
}

func isWechatCallbackRequest(c *gin.Context, body []byte) bool {
	for _, header := range []string{"Wechatpay-Signature", "Wechatpay-Timestamp", "Wechatpay-Nonce", "Wechatpay-Serial"} {
		if strings.TrimSpace(c.GetHeader(header)) == "" {
			return false
		}
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		return false
	}
	resource, ok := payload["resource"].(map[string]interface{})
	return ok && resource != nil
}

func collectRequestHeaders(c *gin.Context) map[string]string {
	headers := make(map[string]string, len(c.Request.Header))
	for key, values := range c.Request.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return headers
}

func respondWechatCallback(c *gin.Context, success bool) {
	if success {
		c.JSON(http.StatusOK, gin.H{"code": wechatCallbackRespCodeSuccess, "message": wechatCallbackRespMsgSuccess})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"code": wechatCallbackRespCodeFail, "message": wechatCallbackRespMsgFail})
}
