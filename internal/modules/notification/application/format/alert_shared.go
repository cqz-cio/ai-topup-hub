package format

import (
	"fmt"
	"strings"

	"github.com/dujiao-next/internal/constants"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
)

func thresholdValueByAlertType(setting settingsstorefront.DashboardAlertSetting, alertType string) int64 {
	switch alertType {
	case constants.NotificationAlertTypeOutOfStockProducts:
		return setting.OutOfStockProductsThreshold
	case constants.NotificationAlertTypeLowStockProducts:
		return setting.LowStockThreshold
	case constants.NotificationAlertTypePendingOrders:
		return setting.PendingPaymentOrdersThreshold
	case constants.NotificationAlertTypePaymentsFailed:
		return setting.PaymentsFailedThreshold
	default:
		return 0
	}
}

func isInventoryAlertType(alertType string) bool {
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case constants.NotificationAlertTypeOutOfStockProducts, constants.NotificationAlertTypeLowStockProducts:
		return true
	default:
		return false
	}
}

func alertTypeLabelByType(locale, alertType string) string {
	type labels struct{ zhCN, zhTW, enUS string }
	values := map[string]labels{
		constants.NotificationAlertTypeOutOfStockProducts: {"售罄商品", "售罄商品", "Out of Stock"},
		constants.NotificationAlertTypeLowStockProducts:   {"低库存商品", "低庫存商品", "Low Stock"},
		constants.NotificationAlertTypePendingOrders:      {"待支付订单", "待支付訂單", "Pending Payment"},
		constants.NotificationAlertTypePaymentsFailed:     {"支付失败", "支付失敗", "Payment Failed"},
	}
	value, ok := values[alertType]
	if !ok {
		return alertType
	}
	switch strings.ToLower(strings.TrimSpace(locale)) {
	case "zh-tw":
		return value.zhTW
	case "en-us", "en":
		return value.enUS
	default:
		return value.zhCN
	}
}

func NormalizeInventoryAlertTypeKey(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	inventoryAlertTypes := []string{
		constants.NotificationAlertTypeOutOfStockProducts,
		constants.NotificationAlertTypeLowStockProducts,
	}
	locales := []string{
		constants.LocaleZhCN,
		constants.LocaleZhTW,
		constants.LocaleEnUS,
	}
	for _, alertType := range inventoryAlertTypes {
		if normalized == strings.ToLower(strings.TrimSpace(alertType)) {
			return alertType
		}
		for _, locale := range locales {
			label := strings.ToLower(strings.TrimSpace(alertTypeLabelByType(locale, alertType)))
			if normalized == label {
				return alertType
			}
		}
	}
	return ""
}

func ResolveInventoryAlertTypeKey(data map[string]interface{}) string {
	if len(data) == 0 {
		return ""
	}
	if key := NormalizeInventoryAlertTypeKey(toString(data["alert_type_key"])); key != "" {
		return key
	}
	return NormalizeInventoryAlertTypeKey(toString(data["alert_type"]))
}

func toString(value interface{}) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprintf("%v", value))
}
