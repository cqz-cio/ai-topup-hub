package settingssecurity

import (
	"encoding/json"
	"net"
	"strings"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

const orderRiskControlVersion = 2

// OrderRateLimitConfig 下单频率限制配置。
type OrderRateLimitConfig struct {
	Enabled       bool `json:"enabled"`
	WindowSeconds int  `json:"window_seconds"`
	MaxRequests   int  `json:"max_requests"`
	BlockSeconds  int  `json:"block_seconds"`
}

// OrderRiskCommonPolicy 保存与购买身份无关的订单风控策略。
type OrderRiskCommonPolicy struct {
	IPBlacklist []string `json:"ip_blacklist"`
}

// OrderRiskGuestPolicy 保存以可信客户端 IP 为主体的游客库存保护策略。
type OrderRiskGuestPolicy struct {
	Enabled                        bool                 `json:"enabled"`
	MaxPendingOrdersPerIP          int                  `json:"max_pending_orders_per_ip"`
	MaxQuantityPerProductPerOrder  int                  `json:"max_quantity_per_product_per_order"`
	MaxPendingQuantityPerIPProduct int                  `json:"max_pending_quantity_per_ip_product"`
	PaymentExpireMinutes           int                  `json:"payment_expire_minutes"`
	RateLimit                      OrderRateLimitConfig `json:"rate_limit"`
}

// OrderRiskMemberPolicy 保存以用户 ID 为主体、默认更宽松的会员策略。
type OrderRiskMemberPolicy struct {
	Enabled                       bool                 `json:"enabled"`
	MaxPendingOrdersPerUser       int                  `json:"max_pending_orders_per_user"`
	MaxPendingOrdersPerIP         int                  `json:"max_pending_orders_per_ip"`
	MaxQuantityPerProductPerOrder int                  `json:"max_quantity_per_product_per_order"`
	RateLimit                     OrderRateLimitConfig `json:"rate_limit"`
}

// OrderRiskControlConfig 订单风控配置。游客邮箱仅用于订单业务，不作为风控身份。
type OrderRiskControlConfig struct {
	Version int                   `json:"version"`
	Enabled bool                  `json:"enabled"`
	Common  OrderRiskCommonPolicy `json:"common"`
	Guest   OrderRiskGuestPolicy  `json:"guest"`
	Member  OrderRiskMemberPolicy `json:"member"`
}

// DefaultOrderRiskControlConfig 返回新安装推荐值；总开关默认关闭，避免静默改变订单行为。
func DefaultOrderRiskControlConfig() OrderRiskControlConfig {
	return OrderRiskControlConfig{
		Version: orderRiskControlVersion,
		Enabled: false,
		Common:  OrderRiskCommonPolicy{IPBlacklist: []string{}},
		Guest: OrderRiskGuestPolicy{
			Enabled:                        true,
			MaxPendingOrdersPerIP:          2,
			MaxQuantityPerProductPerOrder:  1,
			MaxPendingQuantityPerIPProduct: 2,
			PaymentExpireMinutes:           10,
			RateLimit: OrderRateLimitConfig{
				Enabled:       true,
				WindowSeconds: 60,
				MaxRequests:   3,
				BlockSeconds:  120,
			},
		},
		Member: OrderRiskMemberPolicy{
			Enabled:                       true,
			MaxPendingOrdersPerUser:       5,
			MaxPendingOrdersPerIP:         0,
			MaxQuantityPerProductPerOrder: 0,
			RateLimit: OrderRateLimitConfig{
				Enabled:       false,
				WindowSeconds: 60,
				MaxRequests:   10,
				BlockSeconds:  120,
			},
		},
	}
}

// NormalizeOrderRiskControlConfig 归一化风控配置；0 始终表示对应限制关闭。
func NormalizeOrderRiskControlConfig(cfg OrderRiskControlConfig) OrderRiskControlConfig {
	defaults := DefaultOrderRiskControlConfig()
	cfg.Version = orderRiskControlVersion
	cfg.Guest.MaxPendingOrdersPerIP = normalizeRiskLimit(cfg.Guest.MaxPendingOrdersPerIP, 100, defaults.Guest.MaxPendingOrdersPerIP)
	cfg.Guest.MaxQuantityPerProductPerOrder = normalizeRiskLimit(cfg.Guest.MaxQuantityPerProductPerOrder, 100000, defaults.Guest.MaxQuantityPerProductPerOrder)
	cfg.Guest.MaxPendingQuantityPerIPProduct = normalizeRiskLimit(cfg.Guest.MaxPendingQuantityPerIPProduct, 100000, defaults.Guest.MaxPendingQuantityPerIPProduct)
	cfg.Guest.PaymentExpireMinutes = normalizeRiskLimit(cfg.Guest.PaymentExpireMinutes, 10080, defaults.Guest.PaymentExpireMinutes)
	cfg.Guest.RateLimit = normalizeRateLimit(cfg.Guest.RateLimit, defaults.Guest.RateLimit)

	cfg.Member.MaxPendingOrdersPerUser = normalizeRiskLimit(cfg.Member.MaxPendingOrdersPerUser, 100, defaults.Member.MaxPendingOrdersPerUser)
	cfg.Member.MaxPendingOrdersPerIP = normalizeRiskLimit(cfg.Member.MaxPendingOrdersPerIP, 100, defaults.Member.MaxPendingOrdersPerIP)
	cfg.Member.MaxQuantityPerProductPerOrder = normalizeRiskLimit(cfg.Member.MaxQuantityPerProductPerOrder, 100000, defaults.Member.MaxQuantityPerProductPerOrder)
	cfg.Member.RateLimit = normalizeRateLimit(cfg.Member.RateLimit, defaults.Member.RateLimit)

	cleanIPs := make([]string, 0, len(cfg.Common.IPBlacklist))
	seen := make(map[string]struct{}, len(cfg.Common.IPBlacklist))
	for _, raw := range cfg.Common.IPBlacklist {
		entry := strings.TrimSpace(raw)
		if entry == "" || !isValidIPOrCIDR(entry) {
			continue
		}
		if _, exists := seen[entry]; exists {
			continue
		}
		seen[entry] = struct{}{}
		cleanIPs = append(cleanIPs, entry)
	}
	cfg.Common.IPBlacklist = cleanIPs
	return cfg
}

func normalizeRiskLimit(value, maximum, fallback int) int {
	if value < 0 || value > maximum {
		return fallback
	}
	return value
}

func normalizeRateLimit(cfg, fallback OrderRateLimitConfig) OrderRateLimitConfig {
	if cfg.WindowSeconds < 10 || cfg.WindowSeconds > 3600 {
		cfg.WindowSeconds = fallback.WindowSeconds
	}
	if cfg.MaxRequests < 1 || cfg.MaxRequests > 100 {
		cfg.MaxRequests = fallback.MaxRequests
	}
	if cfg.BlockSeconds < 0 || cfg.BlockSeconds > 86400 {
		cfg.BlockSeconds = fallback.BlockSeconds
	}
	return cfg
}

func isValidIPOrCIDR(value string) bool {
	if net.ParseIP(value) != nil {
		return true
	}
	_, _, err := net.ParseCIDR(value)
	return err == nil
}

// DecodeOrderRiskControlConfig 解析新版嵌套配置，并把旧版扁平配置一次性映射到游客/会员策略。
func DecodeOrderRiskControlConfig(raw jsonmap.JSON, fallback OrderRiskControlConfig) OrderRiskControlConfig {
	if raw == nil {
		return NormalizeOrderRiskControlConfig(fallback)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return NormalizeOrderRiskControlConfig(fallback)
	}
	if _, hasGuest := raw["guest"]; hasGuest {
		result := fallback
		if err := json.Unmarshal(data, &result); err != nil {
			return NormalizeOrderRiskControlConfig(fallback)
		}
		return NormalizeOrderRiskControlConfig(result)
	}

	type flatRateLimitConfig struct {
		Enabled       *bool `json:"enabled"`
		WindowSeconds *int  `json:"window_seconds"`
		MaxRequests   *int  `json:"max_requests"`
		BlockSeconds  *int  `json:"block_seconds"`
	}
	type flatOrderRiskConfig struct {
		Enabled                 *bool                `json:"enabled"`
		MaxPendingOrdersPerUser *int                 `json:"max_pending_orders_per_user"`
		MaxPendingOrdersPerIP   *int                 `json:"max_pending_orders_per_ip"`
		OrderRateLimit          *flatRateLimitConfig `json:"order_rate_limit"`
		IPBlacklist             []string             `json:"ip_blacklist"`
	}
	var old flatOrderRiskConfig
	if err := json.Unmarshal(data, &old); err != nil {
		return NormalizeOrderRiskControlConfig(fallback)
	}
	result := fallback
	// 先恢复旧版缺省值，再叠加持久化字段。这样旧的稀疏 JSON 也不会误用 v2 推荐值。
	result.Guest.MaxPendingOrdersPerIP = 5
	result.Member.MaxPendingOrdersPerUser = 3
	result.Member.MaxPendingOrdersPerIP = 5
	result.Guest.MaxQuantityPerProductPerOrder = 0
	result.Guest.MaxPendingQuantityPerIPProduct = 0
	result.Guest.PaymentExpireMinutes = 0
	result.Member.MaxQuantityPerProductPerOrder = 0
	legacyRateLimit := OrderRateLimitConfig{Enabled: false, WindowSeconds: 60, MaxRequests: 5, BlockSeconds: 120}
	result.Guest.RateLimit = legacyRateLimit
	result.Member.RateLimit = legacyRateLimit
	if old.Enabled != nil {
		result.Enabled = *old.Enabled
	}
	if old.MaxPendingOrdersPerUser != nil {
		result.Member.MaxPendingOrdersPerUser = *old.MaxPendingOrdersPerUser
	}
	if old.MaxPendingOrdersPerIP != nil {
		result.Guest.MaxPendingOrdersPerIP = *old.MaxPendingOrdersPerIP
		result.Member.MaxPendingOrdersPerIP = *old.MaxPendingOrdersPerIP
	}
	if old.OrderRateLimit != nil {
		if old.OrderRateLimit.Enabled != nil {
			legacyRateLimit.Enabled = *old.OrderRateLimit.Enabled
		}
		if old.OrderRateLimit.WindowSeconds != nil {
			legacyRateLimit.WindowSeconds = *old.OrderRateLimit.WindowSeconds
		}
		if old.OrderRateLimit.MaxRequests != nil {
			legacyRateLimit.MaxRequests = *old.OrderRateLimit.MaxRequests
		}
		if old.OrderRateLimit.BlockSeconds != nil {
			legacyRateLimit.BlockSeconds = *old.OrderRateLimit.BlockSeconds
		}
		result.Guest.RateLimit = legacyRateLimit
		result.Member.RateLimit = legacyRateLimit
	}
	if old.IPBlacklist != nil {
		result.Common.IPBlacklist = old.IPBlacklist
	}
	return NormalizeOrderRiskControlConfig(result)
}

func EncodeOrderRiskControlConfig(cfg OrderRiskControlConfig) jsonmap.JSON {
	normalized := NormalizeOrderRiskControlConfig(cfg)
	data, err := json.Marshal(normalized)
	if err != nil {
		return jsonmap.JSON{}
	}
	var result jsonmap.JSON
	_ = json.Unmarshal(data, &result)
	return result
}

// NormalizeOrderRiskControlConfigJSON 是 Registry 使用的原始 JSON 写入策略。
func NormalizeOrderRiskControlConfigJSON(value jsonmap.JSON) jsonmap.JSON {
	return EncodeOrderRiskControlConfig(DecodeOrderRiskControlConfig(value, DefaultOrderRiskControlConfig()))
}
