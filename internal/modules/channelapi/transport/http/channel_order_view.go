package channelhttp

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	orderdomain "github.com/dujiao-next/internal/modules/order/domain"

	"github.com/dujiao-next/internal/i18n"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/dujiao-next/internal/shared/money"

	"github.com/gin-gonic/gin"
)

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

func resolveLocalizedJSON(values jsonmap.JSON, locale, defaultLocale string) string {
	if len(values) == 0 {
		return ""
	}
	for _, key := range []string{locale, defaultLocale} {
		if value, ok := values[key]; ok {
			if text := fmt.Sprintf("%v", value); text != "" && text != "<nil>" {
				return text
			}
		}
	}
	for _, value := range values {
		if text := fmt.Sprintf("%v", value); text != "" && text != "<nil>" {
			return text
		}
	}
	return ""
}

func stripHTML(value string) string {
	text := htmlTagRe.ReplaceAllString(value, "")
	lines := strings.Split(text, "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n")
}

func truncate(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit]) + "..."
}

func channelOrderPaidAmount(order *orderdomain.Order) string {
	if order == nil {
		return "0.00"
	}
	return money.FromDecimal(order.WalletPaidAmount.Decimal.Add(order.OnlinePaidAmount.Decimal)).StringFixed(2)
}

func formatChannelNullableTime(value *time.Time) *string {
	if value == nil {
		return nil
	}
	formatted := value.Format(time.RFC3339)
	return &formatted
}

func channelLocalizedValue(value interface{}, locale, defaultLocale string) string {
	switch typed := value.(type) {
	case jsonmap.JSON:
		return resolveLocalizedJSON(typed, locale, defaultLocale)
	case map[string]interface{}:
		return resolveLocalizedJSON(jsonmap.JSON(typed), locale, defaultLocale)
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

func channelLocaleValue(c *gin.Context, explicit string) string {
	if value := strings.TrimSpace(explicit); value != "" {
		return value
	}
	return i18n.ResolveLocale(c)
}
