package settingsstorefront

import (
	"github.com/dujiao-next/internal/shared/jsonmap"
	"testing"
)

func TestAboutPageRejectsUnsafeLinksAndPhoneFallback(t *testing.T) {
	for _, value := range []string{"javascript:alert(1)", "//host", "/\\host", "https://user:pass@host", "https://host/a b", ""} {
		if validAboutLink(value) {
			t.Fatalf("unsafe link accepted: %q", value)
		}
	}
	for _, value := range []string{"/terms", "/blog/guide?q=1", "https://example.com/path"} {
		if !validAboutLink(value) {
			t.Fatalf("valid link rejected: %q", value)
		}
	}
	value := NormalizeAboutPageJSON(jsonmap.JSON{"supportWechat": "13800000000", "links": []interface{}{
		map[string]interface{}{"to": "javascript:alert(1)"},
		map[string]interface{}{"to": "/terms", "title": map[string]interface{}{"zh-CN": "条款"}},
	}})
	if value["supportWechat"] != "Cqz0924ttxk" {
		t.Fatal("phone number must not become WeChat fallback")
	}
	if len(value["links"].([]interface{})) != 1 {
		t.Fatal("unsafe link not removed")
	}
}
