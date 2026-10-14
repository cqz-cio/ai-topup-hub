package publicconfighttp

import (
	"context"
	"encoding/json"
	"github.com/dujiao-next/internal/shared/jsonmap"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"reflect"
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

func TestMainAboutPageUsesSavedContentAndSharedWechatWithoutMutatingSiteConfig(t *testing.T) {
	original := map[string]interface{}{"telegram": "https://t.me/example", "wechat": "old"}
	data := map[string]interface{}{"contact": original, "about": map[string]interface{}{"legacy": "preserve"}}
	page := jsonmap.JSON{"version": 1, "title": map[string]interface{}{"zh-CN": "关于"}, "supportWechat": "Cqz0924ttxk"}
	applyMainAboutPage(data, page)
	if !reflect.DeepEqual(data["about_page"], map[string]interface{}(page)) {
		t.Fatal("content not exposed")
	}
	contact := data["contact"].(map[string]interface{})
	if contact["wechat"] != "Cqz0924ttxk" || contact["telegram"] != original["telegram"] {
		t.Fatal("contact not merged")
	}
	if original["wechat"] != "old" {
		t.Fatal("original contact settings mutated")
	}
	empty := map[string]interface{}{"about": "legacy"}
	applyMainAboutPage(empty, nil)
	if len(empty) != 1 || empty["about"] != "legacy" {
		t.Fatal("old installation changed without saved content")
	}
}
