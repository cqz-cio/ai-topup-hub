package application

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dujiao-next/internal/cache"
	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/modules/notification/application/format"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	"github.com/dujiao-next/internal/queue"
)

func isNotificationEventSupported(eventType string) bool {
	switch strings.ToLower(strings.TrimSpace(eventType)) {
	case constants.NotificationEventWalletRechargeSuccess,
		constants.NotificationEventOrderPaidSuccess,
		constants.NotificationEventManualFulfillmentPending,
		constants.NotificationEventExceptionAlert,
		constants.NotificationEventExceptionAlertCheck:
		return true
	default:
		return false
	}
}

func acquireNotificationDedupe(ctx context.Context, ttlSeconds int, payload queue.NotificationDispatchPayload) (bool, error) {
	if ttlSeconds <= 0 {
		ttlSeconds = 300
	}
	key := buildNotificationDedupeKey(payload)
	return cache.SetNX(ctx, key, "1", time.Duration(ttlSeconds)*time.Second)
}

func buildNotificationDedupeKey(payload queue.NotificationDispatchPayload) string {
	signature := strings.Builder{}
	signature.WriteString(strings.ToLower(strings.TrimSpace(payload.EventType)))
	signature.WriteString("|")
	signature.WriteString(strings.ToLower(strings.TrimSpace(payload.BizType)))
	signature.WriteString("|")
	signature.WriteString(fmt.Sprintf("%d", payload.BizID))
	signature.WriteString("|")

	keys := make([]string, 0, len(payload.Data))
	for key := range payload.Data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if key == "occurred_at" {
			continue
		}
		signature.WriteString(key)
		signature.WriteString("=")
		signature.WriteString(strings.TrimSpace(fmt.Sprintf("%v", payload.Data[key])))
		signature.WriteString(";")
	}
	hash := sha1.Sum([]byte(signature.String()))
	return "notification:dedupe:" + hex.EncodeToString(hash[:])
}

func isInventoryAlertType(alertType string) bool {
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case constants.NotificationAlertTypeOutOfStockProducts, constants.NotificationAlertTypeLowStockProducts:
		return true
	default:
		return false
	}
}

func acquireInventoryAlertInterval(ctx context.Context, intervalSeconds int, payload queue.NotificationDispatchPayload) (bool, error) {
	alertType := format.ResolveInventoryAlertTypeKey(payload.Data)
	if !isInventoryAlertType(alertType) {
		return true, nil
	}
	intervalSeconds = settingsmessaging.NormalizeNotificationInventoryAlertInterval(intervalSeconds)
	key := "notification:inventory_interval:" + alertType
	return cache.SetNX(ctx, key, "1", time.Duration(intervalSeconds)*time.Second)
}

// isPaymentOrderAlertType 判断是否为支付订单类告警
func isPaymentOrderAlertType(alertType string) bool {
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case constants.NotificationAlertTypePendingOrders, constants.NotificationAlertTypePaymentsFailed:
		return true
	default:
		return false
	}
}

// acquirePaymentOrderAlertInterval 获取支付订单告警发送间隔锁
func acquirePaymentOrderAlertInterval(ctx context.Context, intervalSeconds int, payload queue.NotificationDispatchPayload) (bool, error) {
	alertType := resolvePaymentOrderAlertTypeKey(payload.Data)
	if !isPaymentOrderAlertType(alertType) {
		return true, nil
	}
	intervalSeconds = settingsmessaging.NormalizeNotificationPaymentOrderAlertInterval(intervalSeconds)
	key := "notification:payment_order_interval:" + alertType
	return cache.SetNX(ctx, key, "1", time.Duration(intervalSeconds)*time.Second)
}

// resolvePaymentOrderAlertTypeKey 解析支付订单告警类型键
func resolvePaymentOrderAlertTypeKey(data map[string]interface{}) string {
	if len(data) == 0 {
		return ""
	}
	normalized := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", data["alert_type_key"])))
	if isPaymentOrderAlertType(normalized) {
		return normalized
	}
	normalized = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", data["alert_type"])))
	if isPaymentOrderAlertType(normalized) {
		return normalized
	}
	return ""
}
