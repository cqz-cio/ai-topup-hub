package paymentcallbackhttp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	ginutil "github.com/dujiao-next/internal/platform/http/ginutil"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"

	"github.com/gin-gonic/gin"
)

type bodyCallback struct {
	providerType string
	logPrefix    string
	orderNo      string
	providerRef  string
	successBody  string
	failBody     string
	contentType  string
	alertType    string
	alertData    jsonmap.JSON
}

type okpayProbe struct {
	Sign     string
	OrderID  string
	UniqueID string
}

func parseOkpayProbe(body []byte) (okpayProbe, error) {
	raw := strings.TrimSpace(string(body))
	if raw == "" || !strings.HasPrefix(raw, "{") {
		return okpayProbe{}, fmt.Errorf("empty callback")
	}
	var payload struct {
		Sign string `json:"sign"`
		Data struct {
			OrderID  string `json:"order_id"`
			UniqueID string `json:"unique_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return okpayProbe{}, err
	}
	return okpayProbe{
		Sign:     strings.TrimSpace(payload.Sign),
		OrderID:  strings.TrimSpace(payload.Data.OrderID),
		UniqueID: strings.TrimSpace(payload.Data.UniqueID),
	}, nil
}

func (h *Handler) handleOkpayCallback(c *gin.Context) bool {
	body, ok := readCallbackBody(c)
	if !ok {
		return false
	}
	probe, err := parseOkpayProbe(body)
	if err != nil {
		ginutil.RequestLog(c).Debugw("okpay_callback_parse_failed", "error", err)
		return false
	}
	uniqueID, orderID := strings.TrimSpace(probe.UniqueID), strings.TrimSpace(probe.OrderID)
	if strings.TrimSpace(probe.Sign) == "" || (uniqueID == "" && orderID == "") {
		ginutil.RequestLog(c).Debugw("okpay_callback_not_matched", "reason", "missing_sign_or_ids")
		return false
	}
	ginutil.RequestLog(c).Infow("okpay_callback_received", "unique_id", uniqueID, "order_id", orderID, "body_size", len(body))
	return h.processBodyCallback(c, body, bodyCallback{
		providerType: constants.PaymentProviderOkpay, logPrefix: "okpay",
		orderNo: uniqueID, providerRef: orderID,
		successBody: constants.OkpayCallbackSuccess, failBody: constants.OkpayCallbackFail,
		contentType: "application/json", alertType: "okpay_callback_handle_failed",
		alertData: jsonmap.JSON{"unique_id": uniqueID},
	})
}

func (h *Handler) handleTokenPayCallback(c *gin.Context) bool {
	body, ok := readCallbackBody(c)
	if !ok {
		return false
	}
	var probe struct {
		Signature string `json:"Signature"`
		OrderID   string `json:"OutOrderId"`
		TokenID   string `json:"Id"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		ginutil.RequestLog(c).Debugw("tokenpay_callback_parse_failed", "error", err)
		return false
	}
	if strings.TrimSpace(probe.Signature) == "" || strings.TrimSpace(probe.OrderID) == "" || strings.TrimSpace(probe.TokenID) == "" {
		ginutil.RequestLog(c).Debugw("tokenpay_callback_not_matched")
		return false
	}
	ginutil.RequestLog(c).Infow("tokenpay_callback_received", "out_order_id", probe.OrderID, "token_order_id", probe.TokenID, "body_size", len(body))
	return h.processBodyCallback(c, body, bodyCallback{
		providerType: constants.PaymentProviderTokenpay, logPrefix: "tokenpay",
		orderNo: probe.OrderID, providerRef: probe.TokenID,
		successBody: constants.TokenPayCallbackSuccess, failBody: constants.TokenPayCallbackFail,
	})
}

func (h *Handler) handleEpusdtCallback(c *gin.Context) bool {
	body, ok := readCallbackBody(c)
	if !ok {
		return false
	}
	var probe struct {
		PID     string `json:"pid"`
		TradeID string `json:"trade_id"`
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		ginutil.RequestLog(c).Debugw("epusdt_callback_parse_failed", "error", err)
		return false
	}
	if strings.TrimSpace(probe.PID) == "" || probe.TradeID == "" || probe.OrderID == "" {
		ginutil.RequestLog(c).Debugw("epusdt_callback_feature_missing", "has_pid", strings.TrimSpace(probe.PID) != "", "has_trade_id", probe.TradeID != "", "has_order_id", probe.OrderID != "")
		return false
	}
	ginutil.RequestLog(c).Infow("epusdt_callback_received", "pid", probe.PID, "trade_id", probe.TradeID, "order_id", probe.OrderID, "body_size", len(body))
	return h.processBodyCallback(c, body, bodyCallback{
		providerType: constants.PaymentProviderEpusdt, logPrefix: "epusdt",
		orderNo: probe.OrderID, providerRef: probe.TradeID,
		successBody: constants.EpusdtCallbackSuccess, failBody: constants.EpusdtCallbackFail,
	})
}

func (h *Handler) handleBepusdtCallback(c *gin.Context) bool {
	body, ok := readCallbackBody(c)
	if !ok {
		return false
	}
	var probe struct {
		TradeID string `json:"trade_id"`
		OrderID string `json:"order_id"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		ginutil.RequestLog(c).Debugw("bepusdt_callback_parse_failed", "error", err)
		return false
	}
	if probe.TradeID == "" || probe.OrderID == "" {
		ginutil.RequestLog(c).Debugw("bepusdt_callback_missing_fields", "trade_id", probe.TradeID, "order_id", probe.OrderID)
		return false
	}
	ginutil.RequestLog(c).Infow("bepusdt_callback_received", "trade_id", probe.TradeID, "order_id", probe.OrderID, "body_size", len(body))
	return h.processBodyCallback(c, body, bodyCallback{
		providerType: constants.PaymentProviderBepusdt, logPrefix: "bepusdt",
		orderNo: probe.OrderID, providerRef: probe.TradeID,
		successBody: constants.BepusdtCallbackSuccess, failBody: constants.BepusdtCallbackFail,
	})
}

func readCallbackBody(c *gin.Context) ([]byte, bool) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return nil, false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	return body, len(strings.TrimSpace(string(body))) > 0
}

func (h *Handler) processBodyCallback(c *gin.Context, body []byte, callback bodyCallback) bool {
	log := ginutil.RequestLog(c)
	payment, err := h.payments.GetByGatewayOrderNo(strings.TrimSpace(callback.orderNo))
	if err != nil || payment == nil {
		payment, err = h.payments.GetLatestByProviderRef(strings.TrimSpace(callback.providerRef))
		if err != nil || payment == nil {
			log.Warnw(callback.logPrefix+"_callback_payment_not_found", "order_no", callback.orderNo, "provider_ref", callback.providerRef, "error", err)
			respondBodyCallback(c, callback, false)
			return true
		}
	}

	channel, err := h.channels.GetByID(payment.ChannelID)
	if err != nil || channel == nil {
		log.Warnw(callback.logPrefix+"_callback_channel_not_found", "payment_id", payment.ID, "channel_id", payment.ChannelID, "error", err)
		respondBodyCallback(c, callback, false)
		return true
	}
	if !strings.EqualFold(strings.TrimSpace(channel.ProviderType), callback.providerType) {
		log.Warnw(callback.logPrefix+"_callback_provider_invalid", "payment_id", payment.ID, "channel_id", channel.ID, "provider_type", channel.ProviderType)
		respondBodyCallback(c, callback, false)
		return true
	}

	updated, err := h.service.HandleSyncCallback(channel, nil, body)
	if err != nil {
		log.Errorw(callback.logPrefix+"_callback_handle_failed", "payment_id", payment.ID, "error", err)
		if callback.alertType != "" {
			alert := jsonmap.JSON{
				"alert_type": callback.alertType, "alert_level": "error",
				"payment_id": fmt.Sprintf("%d", payment.ID), "message": strings.TrimSpace(err.Error()),
				"provider": callback.providerType,
			}
			for key, value := range callback.alertData {
				alert[key] = value
			}
			h.enqueuePaymentExceptionAlert(c, alert)
		}
		respondBodyCallback(c, callback, false)
		return true
	}
	log.Infow(callback.logPrefix+"_callback_processed", "payment_id", payment.ID, "status", updated.Status)
	respondBodyCallback(c, callback, true)
	return true
}

func respondBodyCallback(c *gin.Context, callback bodyCallback, success bool) {
	body := callback.failBody
	if success {
		body = callback.successBody
	}
	if callback.contentType != "" {
		c.Data(http.StatusOK, callback.contentType, []byte(body))
		return
	}
	c.String(http.StatusOK, body)
}
