package format

import (
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/queue"
)

// BuildPaymentOrderAlertDispatchPayloads 构建支付订单告警通知载荷。
func BuildPaymentOrderAlertDispatchPayloads(
	setting settingsmessaging.NotificationCenterSetting,
	dashboardSetting settingsstorefront.DashboardSetting,
	payload queue.NotificationDispatchPayload,
	counts dashboard.PaymentOrderAlertCountsRow,
) []queue.NotificationDispatchPayload {
	result := make([]queue.NotificationDispatchPayload, 0, 2)
	if itemPayload, ok := buildPaymentOrderAlertDispatchPayload(
		setting,
		payload,
		constants.NotificationAlertTypePendingOrders,
		counts.PendingPaymentOrders,
		dashboardSetting.Alert.PendingPaymentOrdersThreshold,
	); ok {
		result = append(result, itemPayload)
	}
	if itemPayload, ok := buildPaymentOrderAlertDispatchPayload(
		setting,
		payload,
		constants.NotificationAlertTypePaymentsFailed,
		counts.PaymentsFailed,
		dashboardSetting.Alert.PaymentsFailedThreshold,
	); ok {
		result = append(result, itemPayload)
	}
	return result
}

// buildPaymentOrderAlertDispatchPayload 构建单个支付订单告警通知载荷
func buildPaymentOrderAlertDispatchPayload(
	setting settingsmessaging.NotificationCenterSetting,
	payload queue.NotificationDispatchPayload,
	alertType string,
	value int64,
	threshold int64,
) (queue.NotificationDispatchPayload, bool) {
	if value < threshold {
		return queue.NotificationDispatchPayload{}, false
	}

	locale := ResolveLocale(payload.Locale, setting.DefaultLocale)
	data := CloneVariables(payload.Data)
	if data == nil {
		data = map[string]interface{}{}
	}
	data["alert_type"] = alertTypeLabelByType(locale, alertType)
	data["alert_type_label"] = data["alert_type"]
	data["alert_type_key"] = alertType
	data["alert_level"] = "warning"
	data["alert_value"] = fmt.Sprintf("%d", value)
	data["alert_threshold"] = fmt.Sprintf("%d", threshold)
	data["message"] = buildPaymentOrderAlertMessage(locale, alertType, value, threshold, setting.PaymentOrderAlertIntervalSeconds)

	itemPayload := payload
	itemPayload.EventType = constants.NotificationEventExceptionAlert
	itemPayload.BizType = constants.NotificationBizTypeDashboardAlert
	itemPayload.Data = data
	return itemPayload, true
}

// buildPaymentOrderAlertMessage 构建支付订单告警消息内容
func buildPaymentOrderAlertMessage(locale string, alertType string, value, threshold int64, intervalSeconds int) string {
	intervalText := formatPaymentOrderAlertInterval(locale, intervalSeconds)
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case constants.NotificationAlertTypePendingOrders:
		return localizedNotificationText(
			locale,
			fmt.Sprintf("当前统计周期内待支付订单 %d 笔，已达到告警阈值 %d 笔；本类告警最短 %s 发送一次。", value, threshold, intervalText),
			fmt.Sprintf("目前統計週期內待支付訂單 %d 筆，已達到告警門檻 %d 筆；本類告警最短 %s 發送一次。", value, threshold, intervalText),
			fmt.Sprintf("Pending payment orders reached %d in the current alert window, meeting the threshold of %d; this alert is sent at most once every %s.", value, threshold, intervalText),
		)
	default:
		return localizedNotificationText(
			locale,
			fmt.Sprintf("当前统计周期内支付失败 %d 笔，已达到告警阈值 %d 笔；本类告警最短 %s 发送一次。", value, threshold, intervalText),
			fmt.Sprintf("目前統計週期內支付失敗 %d 筆，已達到告警門檻 %d 筆；本類告警最短 %s 發送一次。", value, threshold, intervalText),
			fmt.Sprintf("Payment failures reached %d in the current alert window, meeting the threshold of %d; this alert is sent at most once every %s.", value, threshold, intervalText),
		)
	}
}

// formatPaymentOrderAlertInterval 格式化支付订单告警发送间隔
func formatPaymentOrderAlertInterval(locale string, seconds int) string {
	seconds = normalizeNotificationPaymentOrderAlertInterval(seconds)
	switch {
	case seconds%3600 == 0:
		hours := seconds / 3600
		return localizedNotificationText(locale, fmt.Sprintf("%d 小时", hours), fmt.Sprintf("%d 小時", hours), fmt.Sprintf("%d hours", hours))
	case seconds%60 == 0:
		minutes := seconds / 60
		return localizedNotificationText(locale, fmt.Sprintf("%d 分钟", minutes), fmt.Sprintf("%d 分鐘", minutes), fmt.Sprintf("%d minutes", minutes))
	default:
		return localizedNotificationText(locale, fmt.Sprintf("%d 秒", seconds), fmt.Sprintf("%d 秒", seconds), fmt.Sprintf("%d seconds", seconds))
	}
}
