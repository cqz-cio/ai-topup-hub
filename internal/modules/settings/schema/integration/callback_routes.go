package settingsintegration

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// CallbackRoutesSetting 回调路由配置
type CallbackRoutesSetting struct {
	PaymentCallback  string
	DujiaoPayWebhook string
	PaypalWebhook    string
	StripeWebhook    string
	UpstreamCallback string
}

// HasCustomRoutes 返回是否设置了任何自定义回调路由
func (s *CallbackRoutesSetting) HasCustomRoutes() bool {
	return s.PaymentCallback != "" || s.DujiaoPayWebhook != "" || s.PaypalWebhook != "" ||
		s.StripeWebhook != "" || s.UpstreamCallback != ""
}

// DecodeCallbackRoutesSetting 从 JSON map 解析回调路由配置
func DecodeCallbackRoutesSetting(value jsonmap.JSON) CallbackRoutesSetting {
	return CallbackRoutesSetting{
		PaymentCallback:  normalizeCallbackRoutePath(settingsvalue.ReadString(value, constants.SettingFieldPaymentCallback, "")),
		DujiaoPayWebhook: normalizeCallbackRoutePath(settingsvalue.ReadString(value, constants.SettingFieldDujiaoPayWebhook, "")),
		PaypalWebhook:    normalizeCallbackRoutePath(settingsvalue.ReadString(value, constants.SettingFieldPaypalWebhook, "")),
		StripeWebhook:    normalizeCallbackRoutePath(settingsvalue.ReadString(value, constants.SettingFieldStripeWebhook, "")),
		UpstreamCallback: normalizeCallbackRoutePath(settingsvalue.ReadString(value, constants.SettingFieldUpstreamCallback, "")),
	}
}

// EncodeCallbackRoutesSetting 将回调路由配置序列化为 JSON map
func EncodeCallbackRoutesSetting(s CallbackRoutesSetting) jsonmap.JSON {
	return jsonmap.JSON{
		constants.SettingFieldPaymentCallback:  s.PaymentCallback,
		constants.SettingFieldDujiaoPayWebhook: s.DujiaoPayWebhook,
		constants.SettingFieldPaypalWebhook:    s.PaypalWebhook,
		constants.SettingFieldStripeWebhook:    s.StripeWebhook,
		constants.SettingFieldUpstreamCallback: s.UpstreamCallback,
	}
}

// NormalizeCallbackRoutesSettingJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeCallbackRoutesSettingJSON(value jsonmap.JSON) jsonmap.JSON {
	setting := DecodeCallbackRoutesSetting(value)
	deduplicateCallbackRoutes(&setting)
	return EncodeCallbackRoutesSetting(setting)
}

// reservedRoutePrefixes 已有路由前缀，自定义回调路由不得与之冲突
var reservedRoutePrefixes = []string{
	"/api/v1/public/",
	"/api/v1/admin/",
	"/api/v1/auth/",
	"/api/v1/guest/",
	"/api/v1/channel/",
	"/api/v1/upstream/api/",
	"/api/v1/user/",
}

// normalizeCallbackRoutePath 归一化单条回调路由路径。
// 空字符串表示使用默认值；非空路径必须以 /api/ 开头，不含 query string，
// 且不得与已有路由前缀冲突。
func normalizeCallbackRoutePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	// 去除 query string 和 fragment
	if idx := strings.IndexAny(path, "?#"); idx != -1 {
		path = path[:idx]
	}

	// 去除尾部斜杠
	path = strings.TrimRight(path, "/")

	// 必须以 /api/ 开头
	if !strings.HasPrefix(path, "/api/") {
		return ""
	}

	// 不得与已有路由前缀冲突
	pathWithSlash := path + "/"
	for _, prefix := range reservedRoutePrefixes {
		if strings.HasPrefix(pathWithSlash, prefix) || strings.HasPrefix(prefix, pathWithSlash) {
			return ""
		}
	}

	return path
}

// deduplicateCallbackRoutes 去除重复路径：后出现的重复路径被清空
func deduplicateCallbackRoutes(s *CallbackRoutesSetting) {
	seen := make(map[string]bool, 4)
	fields := []*string{
		&s.PaymentCallback,
		&s.DujiaoPayWebhook,
		&s.PaypalWebhook,
		&s.StripeWebhook,
		&s.UpstreamCallback,
	}
	for _, f := range fields {
		if *f == "" {
			continue
		}
		if seen[*f] {
			*f = "" // 重复路径清空
		} else {
			seen[*f] = true
		}
	}
}
