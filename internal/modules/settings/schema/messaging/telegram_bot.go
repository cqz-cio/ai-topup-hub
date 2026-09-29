package settingsmessaging

import (
	"strings"

	"github.com/dujiao-next/internal/constants"
	settingsvalue "github.com/dujiao-next/internal/modules/settings/schema/value"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

// LocalizedText 多语言文本 {"zh-CN": "...", "zh-TW": "...", "en-US": "..."}
type LocalizedText map[string]string

// TelegramBotConfigSetting Telegram Bot 配置实体（嵌套分组）
type TelegramBotConfigSetting struct {
	Enabled       bool                     `json:"enabled"`
	DefaultLocale string                   `json:"default_locale"`
	ConfigVersion int                      `json:"config_version"`
	Basic         TelegramBotBasicConfig   `json:"basic"`
	Welcome       TelegramBotWelcomeConfig `json:"welcome"`
	Help          TelegramBotHelpConfig    `json:"help"`
	Menu          TelegramBotMenuConfig    `json:"menu"`
}

// TelegramBotBasicConfig 基本信息分组
type TelegramBotBasicConfig struct {
	DisplayName string        `json:"display_name"`
	Description LocalizedText `json:"description"`
	SupportURL  string        `json:"support_url"`
	CoverURL    string        `json:"cover_url"`
}

// TelegramBotWelcomeConfig 欢迎设置分组
type TelegramBotWelcomeConfig struct {
	Enabled bool          `json:"enabled"`
	Message LocalizedText `json:"message"`
}

// TelegramBotHelpConfig 帮助中心配置分组
type TelegramBotHelpConfig struct {
	Enabled     bool                  `json:"enabled"`
	Title       LocalizedText         `json:"title"`
	Intro       LocalizedText         `json:"intro"`
	CenterHint  LocalizedText         `json:"center_hint"`
	SupportHint LocalizedText         `json:"support_hint"`
	Items       []TelegramBotHelpItem `json:"items"`
}

// TelegramBotHelpItem 单个帮助中心条目
type TelegramBotHelpItem struct {
	Key             string        `json:"key"`
	Enabled         bool          `json:"enabled"`
	Order           int           `json:"order"`
	Summary         LocalizedText `json:"summary"`
	Title           LocalizedText `json:"title"`
	Content         LocalizedText `json:"content"`
	ShowSupportLink bool          `json:"show_support_link"`
}

// TelegramBotMenuConfig 菜单配置分组
type TelegramBotMenuConfig struct {
	Items []TelegramBotMenuItem `json:"items"`
}

// TelegramBotMenuItem 单个菜单项
type TelegramBotMenuItem struct {
	Key     string                `json:"key"`
	Enabled bool                  `json:"enabled"`
	Order   int                   `json:"order"`
	Label   LocalizedText         `json:"label"`
	Action  TelegramBotMenuAction `json:"action"`
}

// TelegramBotMenuAction 菜单项动作
type TelegramBotMenuAction struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

// TelegramBotRuntimeStatusSetting Telegram Bot 运行时状态
type TelegramBotRuntimeStatusSetting struct {
	Connected        bool     `json:"connected"`
	LastSeenAt       string   `json:"last_seen_at"`
	BotVersion       string   `json:"bot_version"`
	WebhookStatus    string   `json:"webhook_status"`
	MachineCode      string   `json:"machine_code"`
	LicenseStatus    string   `json:"license_status"`
	LicenseExpiresAt string   `json:"license_expires_at"`
	Warnings         []string `json:"warnings"`
	ConfigVersion    int      `json:"config_version"`
	LastConfigSyncAt string   `json:"last_config_sync_at"`
}

// TelegramBotConfigDefault 默认 Bot 配置
func DefaultTelegramBotConfig() TelegramBotConfigSetting {
	return TelegramBotConfigSetting{
		Enabled:       false,
		DefaultLocale: "zh-CN",
		ConfigVersion: 0,
		Basic: TelegramBotBasicConfig{
			Description: make(LocalizedText),
		},
		Welcome: TelegramBotWelcomeConfig{
			Enabled: false,
			Message: make(LocalizedText),
		},
		Help: TelegramBotHelpConfig{
			Enabled: true,
			Title: LocalizedText{
				"zh-CN": "❓ 帮助中心",
				"zh-TW": "❓ 幫助中心",
				"en-US": "❓ Help Center",
			},
			Intro: LocalizedText{
				"zh-CN": "这里整理了下单、订单、钱包和客服入口，先点最接近你问题的按钮。",
				"zh-TW": "這裡整理了下單、訂單、錢包與客服入口，先點最接近你問題的按鈕。",
				"en-US": "Quick answers for shopping, orders, wallet, and support are listed here. Start with the closest topic.",
			},
			CenterHint: LocalizedText{
				"zh-CN": "如果还是没解决，再进入客服入口即可。",
				"zh-TW": "如果還是沒解決，再進入客服入口即可。",
				"en-US": "If the issue remains, open the support topic from below.",
			},
			SupportHint: LocalizedText{
				"zh-CN": "当前还没有配置客服链接，你可以先查看上面的常见问题，或稍后再试。",
				"zh-TW": "目前還沒有配置客服連結，你可以先查看上面的常見問題，或稍後再試。",
				"en-US": "The support link is not configured yet. You can review the common topics above or try again later.",
			},
			Items: []TelegramBotHelpItem{
				{
					Key:     "shop",
					Enabled: true,
					Order:   1,
					Summary: LocalizedText{"zh-CN": "🛍️ 怎么下单", "zh-TW": "🛍️ 怎麼下單", "en-US": "🛍️ How to buy"},
					Title:   LocalizedText{"zh-CN": "🛍️ 怎么下单", "zh-TW": "🛍️ 怎麼下單", "en-US": "🛍️ How to buy"},
					Content: LocalizedText{
						"zh-CN": "先点“开始购物”，进入分类后选择商品与规格，再确认数量并完成支付。支付成功后，订单会自动进入处理流程。",
						"zh-TW": "先點「開始購物」，進入分類後選擇商品與規格，再確認數量並完成付款。付款成功後，訂單會自動進入處理流程。",
						"en-US": "Tap \"Shop Now\", choose a category, pick the product and spec, confirm quantity, then finish payment. Your order enters processing right after payment succeeds.",
					},
				},
				{
					Key:     "orders",
					Enabled: true,
					Order:   2,
					Summary: LocalizedText{"zh-CN": "📦 订单问题", "zh-TW": "📦 訂單問題", "en-US": "📦 Order issues"},
					Title:   LocalizedText{"zh-CN": "📦 订单问题", "zh-TW": "📦 訂單問題", "en-US": "📦 Order issues"},
					Content: LocalizedText{
						"zh-CN": "在“我的订单”里可以查看状态、支付结果与发货内容。若支付完成但暂时没发货，先刷新订单状态；仍有问题再联系人工客服。",
						"zh-TW": "在「我的訂單」裡可以查看狀態、付款結果與發貨內容。若付款完成但暫時沒發貨，先刷新訂單狀態；仍有問題再聯繫人工客服。",
						"en-US": "Use \"My Orders\" to review status, payment result, and delivery content. If payment is done but delivery is pending, refresh the order status first and contact support if it still looks wrong.",
					},
				},
				{
					Key:     "wallet",
					Enabled: true,
					Order:   3,
					Summary: LocalizedText{"zh-CN": "💰 钱包充值", "zh-TW": "💰 錢包儲值", "en-US": "💰 Wallet help"},
					Title:   LocalizedText{"zh-CN": "💰 钱包充值", "zh-TW": "💰 錢包儲值", "en-US": "💰 Wallet help"},
					Content: LocalizedText{
						"zh-CN": "打开“我的钱包”可以查看余额、充值记录并发起充值。充值成功后，余额会更新，可直接用于支付订单。",
						"zh-TW": "打開「我的錢包」可以查看餘額、儲值記錄並發起儲值。儲值成功後，餘額會更新，可直接用於支付訂單。",
						"en-US": "Open \"My Wallet\" to view balance, recharge history, and create a recharge. Once the recharge succeeds, the balance updates and can be used for orders.",
					},
				},
				{
					Key:             "support",
					Enabled:         true,
					Order:           4,
					ShowSupportLink: true,
					Summary:         LocalizedText{"zh-CN": "💬 联系客服", "zh-TW": "💬 聯繫客服", "en-US": "💬 Contact support"},
					Title:           LocalizedText{"zh-CN": "💬 联系客服", "zh-TW": "💬 聯繫客服", "en-US": "💬 Contact support"},
					Content: LocalizedText{
						"zh-CN": "如果上面的自助说明仍然无法解决问题，请通过下方客服入口联系人工，并尽量附上订单号、商品名和问题截图。",
						"zh-TW": "如果上面的自助說明仍然無法解決問題，請透過下方客服入口聯繫人工，並盡量附上訂單號、商品名與問題截圖。",
						"en-US": "If the self-service guides above do not solve the issue, contact support using the link below and include your order number, product name, and screenshots when possible.",
					},
				},
			},
		},
		Menu: TelegramBotMenuConfig{
			Items: defaultBuiltinMenuItems(),
		},
	}
}

// TelegramBotRuntimeStatusDefault 默认运行时状态
func DefaultTelegramBotRuntimeStatus() TelegramBotRuntimeStatusSetting {
	return TelegramBotRuntimeStatusSetting{
		Connected:     false,
		ConfigVersion: 0,
		Warnings:      []string{},
	}
}

// TelegramBotConfigToMap 转换为 settings 存储结构
func EncodeTelegramBotConfig(setting TelegramBotConfigSetting) map[string]interface{} {
	return map[string]interface{}{
		"enabled":        setting.Enabled,
		"default_locale": strings.TrimSpace(setting.DefaultLocale),
		"config_version": setting.ConfigVersion,
		"basic": map[string]interface{}{
			"display_name": strings.TrimSpace(setting.Basic.DisplayName),
			"description":  localizedTextToMap(setting.Basic.Description),
			"support_url":  strings.TrimSpace(setting.Basic.SupportURL),
			"cover_url":    strings.TrimSpace(setting.Basic.CoverURL),
		},
		"welcome": map[string]interface{}{
			"enabled": setting.Welcome.Enabled,
			"message": localizedTextToMap(setting.Welcome.Message),
		},
		"help": map[string]interface{}{
			"enabled":      setting.Help.Enabled,
			"title":        localizedTextToMap(setting.Help.Title),
			"intro":        localizedTextToMap(setting.Help.Intro),
			"center_hint":  localizedTextToMap(setting.Help.CenterHint),
			"support_hint": localizedTextToMap(setting.Help.SupportHint),
			"items":        helpItemsToSlice(setting.Help.Items),
		},
		"menu": map[string]interface{}{
			"items": menuItemsToSlice(setting.Menu.Items),
		},
	}
}

// MaskTelegramBotConfigForAdmin 返回管理端配置
func MaskTelegramBotConfigForAdmin(setting TelegramBotConfigSetting) jsonmap.JSON {
	return jsonmap.JSON{
		"enabled":        setting.Enabled,
		"default_locale": setting.DefaultLocale,
		"config_version": setting.ConfigVersion,
		"basic": map[string]interface{}{
			"display_name": setting.Basic.DisplayName,
			"description":  localizedTextToMap(setting.Basic.Description),
			"support_url":  setting.Basic.SupportURL,
			"cover_url":    setting.Basic.CoverURL,
		},
		"welcome": map[string]interface{}{
			"enabled": setting.Welcome.Enabled,
			"message": localizedTextToMap(setting.Welcome.Message),
		},
		"help": map[string]interface{}{
			"enabled":      setting.Help.Enabled,
			"title":        localizedTextToMap(setting.Help.Title),
			"intro":        localizedTextToMap(setting.Help.Intro),
			"center_hint":  localizedTextToMap(setting.Help.CenterHint),
			"support_hint": localizedTextToMap(setting.Help.SupportHint),
			"items":        helpItemsToSlice(setting.Help.Items),
		},
		"menu": map[string]interface{}{
			"items": menuItemsToSlice(setting.Menu.Items),
		},
	}
}

// SerializeTelegramBotConfigForChannel 返回 Channel API 配置（bot_token 由调用方注入）
func SerializeTelegramBotConfigForChannel(setting TelegramBotConfigSetting, botToken string) jsonmap.JSON {
	return jsonmap.JSON{
		"enabled":        setting.Enabled,
		"bot_token":      botToken,
		"default_locale": setting.DefaultLocale,
		"config_version": setting.ConfigVersion,
		"basic": map[string]interface{}{
			"display_name": setting.Basic.DisplayName,
			"description":  localizedTextToMap(setting.Basic.Description),
			"support_url":  setting.Basic.SupportURL,
			"cover_url":    setting.Basic.CoverURL,
		},
		"welcome": map[string]interface{}{
			"enabled": setting.Welcome.Enabled,
			"message": localizedTextToMap(setting.Welcome.Message),
		},
		"help": map[string]interface{}{
			"enabled":      setting.Help.Enabled,
			"title":        localizedTextToMap(setting.Help.Title),
			"intro":        localizedTextToMap(setting.Help.Intro),
			"center_hint":  localizedTextToMap(setting.Help.CenterHint),
			"support_hint": localizedTextToMap(setting.Help.SupportHint),
			"items":        helpItemsToSlice(setting.Help.Items),
		},
		"menu": map[string]interface{}{
			"items": menuItemsToSlice(setting.Menu.Items),
		},
	}
}

// TelegramBotRuntimeStatusToMap 转换运行时状态为存储结构
func EncodeTelegramBotRuntimeStatus(status TelegramBotRuntimeStatusSetting) map[string]interface{} {
	return map[string]interface{}{
		"connected":           status.Connected,
		"last_seen_at":        status.LastSeenAt,
		"bot_version":         status.BotVersion,
		"webhook_status":      status.WebhookStatus,
		"machine_code":        status.MachineCode,
		"license_status":      status.LicenseStatus,
		"license_expires_at":  status.LicenseExpiresAt,
		"warnings":            append([]string(nil), status.Warnings...),
		"config_version":      status.ConfigVersion,
		"last_config_sync_at": status.LastConfigSyncAt,
	}
}

// telegramBotConfigFromJSON 从 JSON 读取嵌套结构，兼容旧扁平格式
func DecodeTelegramBotConfig(raw jsonmap.JSON, fallback TelegramBotConfigSetting) TelegramBotConfigSetting {
	next := fallback
	if raw == nil {
		return next
	}

	// 兼容旧扁平格式：检测 bot_display_name 字段自动迁移
	if _, hasOldField := raw["bot_display_name"]; hasOldField {
		return migrateOldTelegramBotConfig(raw, fallback)
	}

	next.Enabled = settingsvalue.ReadBool(raw, "enabled", next.Enabled)
	next.DefaultLocale = settingsvalue.ReadString(raw, "default_locale", next.DefaultLocale)
	next.ConfigVersion = settingsvalue.ReadInt(raw, "config_version", next.ConfigVersion)

	if basicRaw, ok := raw["basic"].(map[string]interface{}); ok {
		next.Basic.DisplayName = settingsvalue.ReadString(basicRaw, "display_name", next.Basic.DisplayName)
		next.Basic.Description = readLocalizedText(basicRaw, "description", next.Basic.Description)
		next.Basic.SupportURL = settingsvalue.ReadString(basicRaw, "support_url", next.Basic.SupportURL)
		next.Basic.CoverURL = settingsvalue.ReadString(basicRaw, "cover_url", next.Basic.CoverURL)
	}

	if welcomeRaw, ok := raw["welcome"].(map[string]interface{}); ok {
		next.Welcome.Enabled = settingsvalue.ReadBool(welcomeRaw, "enabled", next.Welcome.Enabled)
		next.Welcome.Message = readLocalizedText(welcomeRaw, "message", next.Welcome.Message)
	}

	if helpRaw, ok := raw["help"].(map[string]interface{}); ok {
		next.Help.Enabled = settingsvalue.ReadBool(helpRaw, "enabled", next.Help.Enabled)
		next.Help.Title = readLocalizedText(helpRaw, "title", next.Help.Title)
		next.Help.Intro = readLocalizedText(helpRaw, "intro", next.Help.Intro)
		next.Help.CenterHint = readLocalizedText(helpRaw, "center_hint", next.Help.CenterHint)
		next.Help.SupportHint = readLocalizedText(helpRaw, "support_hint", next.Help.SupportHint)
		next.Help.Items = readHelpItems(helpRaw["items"], next.Help.Items)
	}

	if menuRaw, ok := raw["menu"].(map[string]interface{}); ok {
		next.Menu.Items = readMenuItems(menuRaw["items"])
	}

	return next
}

// migrateOldTelegramBotConfig 将旧扁平格式迁移为嵌套结构
func migrateOldTelegramBotConfig(raw jsonmap.JSON, fallback TelegramBotConfigSetting) TelegramBotConfigSetting {
	next := fallback
	defaultLocale := settingsvalue.ReadString(raw, "default_locale", "zh-CN")
	next.DefaultLocale = defaultLocale

	next.Basic.DisplayName = settingsvalue.ReadString(raw, "bot_display_name", "")
	// 旧格式的单语言字段迁移到 default_locale
	oldDescription := settingsvalue.ReadString(raw, "bot_description", "")
	if oldDescription != "" {
		next.Basic.Description = LocalizedText{defaultLocale: oldDescription}
	}
	next.Basic.SupportURL = settingsvalue.ReadString(raw, "support_link", "")
	next.Basic.CoverURL = settingsvalue.ReadString(raw, "welcome_cover_url", "")

	oldWelcomeMessage := settingsvalue.ReadString(raw, "welcome_message", "")
	if oldWelcomeMessage != "" {
		next.Welcome.Enabled = true
		next.Welcome.Message = LocalizedText{defaultLocale: oldWelcomeMessage}
	}

	return next
}

func DecodeTelegramBotRuntimeStatus(raw jsonmap.JSON, fallback TelegramBotRuntimeStatusSetting) TelegramBotRuntimeStatusSetting {
	next := fallback
	if raw == nil {
		return next
	}
	next.Connected = settingsvalue.ReadBool(raw, "connected", next.Connected)
	next.LastSeenAt = settingsvalue.ReadString(raw, "last_seen_at", next.LastSeenAt)
	next.BotVersion = settingsvalue.ReadString(raw, "bot_version", next.BotVersion)
	next.WebhookStatus = settingsvalue.ReadString(raw, "webhook_status", next.WebhookStatus)
	next.MachineCode = settingsvalue.ReadString(raw, "machine_code", next.MachineCode)
	next.LicenseStatus = settingsvalue.ReadString(raw, "license_status", next.LicenseStatus)
	next.LicenseExpiresAt = settingsvalue.ReadString(raw, "license_expires_at", next.LicenseExpiresAt)
	next.Warnings = settingsvalue.ReadStringList(raw, "warnings", next.Warnings)
	next.ConfigVersion = settingsvalue.ReadInt(raw, "config_version", next.ConfigVersion)
	next.LastConfigSyncAt = settingsvalue.ReadString(raw, "last_config_sync_at", next.LastConfigSyncAt)
	return next
}

// normalizeTelegramBotConfig 归一化多语言字段 + trim
func normalizeTelegramBotConfigMap(raw jsonmap.JSON) map[string]interface{} {
	setting := DecodeTelegramBotConfig(raw, DefaultTelegramBotConfig())
	// 归一化多语言字段：确保所有支持的语言键都存在
	setting.Basic.Description = NormalizeLocalizedText(setting.Basic.Description)
	setting.Welcome.Message = NormalizeLocalizedText(setting.Welcome.Message)
	setting.Help.Title = NormalizeLocalizedText(setting.Help.Title)
	setting.Help.Intro = NormalizeLocalizedText(setting.Help.Intro)
	setting.Help.CenterHint = NormalizeLocalizedText(setting.Help.CenterHint)
	setting.Help.SupportHint = NormalizeLocalizedText(setting.Help.SupportHint)
	setting.Help.Items = NormalizeHelpItems(setting.Help.Items)
	setting.Menu.Items = NormalizeTelegramBotMenuItems(setting.Menu.Items)
	return EncodeTelegramBotConfig(setting)
}

// validMenuActionTypes 菜单项 action type 白名单
var validMenuActionTypes = map[string]bool{
	"builtin": true,
	"url":     true,
	"web_app": true,
	"command": true,
}

const helpItemsMaxCount = 12
const menuItemsMaxCount = 20

// readHelpItems 从 JSON 解析帮助中心条目数组
func readHelpItems(raw interface{}, fallback []TelegramBotHelpItem) []TelegramBotHelpItem {
	arr, ok := raw.([]interface{})
	if !ok {
		return fallback
	}
	if len(arr) == 0 {
		return []TelegramBotHelpItem{}
	}
	items := make([]TelegramBotHelpItem, 0, len(arr))
	for _, v := range arr {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		items = append(items, TelegramBotHelpItem{
			Key:             settingsvalue.ReadString(m, "key", ""),
			Enabled:         settingsvalue.ReadBool(m, "enabled", true),
			Order:           settingsvalue.ReadInt(m, "order", 0),
			Summary:         readLocalizedText(m, "summary", make(LocalizedText)),
			Title:           readLocalizedText(m, "title", make(LocalizedText)),
			Content:         readLocalizedText(m, "content", make(LocalizedText)),
			ShowSupportLink: settingsvalue.ReadBool(m, "show_support_link", false),
		})
	}
	return items
}

// helpItemsToSlice 序列化帮助中心条目为存储格式
func helpItemsToSlice(items []TelegramBotHelpItem) []interface{} {
	result := make([]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{
			"key":               strings.TrimSpace(item.Key),
			"enabled":           item.Enabled,
			"order":             item.Order,
			"summary":           localizedTextToMap(item.Summary),
			"title":             localizedTextToMap(item.Title),
			"content":           localizedTextToMap(item.Content),
			"show_support_link": item.ShowSupportLink,
		})
	}
	return result
}

// readMenuItems 从 JSON 解析菜单项数组
func readMenuItems(raw interface{}) []TelegramBotMenuItem {
	arr, ok := raw.([]interface{})
	if !ok || len(arr) == 0 {
		return []TelegramBotMenuItem{}
	}
	items := make([]TelegramBotMenuItem, 0, len(arr))
	for _, v := range arr {
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		item := TelegramBotMenuItem{
			Key:     settingsvalue.ReadString(m, "key", ""),
			Enabled: settingsvalue.ReadBool(m, "enabled", true),
			Order:   settingsvalue.ReadInt(m, "order", 0),
			Label:   readLocalizedText(m, "label", make(LocalizedText)),
		}
		if actionRaw, ok := m["action"].(map[string]interface{}); ok {
			item.Action.Type = settingsvalue.ReadString(actionRaw, "type", "builtin")
			item.Action.Value = settingsvalue.ReadString(actionRaw, "value", "")
		}
		items = append(items, item)
	}
	return items
}

// menuItemsToSlice 序列化菜单项为存储格式
func menuItemsToSlice(items []TelegramBotMenuItem) []interface{} {
	result := make([]interface{}, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]interface{}{
			"key":     strings.TrimSpace(item.Key),
			"enabled": item.Enabled,
			"order":   item.Order,
			"label":   localizedTextToMap(item.Label),
			"action": map[string]interface{}{
				"type":  strings.TrimSpace(item.Action.Type),
				"value": strings.TrimSpace(item.Action.Value),
			},
		})
	}
	return result
}

// NormalizeHelpItems 归一化帮助中心条目：trim、多语言归一化、上限 12 项
func NormalizeHelpItems(items []TelegramBotHelpItem) []TelegramBotHelpItem {
	if len(items) > helpItemsMaxCount {
		items = items[:helpItemsMaxCount]
	}
	result := make([]TelegramBotHelpItem, 0, len(items))
	for _, item := range items {
		item.Key = strings.TrimSpace(item.Key)
		item.Summary = NormalizeLocalizedText(item.Summary)
		item.Title = NormalizeLocalizedText(item.Title)
		item.Content = NormalizeLocalizedText(item.Content)
		result = append(result, item)
	}
	return result
}

// BuiltinTelegramBotMenuKeysOrder 内置菜单 key 显示顺序，必须与 telegram-bot 端 builtinMenuButtons 保持一致。
// 后台 default seed、补齐缺失项时均按该顺序生成。
var BuiltinTelegramBotMenuKeysOrder = []string{
	"shop_home",
	"my_orders",
	"my_wallet",
	"affiliate",
	"gift_card",
	"switch_language",
	"contact_support",
}

// builtinMenuLabels 内置菜单的多语言默认 label（与 telegram-bot 的 i18n 默认值一致）。
var builtinMenuLabels = map[string]LocalizedText{
	"shop_home":       {"zh-CN": "🛍️ 开始购物", "zh-TW": "🛍️ 開始購物", "en-US": "🛍️ Shop Now"},
	"my_orders":       {"zh-CN": "📦 我的订单", "zh-TW": "📦 我的訂單", "en-US": "📦 My Orders"},
	"my_wallet":       {"zh-CN": "💰 我的钱包", "zh-TW": "💰 我的錢包", "en-US": "💰 My Wallet"},
	"affiliate":       {"zh-CN": "📣 推广返利", "zh-TW": "📣 推廣返利", "en-US": "📣 Affiliate"},
	"gift_card":       {"zh-CN": "🎁 礼品卡兑换", "zh-TW": "🎁 禮品卡兌換", "en-US": "🎁 Redeem Gift Card"},
	"switch_language": {"zh-CN": "🌐 切换语言", "zh-TW": "🌐 切換語言", "en-US": "🌐 Language"},
	"contact_support": {"zh-CN": "❓ 帮助中心", "zh-TW": "❓ 幫助中心", "en-US": "❓ Help"},
}

// defaultBuiltinMenuItems 返回 7 项内置菜单的默认配置（用于 default seed 与缺失补齐）。
func defaultBuiltinMenuItems() []TelegramBotMenuItem {
	items := make([]TelegramBotMenuItem, 0, len(BuiltinTelegramBotMenuKeysOrder))
	for i, key := range BuiltinTelegramBotMenuKeysOrder {
		labelSrc := builtinMenuLabels[key]
		label := make(LocalizedText, len(labelSrc))
		for k, v := range labelSrc {
			label[k] = v
		}
		items = append(items, TelegramBotMenuItem{
			Key:     key,
			Enabled: true,
			Order:   i + 1,
			Label:   label,
			Action:  TelegramBotMenuAction{Type: "builtin", Value: ""},
		})
	}
	return items
}

// EnsureBuiltinMenuItems 补齐缺失的内置菜单 key（保留已有项的 enabled/label/order）。
// 这样后台 UI 总能看到 7 个内置菜单的开关，避免老库数据缺项导致管理员误以为某些菜单"无法配置"。
func EnsureBuiltinMenuItems(items []TelegramBotMenuItem) []TelegramBotMenuItem {
	seen := make(map[string]bool, len(items))
	maxOrder := 0
	for _, it := range items {
		seen[it.Key] = true
		if it.Order > maxOrder {
			maxOrder = it.Order
		}
	}
	for _, key := range BuiltinTelegramBotMenuKeysOrder {
		if seen[key] {
			continue
		}
		maxOrder++
		labelSrc := builtinMenuLabels[key]
		label := make(LocalizedText, len(labelSrc))
		for k, v := range labelSrc {
			label[k] = v
		}
		items = append(items, TelegramBotMenuItem{
			Key:     key,
			Enabled: true,
			Order:   maxOrder,
			Label:   label,
			Action:  TelegramBotMenuAction{Type: "builtin", Value: ""},
		})
	}
	return items
}

// NormalizeTelegramBotMenuItems 归一化菜单项：trim、归一化 label、验证 action type、上限 20 项；最后补齐内置 key
func NormalizeTelegramBotMenuItems(items []TelegramBotMenuItem) []TelegramBotMenuItem {
	if len(items) > menuItemsMaxCount {
		items = items[:menuItemsMaxCount]
	}
	result := make([]TelegramBotMenuItem, 0, len(items)+len(BuiltinTelegramBotMenuKeysOrder))
	for _, item := range items {
		item.Key = strings.TrimSpace(item.Key)
		item.Label = NormalizeLocalizedText(item.Label)
		item.Action.Type = strings.TrimSpace(item.Action.Type)
		item.Action.Value = strings.TrimSpace(item.Action.Value)
		if !validMenuActionTypes[item.Action.Type] {
			item.Action.Type = "builtin"
		}
		result = append(result, item)
	}
	return EnsureBuiltinMenuItems(result)
}

// readLocalizedText 从 JSON map 读取 LocalizedText 字段
func readLocalizedText(source map[string]interface{}, key string, fallback LocalizedText) LocalizedText {
	raw, ok := source[key]
	if !ok {
		return fallback
	}
	mapRaw, ok := raw.(map[string]interface{})
	if !ok {
		return fallback
	}
	result := make(LocalizedText, len(mapRaw))
	for k, v := range mapRaw {
		if s, ok := v.(string); ok {
			result[k] = strings.TrimSpace(s)
		}
	}
	if len(result) == 0 {
		return fallback
	}
	return result
}

// localizedTextToMap 将 LocalizedText 转换为 map[string]interface{}
func localizedTextToMap(lt LocalizedText) map[string]interface{} {
	result := make(map[string]interface{}, len(lt))
	for k, v := range lt {
		result[k] = v
	}
	return result
}

// NormalizeLocalizedText 确保所有支持的语言键都存在并 trim
func NormalizeLocalizedText(lt LocalizedText) LocalizedText {
	result := make(LocalizedText, len(constants.SupportedLocales))
	for _, lang := range constants.SupportedLocales {
		result[lang] = ""
	}
	for k, v := range lt {
		result[k] = strings.TrimSpace(v)
	}
	return result
}

// NormalizeTelegramBotConfigJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeTelegramBotConfigJSON(raw jsonmap.JSON) jsonmap.JSON {
	return jsonmap.JSON(normalizeTelegramBotConfigMap(raw))
}
