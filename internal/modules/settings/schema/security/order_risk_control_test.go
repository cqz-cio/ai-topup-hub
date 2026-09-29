package settingssecurity

import (
	"testing"

	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestNormalizeOrderRiskControlConfig_Defaults(t *testing.T) {
	cfg := NormalizeOrderRiskControlConfig(OrderRiskControlConfig{
		Guest: OrderRiskGuestPolicy{
			MaxPendingOrdersPerIP:         0,
			MaxQuantityPerProductPerOrder: 0,
		},
		Member: OrderRiskMemberPolicy{MaxPendingOrdersPerUser: 0},
	})
	if cfg.Guest.MaxPendingOrdersPerIP != 0 || cfg.Guest.MaxQuantityPerProductPerOrder != 0 {
		t.Fatalf("zero guest limits should be preserved, got %+v", cfg.Guest)
	}
	if cfg.Member.MaxPendingOrdersPerUser != 0 {
		t.Fatalf("zero member limit should be preserved, got %+v", cfg.Member)
	}
}

func TestNormalizeOrderRiskControlConfig_ClampValues(t *testing.T) {
	cfg := NormalizeOrderRiskControlConfig(OrderRiskControlConfig{
		Guest: OrderRiskGuestPolicy{
			MaxPendingOrdersPerIP:          200,
			MaxQuantityPerProductPerOrder:  -1,
			MaxPendingQuantityPerIPProduct: 100001,
			PaymentExpireMinutes:           20000,
			RateLimit: OrderRateLimitConfig{
				WindowSeconds: 5,
				MaxRequests:   0,
				BlockSeconds:  -100,
			},
		},
		Member: OrderRiskMemberPolicy{MaxPendingOrdersPerUser: -1},
	})
	if cfg.Guest.MaxPendingOrdersPerIP != 2 {
		t.Fatalf("expected invalid guest IP limit to use default 2, got %d", cfg.Guest.MaxPendingOrdersPerIP)
	}
	if cfg.Guest.MaxQuantityPerProductPerOrder != 1 {
		t.Fatalf("expected invalid guest quantity limit to use default 1, got %d", cfg.Guest.MaxQuantityPerProductPerOrder)
	}
	if cfg.Guest.MaxPendingQuantityPerIPProduct != 2 {
		t.Fatalf("expected invalid pending product quantity to use default 2, got %d", cfg.Guest.MaxPendingQuantityPerIPProduct)
	}
	if cfg.Guest.PaymentExpireMinutes != 10 {
		t.Fatalf("expected invalid guest expiry to use default 10, got %d", cfg.Guest.PaymentExpireMinutes)
	}
	if cfg.Guest.RateLimit.WindowSeconds != 60 || cfg.Guest.RateLimit.MaxRequests != 3 || cfg.Guest.RateLimit.BlockSeconds != 120 {
		t.Fatalf("unexpected normalized guest rate limit: %+v", cfg.Guest.RateLimit)
	}
	if cfg.Member.MaxPendingOrdersPerUser != 5 {
		t.Fatalf("expected invalid member pending limit to use default 5, got %d", cfg.Member.MaxPendingOrdersPerUser)
	}
}

func TestNormalizeOrderRiskControlConfig_IPValidation(t *testing.T) {
	cfg := NormalizeOrderRiskControlConfig(OrderRiskControlConfig{
		Common: OrderRiskCommonPolicy{
			IPBlacklist: []string{
				"1.2.3.4",         // valid IP
				"10.0.0.0/8",      // valid CIDR
				"invalid_ip",      // invalid - should be removed
				"",                // empty - should be removed
				"  192.168.1.1  ", // valid with whitespace
				"999.999.999.999", // invalid IP
				"abc/24",          // invalid CIDR
			},
		},
	})
	expected := []string{"1.2.3.4", "10.0.0.0/8", "192.168.1.1"}
	if len(cfg.Common.IPBlacklist) != len(expected) {
		t.Fatalf("expected %d IPs, got %d: %v", len(expected), len(cfg.Common.IPBlacklist), cfg.Common.IPBlacklist)
	}
	for i, ip := range expected {
		if cfg.Common.IPBlacklist[i] != ip {
			t.Fatalf("expected IP[%d]=%q, got %q", i, ip, cfg.Common.IPBlacklist[i])
		}
	}
}

func TestDecodeOrderRiskControlConfig_MigratesFlatPolicyWithoutEmailRisk(t *testing.T) {
	cfg := DecodeOrderRiskControlConfig(jsonmap.JSON{
		"enabled":                            true,
		"max_pending_orders_per_user":        float64(4),
		"max_pending_orders_per_ip":          float64(6),
		"max_pending_orders_per_guest_email": float64(9),
		"email_blacklist":                    []interface{}{"ignored@example.com"},
		"order_rate_limit": map[string]interface{}{
			"enabled":        true,
			"window_seconds": float64(90),
			"max_requests":   float64(7),
			"block_seconds":  float64(180),
		},
	}, DefaultOrderRiskControlConfig())

	if !cfg.Enabled || cfg.Guest.MaxPendingOrdersPerIP != 6 || cfg.Member.MaxPendingOrdersPerUser != 4 {
		t.Fatalf("unexpected migrated policy: %+v", cfg)
	}
	if cfg.Guest.RateLimit.MaxRequests != 7 || cfg.Member.RateLimit.MaxRequests != 7 {
		t.Fatalf("expected old rate limit to migrate to both policies: guest=%+v member=%+v", cfg.Guest.RateLimit, cfg.Member.RateLimit)
	}
	if cfg.Guest.MaxQuantityPerProductPerOrder != 0 || cfg.Guest.MaxPendingQuantityPerIPProduct != 0 || cfg.Guest.PaymentExpireMinutes != 0 {
		t.Fatalf("new guest protections must remain disabled for an existing flat config: %+v", cfg.Guest)
	}
	encoded := EncodeOrderRiskControlConfig(cfg)
	if _, exists := encoded["email_blacklist"]; exists {
		t.Fatalf("email blacklist must not survive v2 encoding: %#v", encoded)
	}
	if _, exists := encoded["max_pending_orders_per_guest_email"]; exists {
		t.Fatalf("guest email pending limit must not survive v2 encoding: %#v", encoded)
	}
}

func TestDecodeOrderRiskControlConfig_PreservesLegacyDefaultsForSparseFlatPolicy(t *testing.T) {
	cfg := DecodeOrderRiskControlConfig(jsonmap.JSON{
		"enabled": true,
		"order_rate_limit": map[string]interface{}{
			"window_seconds": float64(90),
		},
	}, DefaultOrderRiskControlConfig())

	if cfg.Guest.MaxPendingOrdersPerIP != 5 || cfg.Member.MaxPendingOrdersPerUser != 3 || cfg.Member.MaxPendingOrdersPerIP != 5 {
		t.Fatalf("sparse flat policy must retain v1 pending defaults: %+v", cfg)
	}
	if cfg.Guest.RateLimit.Enabled || cfg.Member.RateLimit.Enabled || cfg.Guest.RateLimit.MaxRequests != 5 || cfg.Member.RateLimit.MaxRequests != 5 {
		t.Fatalf("sparse flat policy must retain the disabled v1 rate default: guest=%+v member=%+v", cfg.Guest.RateLimit, cfg.Member.RateLimit)
	}
	if cfg.Guest.RateLimit.WindowSeconds != 90 || cfg.Member.RateLimit.WindowSeconds != 90 {
		t.Fatalf("explicit legacy rate fields must still migrate: guest=%+v member=%+v", cfg.Guest.RateLimit, cfg.Member.RateLimit)
	}
}

func TestIsValidIPOrCIDR(t *testing.T) {
	tests := []struct {
		input string
		valid bool
	}{
		{"1.2.3.4", true},
		{"::1", true},
		{"10.0.0.0/8", true},
		{"192.168.0.0/16", true},
		{"fe80::/10", true},
		{"invalid", false},
		{"999.999.999.999", false},
		{"abc/24", false},
		{"", false},
	}
	for _, tc := range tests {
		if got := isValidIPOrCIDR(tc.input); got != tc.valid {
			t.Errorf("isValidIPOrCIDR(%q) = %v, want %v", tc.input, got, tc.valid)
		}
	}
}
