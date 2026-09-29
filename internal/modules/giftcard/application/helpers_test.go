package application

import (
	"testing"
	"time"
)

func TestNormalizeIDsDeduplicatesAndDropsZero(t *testing.T) {
	got := normalizeIDs([]uint{0, 2, 2, 3, 0, 3})
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("unexpected ids: %#v", got)
	}
}

func TestNormalizeExpireAtUTC(t *testing.T) {
	raw := time.Date(2026, 7, 21, 10, 0, 0, 0, time.FixedZone("CST", 8*3600))
	got := normalizeExpireAt(&raw)
	if got == nil || got.Location() != time.UTC {
		t.Fatalf("expected UTC expire at, got %#v", got)
	}
}
