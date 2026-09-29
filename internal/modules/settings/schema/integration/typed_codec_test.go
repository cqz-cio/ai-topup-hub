package settingsintegration

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/dujiao-next/internal/constants"
	settingsstorefront "github.com/dujiao-next/internal/modules/settings/schema/storefront"
	"github.com/dujiao-next/internal/shared/jsonmap"
)

func TestDashboardSettingCodecPreservesDefaultsAndAcceptedNumberShapes(t *testing.T) {
	fallback := settingsstorefront.DefaultDashboardSetting()
	decoded := settingsstorefront.DecodeDashboardSetting(jsonmap.JSON{
		"alert": map[string]interface{}{
			"low_stock_threshold":              json.Number("7"),
			"out_of_stock_products_threshold":  float64(2),
			"pending_payment_orders_threshold": 200001,
			"payments_failed_threshold":        0,
		},
		"ranking": map[string]interface{}{
			"top_products_limit": float64(8),
			"top_channels_limit": "9",
		},
		"accounting": map[string]interface{}{
			"refund_reverses_cost": "yes",
		},
	}, fallback)

	if decoded.Alert.LowStockThreshold != 7 || decoded.Alert.OutOfStockProductsThreshold != 2 {
		t.Fatalf("dashboard alert numeric decode mismatch: %#v", decoded.Alert)
	}
	if decoded.Alert.PendingPaymentOrdersThreshold != 20 || decoded.Alert.PaymentsFailedThreshold != 10 {
		t.Fatalf("dashboard alert bounds mismatch: %#v", decoded.Alert)
	}
	if decoded.Ranking.TopProductsLimit != 8 || decoded.Ranking.TopChannelsLimit != 9 {
		t.Fatalf("dashboard ranking decode mismatch: %#v", decoded.Ranking)
	}
	if !decoded.Accounting.RefundReversesCost {
		t.Fatalf("dashboard accounting decode mismatch: %#v", decoded.Accounting)
	}

	encoded := settingsstorefront.EncodeDashboardSetting(decoded)
	if !reflect.DeepEqual(encoded["alert"], map[string]interface{}{
		"low_stock_threshold":              int64(7),
		"out_of_stock_products_threshold":  int64(2),
		"pending_payment_orders_threshold": int64(20),
		"payments_failed_threshold":        int64(10),
	}) {
		t.Fatalf("dashboard encode mismatch: %#v", encoded)
	}
	if !reflect.DeepEqual(encoded["accounting"], map[string]interface{}{
		"refund_reverses_cost": true,
	}) {
		t.Fatalf("dashboard accounting encode mismatch: %#v", encoded)
	}
}

func TestAffiliateSettingCodecNormalizesAndDetachesWithdrawChannels(t *testing.T) {
	setting := NormalizeAffiliateSetting(AffiliateSetting{
		Enabled:           true,
		CommissionRate:    123.456,
		ConfirmDays:       -10,
		MinWithdrawAmount: -100.239,
		WithdrawChannels:  []string{"  usdt  ", "USDT", "", "paypal"},
	})
	if setting.CommissionRate != 100 || setting.ConfirmDays != 0 || setting.MinWithdrawAmount != 0 {
		t.Fatalf("affiliate numeric normalization mismatch: %#v", setting)
	}
	if !reflect.DeepEqual(setting.WithdrawChannels, []string{"usdt", "paypal"}) {
		t.Fatalf("affiliate channels mismatch: %#v", setting.WithdrawChannels)
	}
	if err := ValidateAffiliateSetting(setting); err != nil {
		t.Fatalf("normalized affiliate setting should validate: %v", err)
	}
	if ErrAffiliateConfigInvalid.Error() != "affiliate config invalid" {
		t.Fatalf("affiliate validation sentinel changed: %v", ErrAffiliateConfigInvalid)
	}

	encoded := EncodeAffiliateSetting(setting)
	channels, ok := encoded["withdraw_channels"].([]string)
	if !ok {
		t.Fatalf("affiliate channels encoded with unexpected type: %T", encoded["withdraw_channels"])
	}
	channels[0] = "mutated"
	if setting.WithdrawChannels[0] != "usdt" {
		t.Fatalf("affiliate encoded channels share backing storage: %#v", setting.WithdrawChannels)
	}
}

func TestAffiliateSettingDecodeUsesFallbackAndAcceptedScalarShapes(t *testing.T) {
	fallback := DefaultAffiliateSetting()
	decoded := DecodeAffiliateSetting(jsonmap.JSON{
		"enabled":             "yes",
		"commission_rate":     "12.345",
		"confirm_days":        float64(4),
		"min_withdraw_amount": 8,
		"withdraw_channels":   []interface{}{"  bank  ", "BANK", "paypal"},
	}, fallback)

	if !decoded.Enabled || decoded.CommissionRate != 12.35 || decoded.ConfirmDays != 4 || decoded.MinWithdrawAmount != 8 {
		t.Fatalf("affiliate decode mismatch: %#v", decoded)
	}
	if !reflect.DeepEqual(decoded.WithdrawChannels, []string{"bank", "paypal"}) {
		t.Fatalf("affiliate decoded channels mismatch: %#v", decoded.WithdrawChannels)
	}
}

func TestUpstreamSyncCodecPreservesFallbackAndBounds(t *testing.T) {
	fallback := UpstreamSyncFallback("30m")
	if fallback.IntervalMinutes != 30 || !fallback.PreOrderStockCheckEnabled {
		t.Fatalf("upstream fallback mismatch: %#v", fallback)
	}

	decoded := DecodeUpstreamSyncConfig(jsonmap.JSON{
		constants.SettingFieldUpstreamSyncIntervalMin: "360",
		constants.SettingFieldUpstreamPreOrderCheck:   false,
		constants.SettingFieldUpstreamSyncPageSize:    float64(100),
		constants.SettingFieldUpstreamSyncMaxPages:    9999,
		constants.SettingFieldUpstreamSyncConcurrency: 0,
	}, fallback)
	if decoded.IntervalMinutes != 360 || decoded.PreOrderStockCheckEnabled || decoded.SyncPageSize != 100 {
		t.Fatalf("upstream decode mismatch: %#v", decoded)
	}
	if decoded.SyncMaxPages != 500 || decoded.SyncConnConcurrency != 3 {
		t.Fatalf("upstream bounds mismatch: %#v", decoded)
	}

	encoded := EncodeUpstreamSyncConfig(decoded)
	if encoded[constants.SettingFieldUpstreamSyncMaxPages] != 500 {
		t.Fatalf("upstream encode mismatch: %#v", encoded)
	}
	if got := FormatUpstreamSyncIntervalForScheduler(time.Minute); got != "5m" {
		t.Fatalf("scheduler minimum mismatch: %q", got)
	}
	if got := FormatUpstreamSyncIntervalForScheduler(2 * time.Hour); got != "120m" {
		t.Fatalf("scheduler interval mismatch: %q", got)
	}
}

func TestTypedSettingJSONNormalizersComposeDefaultDecodeAndEncode(t *testing.T) {
	dashboard := settingsstorefront.NormalizeDashboardSettingJSON(jsonmap.JSON{})
	if dashboard["alert"] == nil || dashboard["ranking"] == nil || dashboard["accounting"] == nil {
		t.Fatalf("dashboard JSON normalizer omitted defaults: %#v", dashboard)
	}
	affiliate := NormalizeAffiliateSettingJSON(jsonmap.JSON{"commission_rate": 101})
	if affiliate["commission_rate"] != 100.0 {
		t.Fatalf("affiliate JSON normalizer did not clamp: %#v", affiliate)
	}
	upstream := NormalizeUpstreamSyncConfigJSON(jsonmap.JSON{
		constants.SettingFieldUpstreamSyncIntervalMin: 1,
	})
	if upstream[constants.SettingFieldUpstreamSyncIntervalMin] != 5 {
		t.Fatalf("upstream JSON normalizer did not restore minimum: %#v", upstream)
	}
}
