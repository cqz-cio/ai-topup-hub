package format

import (
	"fmt"
	"strings"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/constants"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

type OrderItemCounts struct {
	Total    int
	Auto     int
	Manual   int
	Upstream int
}

func BuildOrderItemSummaries(items []orderdomain.OrderItem, locale string) (string, string, OrderItemCounts) {
	counts := OrderItemCounts{Total: len(items)}
	if len(items) == 0 {
		empty := localizedNotificationText(locale, "暂无商品明细", "暫無商品明細", "No item details")
		return empty, empty, counts
	}

	allLines := make([]string, 0, len(items))
	fulfillmentLines := make([]string, 0, len(items))
	for idx, item := range items {
		line := buildNotificationOrderItemLine(idx, item, locale)
		allLines = append(allLines, line)

		switch NormalizeFulfillmentType(item.FulfillmentType) {
		case constants.FulfillmentTypeAuto:
			counts.Auto++
		case constants.FulfillmentTypeUpstream:
			counts.Upstream++
			fulfillmentLines = append(fulfillmentLines, line)
		default:
			counts.Manual++
			fulfillmentLines = append(fulfillmentLines, line)
		}
	}

	if len(fulfillmentLines) == 0 {
		fulfillmentLines = append(fulfillmentLines, localizedNotificationText(locale, "无需人工跟进", "無需人工跟進", "No manual follow-up required"))
	}
	return strings.Join(allLines, "\n"), strings.Join(fulfillmentLines, "\n"), counts
}

func buildNotificationOrderItemLine(index int, item orderdomain.OrderItem, locale string) string {
	title := resolveNotificationLocalizedJSON(item.TitleJSON, locale, constants.LocaleZhCN)
	if title == "" {
		title = localizedNotificationText(locale, "未命名商品", "未命名商品", "Unnamed item")
	}

	skuText := buildNotificationSKUSummary(item.SKUSnapshotJSON, locale)
	fulfillmentLabel := notificationFulfillmentLabel(locale, item.FulfillmentType)
	line := fmt.Sprintf("%d. %s", index+1, title)
	if skuText != "" {
		line += " / " + skuText
	}
	line += fmt.Sprintf(" x%d", item.Quantity)
	if fulfillmentLabel != "" {
		line += " [" + fulfillmentLabel + "]"
	}
	return line
}

func buildNotificationSKUSummary(snapshot jsonmap.JSON, locale string) string {
	if len(snapshot) == 0 {
		return ""
	}
	specText := notificationInterfaceText(snapshot["spec_values"], locale, constants.LocaleZhCN)
	if specText != "" {
		return specText
	}
	code := strings.TrimSpace(fmt.Sprintf("%v", snapshot["sku_code"]))
	if code == "" || code == "<nil>" {
		return ""
	}
	return code
}

func BuildDeliverySummary(locale string, counts OrderItemCounts) string {
	return localizedNotificationText(
		locale,
		fmt.Sprintf("共%d项，自动交付%d项，人工交付%d项，上游交付%d项", counts.Total, counts.Auto, counts.Manual, counts.Upstream),
		fmt.Sprintf("共%d項，自動交付%d項，人工交付%d項，上游交付%d項", counts.Total, counts.Auto, counts.Manual, counts.Upstream),
		fmt.Sprintf("Total %d items, auto %d, manual %d, upstream %d", counts.Total, counts.Auto, counts.Manual, counts.Upstream),
	)
}

func notificationFulfillmentLabel(locale, fulfillmentType string) string {
	switch NormalizeFulfillmentType(fulfillmentType) {
	case constants.FulfillmentTypeAuto:
		return localizedNotificationText(locale, "自动交付", "自動交付", "Auto")
	case constants.FulfillmentTypeUpstream:
		return localizedNotificationText(locale, "上游交付", "上游交付", "Upstream")
	default:
		return localizedNotificationText(locale, "人工交付", "人工交付", "Manual")
	}
}

func NormalizeFulfillmentType(fulfillmentType string) string {
	switch strings.ToLower(strings.TrimSpace(fulfillmentType)) {
	case constants.FulfillmentTypeAuto:
		return constants.FulfillmentTypeAuto
	case constants.FulfillmentTypeUpstream:
		return constants.FulfillmentTypeUpstream
	default:
		return constants.FulfillmentTypeManual
	}
}

func notificationInterfaceText(value interface{}, locale, defaultLocale string) string {
	switch typed := value.(type) {
	case jsonmap.JSON:
		return resolveNotificationLocalizedJSON(typed, locale, defaultLocale)
	case map[string]interface{}:
		return resolveNotificationLocalizedJSON(jsonmap.JSON(typed), locale, defaultLocale)
	case nil:
		return ""
	default:
		text := strings.TrimSpace(fmt.Sprintf("%v", typed))
		if text == "<nil>" {
			return ""
		}
		return text
	}
}

func resolveNotificationLocalizedJSON(value jsonmap.JSON, locale, defaultLocale string) string {
	if len(value) == 0 {
		return ""
	}
	if text := strings.TrimSpace(fmt.Sprintf("%v", value[locale])); text != "" && text != "<nil>" {
		return text
	}
	if text := strings.TrimSpace(fmt.Sprintf("%v", value[defaultLocale])); text != "" && text != "<nil>" {
		return text
	}
	for _, item := range value {
		if text := strings.TrimSpace(fmt.Sprintf("%v", item)); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func localizedNotificationText(locale, zhCN, zhTW, enUS string) string {
	switch normalizeNotificationLocale(locale) {
	case constants.LocaleZhTW:
		return zhTW
	case constants.LocaleEnUS:
		return enUS
	default:
		return zhCN
	}
}
