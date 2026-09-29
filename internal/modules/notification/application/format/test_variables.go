package format

import (
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
)

func PickMessage(value interface{}, fallback string) string {
	normalized := strings.TrimSpace(fmt.Sprintf("%v", value))
	if normalized == "" || normalized == "<nil>" {
		return fallback
	}
	return normalized
}

func ApplyTestVariables(target map[string]interface{}, defaults map[string]interface{}) {
	if target == nil || len(defaults) == 0 {
		return
	}
	for key, value := range defaults {
		if _, exists := target[key]; exists {
			continue
		}
		target[key] = value
	}
}

func BuildTestVariables(scene, locale string) map[string]interface{} {
	locale = ResolveLocale(locale, constants.LocaleZhCN)
	switch strings.ToLower(strings.TrimSpace(scene)) {
	case constants.NotificationEventWalletRechargeSuccess:
		return map[string]interface{}{
			"customer_label":  localizedNotificationText(locale, "张三 <zhangsan@example.com>", "張三 <zhangsan@example.com>", "Alex Zhang <zhangsan@example.com>"),
			"customer_email":  "zhangsan@example.com",
			"recharge_no":     "RC202603230001",
			"amount":          "100.00",
			"currency":        "USD",
			"payment_channel": "epay/alipay",
		}
	case constants.NotificationEventOrderPaidSuccess:
		return map[string]interface{}{
			"customer_label":            localizedNotificationText(locale, "张三 <zhangsan@example.com>", "張三 <zhangsan@example.com>", "Alex Zhang <zhangsan@example.com>"),
			"customer_email":            "zhangsan@example.com",
			"order_no":                  "DJ202603230001",
			"amount":                    "299.00",
			"currency":                  "USD",
			"payment_channel":           "epay/alipay",
			"items_summary":             buildNotificationTestOrderItems(locale),
			"fulfillment_items_summary": buildNotificationTestFulfillmentItems(locale),
			"delivery_summary":          BuildDeliverySummary(locale, OrderItemCounts{Total: 2, Auto: 1, Manual: 1}),
		}
	case constants.NotificationEventManualFulfillmentPending:
		return map[string]interface{}{
			"customer_label":            localizedNotificationText(locale, "张三 <zhangsan@example.com>", "張三 <zhangsan@example.com>", "Alex Zhang <zhangsan@example.com>"),
			"customer_email":            "zhangsan@example.com",
			"order_no":                  "DJ202603230001",
			"order_status":              constants.OrderStatusPaid,
			"fulfillment_items_summary": buildNotificationTestFulfillmentItems(locale),
			"delivery_summary":          BuildDeliverySummary(locale, OrderItemCounts{Total: 2, Auto: 1, Manual: 1}),
		}
	default:
		return map[string]interface{}{
			"alert_type":             alertTypeLabelByType(locale, constants.NotificationAlertTypeLowStockProducts),
			"alert_type_label":       alertTypeLabelByType(locale, constants.NotificationAlertTypeLowStockProducts),
			"alert_level":            "warning",
			"alert_value":            "2",
			"alert_threshold":        "5",
			"affected_items_count":   "2",
			"affected_product_count": "2",
			"affected_items_summary": buildNotificationTestInventoryItems(locale),
			"message": localizedNotificationText(
				locale,
				"检测到 2 个低库存商品，涉及 2 个具体库存项；本类告警最短 30 分钟发送一次。",
				"偵測到 2 個低庫存商品，涉及 2 個具體庫存項；本類告警最短 30 分鐘發送一次。",
				"Detected 2 low-stock products across 2 inventory items; this alert is sent at most once every 30 minutes.",
			),
		}
	}
}

func buildNotificationTestOrderItems(locale string) string {
	return strings.Join([]string{
		localizedNotificationText(locale, "1. Netflix 年付 / 区域: HK x1 [自动交付]", "1. Netflix 年付 / 區域: HK x1 [自動交付]", "1. Netflix Annual / Region: HK x1 [Auto]"),
		localizedNotificationText(locale, "2. ChatGPT Plus 代充 / 周期: 1个月 x1 [人工交付]", "2. ChatGPT Plus 代充 / 週期: 1個月 x1 [人工交付]", "2. ChatGPT Plus Recharge / Cycle: 1 month x1 [Manual]"),
	}, "\n")
}

func buildNotificationTestFulfillmentItems(locale string) string {
	return localizedNotificationText(
		locale,
		"1. ChatGPT Plus 代充 / 周期: 1个月 x1 [人工交付]",
		"1. ChatGPT Plus 代充 / 週期: 1個月 x1 [人工交付]",
		"1. ChatGPT Plus Recharge / Cycle: 1 month x1 [Manual]",
	)
}

func buildNotificationTestInventoryItems(locale string) string {
	return strings.Join([]string{
		localizedNotificationText(locale, "1. Netflix 年付 / 区域: HK [自动交付] 剩余 1（低库存）", "1. Netflix 年付 / 區域: HK [自動交付] 剩餘 1（低庫存）", "1. Netflix Annual / Region: HK [Auto] | Remaining 1 (Low stock)"),
		localizedNotificationText(locale, "2. ChatGPT Plus 代充 / 周期: 1个月 [人工交付] 剩余 0（缺货）", "2. ChatGPT Plus 代充 / 週期: 1個月 [人工交付] 剩餘 0（缺貨）", "2. ChatGPT Plus Recharge / Cycle: 1 month [Manual] | Remaining 0 (Out of stock)"),
	}, "\n")
}
