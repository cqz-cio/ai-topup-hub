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

type siteURLCache struct{ data map[string]interface{} }

func (*siteURLCache) CacheKey(*uint) string { return "public-config" }
func (s *siteURLCache) GetJSON(_ context.Context, _ string, dst *map[string]interface{}) (bool, error) {
	*dst = s.data
	return s.data != nil, nil
}
func (s *siteURLCache) SetJSON(_ context.Context, _ string, value interface{}, _ time.Duration) error {
	s.data = value.(map[string]interface{})
	return nil
}

type siteURLSettings struct{}

func (siteURLSettings) GetConfig(map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"brand": jsonmap.JSON{"site_url": "http://127.0.0.1:4170", "name": "测试商城"}}, nil
}
func (siteURLSettings) GetWalletRechargeChannelIDs() []uint                     { return nil }
func (siteURLSettings) GetWalletOnlyPayment() bool                              { return false }
func (siteURLSettings) GetAffiliateSettingMap() (map[string]interface{}, error) { return nil, nil }
func (siteURLSettings) GetSMTPEnabled() bool                                    { return false }
func (siteURLSettings) GetRegistrationEnabled(value bool) (bool, error)         { return value, nil }
func (siteURLSettings) GetEmailVerificationEnabled(value bool) (bool, error)    { return value, nil }
func (siteURLSettings) GetRegistrationEmailDomainPolicy() (bool, []string, error) {
	return false, nil, nil
}
func (siteURLSettings) GetByKey(string) (interface{}, error)            { return nil, nil }
func (siteURLSettings) GetActiveHomeAnnouncement() (jsonmap.JSON, bool) { return nil, false }

type siteURLPayments struct{}

func (siteURLPayments) GetOrderPaymentChannels() ([]map[string]interface{}, error) { return nil, nil }

func TestPublicConfigRepairsSiteURLOnCacheMissAndHitWithoutCachingRequestHost(t *testing.T) {
	cache := &siteURLCache{}
	handler := &Handler{cache: cache, settings: siteURLSettings{}, payments: siteURLPayments{}}
	for _, origin := range []string{"https://www.qzweb.cloud", "https://other.example", "http://127.0.0.1:5173"} {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest("GET", origin+"/api/v1/public/config", nil)
		handler.GetConfig(c)
		var result struct {
			Data struct {
				Brand map[string]interface{} `json:"brand"`
			} `json:"data"`
		}
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
			t.Fatalf("unexpected response: %d %s", w.Code, w.Body.String())
		}
		if result.Data.Brand["site_url"] != origin || result.Data.Brand["name"] != "测试商城" {
			t.Fatalf("incorrect public brand: %+v", result.Data.Brand)
		}
		if aboutObject(cache.data["brand"])["site_url"] != "http://127.0.0.1:4170" {
			t.Fatal("request origin contaminated cached settings")
		}
		if _, exists := cache.data["server_time"]; exists {
			t.Fatal("response metadata contaminated cache")
		}
	}
}
