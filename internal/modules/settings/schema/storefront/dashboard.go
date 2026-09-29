package settingsstorefront

import (
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// DashboardAlertSetting 描述仪表盘告警阈值。
type DashboardAlertSetting struct {
	LowStockThreshold             int64 `json:"low_stock_threshold"`
	OutOfStockProductsThreshold   int64 `json:"out_of_stock_products_threshold"`
	PendingPaymentOrdersThreshold int64 `json:"pending_payment_orders_threshold"`
	PaymentsFailedThreshold       int64 `json:"payments_failed_threshold"`
}

// DashboardRankingSetting 描述仪表盘排行数量限制。
type DashboardRankingSetting struct {
	TopProductsLimit int `json:"top_products_limit"`
	TopChannelsLimit int `json:"top_channels_limit"`
}

// DashboardAccountingSetting 描述仪表盘财务统计口径。
type DashboardAccountingSetting struct {
	RefundReversesCost bool `json:"refund_reverses_cost"`
}

// DashboardSetting 是仪表盘设置的 typed representation。
type DashboardSetting struct {
	Alert      DashboardAlertSetting      `json:"alert"`
	Ranking    DashboardRankingSetting    `json:"ranking"`
	Accounting DashboardAccountingSetting `json:"accounting"`
}

// DefaultDashboardSetting 返回稳定的仪表盘默认设置。
func DefaultDashboardSetting() DashboardSetting {
	return NormalizeDashboardSetting(DashboardSetting{
		Alert: DashboardAlertSetting{
			LowStockThreshold:             5,
			OutOfStockProductsThreshold:   1,
			PendingPaymentOrdersThreshold: 20,
			PaymentsFailedThreshold:       10,
		},
		Ranking: DashboardRankingSetting{
			TopProductsLimit: 5,
			TopChannelsLimit: 5,
		},
		Accounting: DashboardAccountingSetting{
			RefundReversesCost: false,
		},
	})
}

// NormalizeDashboardSetting 将越界值恢复为既有默认值。
func NormalizeDashboardSetting(setting DashboardSetting) DashboardSetting {
	if setting.Alert.LowStockThreshold < 1 || setting.Alert.LowStockThreshold > 500 {
		setting.Alert.LowStockThreshold = 5
	}
	if setting.Alert.OutOfStockProductsThreshold < 1 || setting.Alert.OutOfStockProductsThreshold > 10000 {
		setting.Alert.OutOfStockProductsThreshold = 1
	}
	if setting.Alert.PendingPaymentOrdersThreshold < 1 || setting.Alert.PendingPaymentOrdersThreshold > 100000 {
		setting.Alert.PendingPaymentOrdersThreshold = 20
	}
	if setting.Alert.PaymentsFailedThreshold < 1 || setting.Alert.PaymentsFailedThreshold > 100000 {
		setting.Alert.PaymentsFailedThreshold = 10
	}
	if setting.Ranking.TopProductsLimit < 1 || setting.Ranking.TopProductsLimit > 20 {
		setting.Ranking.TopProductsLimit = 5
	}
	if setting.Ranking.TopChannelsLimit < 1 || setting.Ranking.TopChannelsLimit > 20 {
		setting.Ranking.TopChannelsLimit = 5
	}
	return setting
}

// DecodeDashboardSetting 从持久化 JSON 解码，并对缺失字段使用 fallback。
func DecodeDashboardSetting(raw jsonmap.JSON, fallback DashboardSetting) DashboardSetting {
	result := fallback
	if alert, ok := raw["alert"].(map[string]interface{}); ok {
		if parsed, err := settingsvalue.ParseInt(alert["low_stock_threshold"]); err == nil {
			result.Alert.LowStockThreshold = int64(parsed)
		}
		if parsed, err := settingsvalue.ParseInt(alert["out_of_stock_products_threshold"]); err == nil {
			result.Alert.OutOfStockProductsThreshold = int64(parsed)
		}
		if parsed, err := settingsvalue.ParseInt(alert["pending_payment_orders_threshold"]); err == nil {
			result.Alert.PendingPaymentOrdersThreshold = int64(parsed)
		}
		if parsed, err := settingsvalue.ParseInt(alert["payments_failed_threshold"]); err == nil {
			result.Alert.PaymentsFailedThreshold = int64(parsed)
		}
	}
	if ranking, ok := raw["ranking"].(map[string]interface{}); ok {
		if parsed, err := settingsvalue.ParseInt(ranking["top_products_limit"]); err == nil {
			result.Ranking.TopProductsLimit = parsed
		}
		if parsed, err := settingsvalue.ParseInt(ranking["top_channels_limit"]); err == nil {
			result.Ranking.TopChannelsLimit = parsed
		}
	}
	if accounting, ok := raw["accounting"].(map[string]interface{}); ok {
		result.Accounting.RefundReversesCost = settingsvalue.ReadBool(
			accounting,
			"refund_reverses_cost",
			result.Accounting.RefundReversesCost,
		)
	}
	return NormalizeDashboardSetting(result)
}

// EncodeDashboardSetting 把 typed setting 编码为稳定的持久化 JSON。
func EncodeDashboardSetting(setting DashboardSetting) jsonmap.JSON {
	normalized := NormalizeDashboardSetting(setting)
	return jsonmap.JSON{
		"alert": map[string]interface{}{
			"low_stock_threshold":              normalized.Alert.LowStockThreshold,
			"out_of_stock_products_threshold":  normalized.Alert.OutOfStockProductsThreshold,
			"pending_payment_orders_threshold": normalized.Alert.PendingPaymentOrdersThreshold,
			"payments_failed_threshold":        normalized.Alert.PaymentsFailedThreshold,
		},
		"ranking": map[string]interface{}{
			"top_products_limit": normalized.Ranking.TopProductsLimit,
			"top_channels_limit": normalized.Ranking.TopChannelsLimit,
		},
		"accounting": map[string]interface{}{
			"refund_reverses_cost": normalized.Accounting.RefundReversesCost,
		},
	}
}

// NormalizeDashboardSettingJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeDashboardSettingJSON(raw jsonmap.JSON) jsonmap.JSON {
	return EncodeDashboardSetting(DecodeDashboardSetting(raw, DefaultDashboardSetting()))
}
