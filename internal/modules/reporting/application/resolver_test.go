package application

import (
	"errors"
	"testing"
	"time"

	reportingdomain "github.com/dujiao-next/internal/modules/reporting/domain"
)

func TestResolveTodayInRequestedTimezone(t *testing.T) {
	now := time.Date(2026, 7, 20, 18, 0, 0, 0, time.UTC)
	window, err := Resolve(reportingdomain.Query{Range: "today", Timezone: "Asia/Shanghai"}, now)
	if err != nil {
		t.Fatalf("resolve today: %v", err)
	}
	if got := window.StartAt.Format(time.RFC3339); got != "2026-07-21T00:00:00+08:00" {
		t.Fatalf("start want 2026-07-21T00:00:00+08:00, got %s", got)
	}
	if window.EndAt.Sub(window.StartAt) != 24*time.Hour {
		t.Fatalf("today duration want 24h, got %s", window.EndAt.Sub(window.StartAt))
	}
}

func TestResolveRejectsOversizedCustomRange(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 0, CustomMaxDays+1)
	_, err := Resolve(reportingdomain.Query{Range: "custom", From: &from, To: &to, Timezone: "UTC"}, from)
	if !errors.Is(err, reportingdomain.ErrRangeInvalid) {
		t.Fatalf("want ErrRangeInvalid, got %v", err)
	}
}
