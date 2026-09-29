package settingsapp

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/dujiao-next/internal/constants"
)

// parseSettingInt 解析设置中的整数值。
func parseSettingInt(value interface{}) (int, error) {
	switch v := value.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	case json.Number:
		if i, err := v.Int64(); err == nil {
			return int(i), nil
		}
		if f, err := v.Float64(); err == nil {
			return int(f), nil
		}
		return 0, fmt.Errorf("invalid json number")
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, fmt.Errorf("empty string")
		}
		parsed, err := strconv.Atoi(trimmed)
		if err != nil {
			return 0, err
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported value type")
	}
}

// normalizeSiteCurrency 统一归一化站点币种。
func normalizeSiteCurrency(raw interface{}) string {
	currency := strings.ToUpper(normalizeSettingText(raw))
	if !settingCurrencyCodePattern.MatchString(currency) {
		return constants.SiteCurrencyDefault
	}
	return currency
}

// IsCurrencyCode reports whether value is a normalized three-letter currency
// code accepted by settings and payment validation.
func IsCurrencyCode(value string) bool {
	return settingCurrencyCodePattern.MatchString(strings.ToUpper(strings.TrimSpace(value)))
}
