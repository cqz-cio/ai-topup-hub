package settingsvalue

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

func ParseInt(value interface{}) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	case json.Number:
		if integer, err := typed.Int64(); err == nil {
			return int(integer), nil
		}
		if decimal, err := typed.Float64(); err == nil {
			return int(decimal), nil
		}
		return 0, fmt.Errorf("invalid json number")
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, fmt.Errorf("empty string")
		}
		return strconv.Atoi(trimmed)
	default:
		return 0, fmt.Errorf("unsupported value type")
	}
}

func ParseFloat(value interface{}) (float64, error) {
	switch typed := value.(type) {
	case float32:
		return float64(typed), nil
	case float64:
		return typed, nil
	case int:
		return float64(typed), nil
	case int64:
		return float64(typed), nil
	case json.Number:
		return typed.Float64()
	case string:
		trimmed := strings.TrimSpace(typed)
		if trimmed == "" {
			return 0, fmt.Errorf("empty string")
		}
		return strconv.ParseFloat(trimmed, 64)
	default:
		return 0, fmt.Errorf("unsupported value type")
	}
}

func ParseBool(value interface{}) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case int:
		return typed != 0
	case int64:
		return typed != 0
	case float64:
		return typed != 0
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		return normalized == "1" || normalized == "true" || normalized == "yes" || normalized == "on"
	default:
		return false
	}
}

func NormalizeTextWithRuneLimit(value interface{}, maxRuneCount int) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	text = strings.TrimSpace(text)
	if text == "" || maxRuneCount <= 0 {
		return text
	}
	runes := []rune(text)
	if len(runes) <= maxRuneCount {
		return text
	}
	return string(runes[:maxRuneCount])
}

func CloneStringSlice(items []string) []string {
	if len(items) == 0 {
		return []string{}
	}
	return append([]string(nil), items...)
}

func NormalizeStringList(value interface{}) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []interface{}:
		items := make([]string, 0, len(typed))
		for _, item := range typed {
			text, _ := item.(string)
			items = append(items, strings.TrimSpace(text))
		}
		return items
	default:
		return nil
	}
}
