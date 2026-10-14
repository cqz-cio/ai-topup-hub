package publicconfighttp

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"testing"
	"time"
)

type featureCache struct{}

func (featureCache) CacheKey(*uint) string { return "public-config" }
func (featureCache) GetJSON(_ context.Context, _ string, dst *map[string]interface{}) (bool, error) {
	*dst = map[string]interface{}{"features": map[string]bool{"redemption": true}}
	return true, nil
}
func (featureCache) SetJSON(context.Context, string, interface{}, time.Duration) error { return nil }

func TestCachedConfigCannotAdvertiseUnavailableRedemption(t *testing.T) {
	enabled := false
	h := &Handler{cache: featureCache{}, features: func() map[string]bool { return map[string]bool{"redemption": enabled} }}
	for _, state := range []bool{false, true, false} {
		enabled = state
		r := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(r)
		c.Request = httptest.NewRequest("GET", "/config", nil)
		h.GetConfig(c)
		var result struct {
			Data struct {
				Features map[string]bool `json:"features"`
			} `json:"data"`
		}
		if err := json.Unmarshal(r.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.Data.Features["redemption"] != enabled {
			t.Fatalf("cached runtime feature leaked: %s", r.Body.String())
		}
	}
}
