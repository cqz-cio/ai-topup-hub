package config

import (
	"os"
	"testing"
	"time"
)

func TestUpstreamPurchaseWindow(t *testing.T) {
	const floorKey, timeKey = "UPSTREAM_PURCHASE_ORDER_ID_FLOOR", "UPSTREAM_PURCHASE_PAID_AFTER"
	for _, tc := range []struct {
		name, floor, stamp           string
		hasFloor, hasTime, wantError bool
	}{
		{name: "legacy unset"},
		{name: "valid", floor: "44", stamp: "2026-10-16T22:00:00+08:00", hasFloor: true, hasTime: true},
		{name: "missing time", floor: "44", hasFloor: true, wantError: true},
		{name: "missing floor", stamp: "2026-10-16T14:00:00Z", hasTime: true, wantError: true},
		{name: "empty values", hasFloor: true, hasTime: true, wantError: true},
		{name: "negative floor", floor: "-1", stamp: "2026-10-16T14:00:00Z", hasFloor: true, hasTime: true, wantError: true},
		{name: "overflow floor", floor: "999999999999999999999999", stamp: "2026-10-16T14:00:00Z", hasFloor: true, hasTime: true, wantError: true},
		{name: "missing timezone", floor: "44", stamp: "2026-10-16T14:00:00", hasFloor: true, hasTime: true, wantError: true},
		{name: "zero time", floor: "44", stamp: "0001-01-01T00:00:00Z", hasFloor: true, hasTime: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(floorKey, tc.floor)
			t.Setenv(timeKey, tc.stamp)
			if !tc.hasFloor {
				_ = os.Unsetenv(floorKey)
			}
			if !tc.hasTime {
				_ = os.Unsetenv(timeKey)
			}
			floor, after, err := UpstreamPurchaseWindow()
			if (err != nil) != tc.wantError {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.name == "valid" && (floor != 44 || after == nil || !after.Equal(time.Date(2026, 10, 16, 14, 0, 0, 0, time.UTC))) {
				t.Fatal("wrong purchase boundary")
			}
			if tc.wantError && after != nil {
				t.Fatal("invalid boundary must not become usable")
			}
			if tc.name == "legacy unset" && (floor != 0 || after != nil) {
				t.Fatal("unset legacy boundary changed")
			}
		})
	}
}
