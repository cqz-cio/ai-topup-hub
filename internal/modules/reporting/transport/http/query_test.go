package reportinghttp

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/?range=custom&from=2026-07-01T00:00:00Z&to=2026-07-02T00:00:00Z&tz=UTC&force_refresh=true", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	query, err := ParseQuery(context)
	if err != nil {
		t.Fatalf("parse reporting query: %v", err)
	}
	if query.Range != "custom" || query.From == nil || query.To == nil || query.Timezone != "UTC" || !query.ForceRefresh {
		t.Fatalf("unexpected query: %+v", query)
	}
}
