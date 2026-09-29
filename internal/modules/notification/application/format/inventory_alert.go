package format

import (
	"fmt"
	"sort"
	"strings"

	productdomain "github.com/dujiao-next/internal/modules/catalog/product/domain"

	"github.com/dujiao-next/internal/constants"
	dashboard "github.com/dujiao-next/internal/modules/dashboard/contract"
	settingsmessaging "github.com/dujiao-next/internal/modules/settings/schema/messaging"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/queue"
)

func BuildInventoryAlertDispatchPayloads(
	setting settingsmessaging.NotificationCenterSetting,
	dashboardSetting settingsstorefront.DashboardSetting,
	payload queue.NotificationDispatchPayload,
	rows []dashboard.InventoryAlertRow,
) []queue.NotificationDispatchPayload {
	filtered := filterInventoryAlertRows(rows, setting.IgnoredProductIDs)
	if len(filtered) == 0 {
		return nil
	}

	locale := ResolveLocale(payload.Locale, setting.DefaultLocale)
	groups := map[string][]dashboard.InventoryAlertRow{
		constants.NotificationAlertTypeOutOfStockProducts: {},
		constants.NotificationAlertTypeLowStockProducts:   {},
	}
	for _, row := range filtered {
		alertType := strings.ToLower(strings.TrimSpace(row.AlertType))
		if !isInventoryAlertType(alertType) {
			continue
		}
		groups[alertType] = append(groups[alertType], row)
	}

	result := make([]queue.NotificationDispatchPayload, 0, 2)
	for _, alertType := range []string{constants.NotificationAlertTypeOutOfStockProducts, constants.NotificationAlertTypeLowStockProducts} {
		group := groups[alertType]
		if len(group) == 0 {
			continue
		}

		productCount := inventoryAlertUniqueProductCount(group)
		if alertType == constants.NotificationAlertTypeOutOfStockProducts && int64(productCount) < thresholdValueByAlertType(dashboardSetting.Alert, alertType) {
			continue
		}

		data := CloneVariables(payload.Data)
		if data == nil {
			data = map[string]interface{}{}
		}
		data["alert_type"] = alertTypeLabelByType(locale, alertType)
		data["alert_type_label"] = data["alert_type"]
		data["alert_type_key"] = alertType
		data["alert_level"] = inventoryAlertLevel(alertType)
		data["alert_value"] = fmt.Sprintf("%d", len(group))
		data["alert_threshold"] = fmt.Sprintf("%d", thresholdValueByAlertType(dashboardSetting.Alert, alertType))
		data["affected_items_count"] = fmt.Sprintf("%d", len(group))
		data["affected_product_count"] = fmt.Sprintf("%d", productCount)
		data["affected_items_summary"] = buildInventoryAlertSummary(locale, group)
		data["inventory_alert_scope"] = "inventory"
		data["message"] = buildInventoryAlertMessage(locale, alertType, productCount, len(group), setting.InventoryAlertIntervalSeconds)

		itemPayload := payload
		itemPayload.EventType = constants.NotificationEventExceptionAlert
		itemPayload.BizType = constants.NotificationBizTypeDashboardAlert
		itemPayload.Data = data
		result = append(result, itemPayload)
	}
	return result
}

func filterInventoryAlertRows(rows []dashboard.InventoryAlertRow, ignoredProductIDs []uint) []dashboard.InventoryAlertRow {
	if len(rows) == 0 {
		return nil
	}
	if len(ignoredProductIDs) == 0 {
		return append([]dashboard.InventoryAlertRow(nil), rows...)
	}
	ignored := make(map[uint]struct{}, len(ignoredProductIDs))
	for _, id := range ignoredProductIDs {
		if id == 0 {
			continue
		}
		ignored[id] = struct{}{}
	}
	result := make([]dashboard.InventoryAlertRow, 0, len(rows))
	for _, row := range rows {
		if _, skip := ignored[row.ProductID]; skip {
			continue
		}
		result = append(result, row)
	}
	return result
}

func inventoryAlertUniqueProductCount(rows []dashboard.InventoryAlertRow) int {
	seen := make(map[uint]struct{}, len(rows))
	for _, row := range rows {
		if row.ProductID == 0 {
			continue
		}
		seen[row.ProductID] = struct{}{}
	}
	return len(seen)
}

func inventoryAlertLevel(alertType string) string {
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case constants.NotificationAlertTypeOutOfStockProducts:
		return "error"
	default:
		return "warning"
	}
}

func buildInventoryAlertSummary(locale string, rows []dashboard.InventoryAlertRow) string {
	if len(rows) == 0 {
		return ""
	}
	sortedRows := append([]dashboard.InventoryAlertRow(nil), rows...)
	sort.SliceStable(sortedRows, func(i, j int) bool {
		if sortedRows[i].ProductID != sortedRows[j].ProductID {
			return sortedRows[i].ProductID < sortedRows[j].ProductID
		}
		if sortedRows[i].SKUID != sortedRows[j].SKUID {
			return sortedRows[i].SKUID < sortedRows[j].SKUID
		}
		return strings.Compare(sortedRows[i].AlertType, sortedRows[j].AlertType) < 0
	})

	lines := make([]string, 0, len(sortedRows))
	for idx, row := range sortedRows {
		title := resolveNotificationLocalizedJSON(row.ProductTitleJSON, locale, constants.LocaleZhCN)
		if title == "" {
			title = localizedNotificationText(locale, "未命名商品", "未命名商品", "Unnamed item")
		}
		skuText := buildInventoryAlertSKUSummary(row, locale)
		fulfillmentLabel := notificationFulfillmentLabel(locale, row.FulfillmentType)
		statusLabel := inventoryAlertStatusLabel(locale, row.AlertType)

		line := fmt.Sprintf("%d. %s", idx+1, title)
		if skuText != "" {
			line += " / " + skuText
		}
		if fulfillmentLabel != "" {
			line += " [" + fulfillmentLabel + "]"
		}
		line += localizedNotificationText(
			locale,
			fmt.Sprintf(" 剩余 %d（%s）", row.AvailableStock, statusLabel),
			fmt.Sprintf(" 剩餘 %d（%s）", row.AvailableStock, statusLabel),
			fmt.Sprintf(" | Remaining %d (%s)", row.AvailableStock, statusLabel),
		)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func buildInventoryAlertSKUSummary(row dashboard.InventoryAlertRow, locale string) string {
	specText := notificationInterfaceText(row.SKUSpecValuesJSON, locale, constants.LocaleZhCN)
	if specText != "" {
		return specText
	}
	code := strings.TrimSpace(row.SKUCode)
	if code == "" || strings.EqualFold(code, productdomain.DefaultSKUCode) {
		return ""
	}
	return code
}

func inventoryAlertStatusLabel(locale, alertType string) string {
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case constants.NotificationAlertTypeOutOfStockProducts:
		return localizedNotificationText(locale, "缺货", "缺貨", "Out of stock")
	default:
		return localizedNotificationText(locale, "低库存", "低庫存", "Low stock")
	}
}

func buildInventoryAlertMessage(locale, alertType string, productCount, itemCount, intervalSeconds int) string {
	intervalText := formatInventoryAlertInterval(locale, intervalSeconds)
	switch strings.ToLower(strings.TrimSpace(alertType)) {
	case constants.NotificationAlertTypeOutOfStockProducts:
		return localizedNotificationText(
			locale,
			fmt.Sprintf("检测到 %d 个 SKU 缺货（涉及 %d 个商品）；本类告警最短 %s 发送一次。", itemCount, productCount, intervalText),
			fmt.Sprintf("偵測到 %d 個 SKU 缺貨（涉及 %d 個商品）；本類告警最短 %s 發送一次。", itemCount, productCount, intervalText),
			fmt.Sprintf("Detected %d out-of-stock SKUs across %d products; this alert is sent at most once every %s.", itemCount, productCount, intervalText),
		)
	default:
		return localizedNotificationText(
			locale,
			fmt.Sprintf("检测到 %d 个 SKU 低库存（涉及 %d 个商品）；本类告警最短 %s 发送一次。", itemCount, productCount, intervalText),
			fmt.Sprintf("偵測到 %d 個 SKU 低庫存（涉及 %d 個商品）；本類告警最短 %s 發送一次。", itemCount, productCount, intervalText),
			fmt.Sprintf("Detected %d low-stock SKUs across %d products; this alert is sent at most once every %s.", itemCount, productCount, intervalText),
		)
	}
}

func formatInventoryAlertInterval(locale string, seconds int) string {
	seconds = normalizeNotificationInventoryAlertInterval(seconds)
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
