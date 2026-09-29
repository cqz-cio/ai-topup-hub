package ginutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestParseQueryTimeRangeTrimsAndParsesRFC3339(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?created_from=+2026-06-01T10:00:00Z+&created_to=2026-06-02T10:00:00Z", nil)

	from, to, err := ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		t.Fatalf("ParseQueryTimeRange: %v", err)
	}
	if from == nil || to == nil {
		t.Fatalf("expected both range values parsed, got from=%v to=%v", from, to)
	}
	if got := from.Format(time.RFC3339); got != "2026-06-01T10:00:00Z" {
		t.Fatalf("expected created_from parsed and trimmed, got %s", got)
	}
	if got := to.Format(time.RFC3339); got != "2026-06-02T10:00:00Z" {
		t.Fatalf("expected created_to parsed, got %s", got)
	}
}

func TestParseQueryTimeRangeAllowsEmptyBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)

	from, to, err := ParseQueryTimeRange(c, "created_from", "created_to")
	if err != nil {
		t.Fatalf("ParseQueryTimeRange: %v", err)
	}
	if from != nil || to != nil {
		t.Fatalf("expected nil empty bounds, got from=%v to=%v", from, to)
	}
}

func TestParseQueryTimeRangeRejectsInvalidBound(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/?created_from=not-a-time", nil)

	if _, _, err := ParseQueryTimeRange(c, "created_from", "created_to"); err == nil {
		t.Fatalf("expected invalid time error")
	}
}
