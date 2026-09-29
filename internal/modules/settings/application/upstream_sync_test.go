package settingsapp

import (
	"testing"

	"github.com/dujiao-next/internal/constants"
	settingsintegration "github.com/dujiao-next/internal/modules/settings/schema/integration"
)

func TestGetUpstreamSyncConfigFallbackToYaml(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)

	cfg, err := svc.GetUpstreamSyncConfig("30m")
	if err != nil {
		t.Fatalf("GetUpstreamSyncConfig failed: %v", err)
	}
	if cfg.IntervalMinutes != 30 {
		t.Fatalf("expected interval=30 (from yaml fallback), got %d", cfg.IntervalMinutes)
	}
	if !cfg.PreOrderStockCheckEnabled {
		t.Fatalf("expected pre-order check enabled by default")
	}
}

func TestGetUpstreamSyncConfigReadsFromDB(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)

	_, err := svc.Update(constants.SettingKeyUpstreamSyncConfig, map[string]interface{}{
		"interval_minutes":              360,
		"pre_order_stock_check_enabled": false,
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	cfg, err := svc.GetUpstreamSyncConfig("5m")
	if err != nil {
		t.Fatalf("GetUpstreamSyncConfig failed: %v", err)
	}
	if cfg.IntervalMinutes != 360 {
		t.Fatalf("expected interval=360 (from DB), got %d", cfg.IntervalMinutes)
	}
	if cfg.PreOrderStockCheckEnabled {
		t.Fatalf("expected pre-order check disabled per DB setting")
	}
}

func TestUpdateUpstreamSyncConfigNormalizesBelowMinimum(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)

	result, err := svc.Update(constants.SettingKeyUpstreamSyncConfig, map[string]interface{}{
		"interval_minutes": 1, // < 5
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	assertSettingIntValue(t, result, "interval_minutes", settingsintegration.DefaultUpstreamSyncConfig().IntervalMinutes)
}

func TestUpdateUpstreamSyncConfigClampsAboveMaximum(t *testing.T) {
	repo := newMockSettingRepo()
	svc := NewService(repo)

	result, err := svc.Update(constants.SettingKeyUpstreamSyncConfig, map[string]interface{}{
		"interval_minutes": 99999,
	})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}
	maximum := settingsintegration.NormalizeUpstreamSyncConfig(settingsintegration.UpstreamSyncConfig{IntervalMinutes: 99999}).IntervalMinutes
	assertSettingIntValue(t, result, "interval_minutes", maximum)
}
