package publiccontent

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestAboutSavedLanguagePairsBlankFieldsAndEscaping(t *testing.T) {
	raw := map[string]any{"version": float64(1), "title": map[string]any{"zh-CN": "<script>title</script>"}, "services": []any{map[string]any{"title": map[string]any{"en-US": "English heading"}, "text": map[string]any{"zh-CN": "中文正文"}}}, "links": []any{map[string]any{"title": map[string]any{"zh-CN": "恶意链接"}, "to": "javascript:alert(1)"}}}
	title, summary, body := AboutCopy(raw, nil, nil, true)
	if title != "<script>title</script>" || summary != "" || !strings.Contains(body, "中文正文") || strings.Contains(body, "English heading") || strings.Contains(body, "恶意链接") || strings.Contains(body, "官方直充") {
		t.Fatal("saved resolution differs from storefront", title, summary, body)
	}
	_, _, reseller := AboutCopy(raw, map[string]any{"introduction": map[string]any{"zh-CN": "租户介绍"}}, nil, false)
	if !strings.Contains(reseller, "租户介绍") || strings.Contains(reseller, "π星") || strings.Contains(reseller, "Cqz") || strings.Contains(reseller, "1669268759") {
		t.Fatal("main-store identity leaked to tenant", reseller)
	}
}

func TestHomeFirstVisitCopyMatchesExistingVisibleStorefront(t *testing.T) {
	raw, err := os.ReadFile("../../../frontend/user/src/i18n/locales/zh-CN.json")
	if err != nil {
		t.Fatal(err)
	}
	var messages map[string]map[string]any
	var copy map[string]any
	if err := json.Unmarshal(raw, &messages); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(homeCopyJSON, &copy); err != nil {
		t.Fatal(err)
	}
	for key, value := range copy {
		a, _ := json.Marshal(value)
		b, _ := json.Marshal(messages["shopUi"][key])
		if string(a) != string(b) {
			t.Fatal("public and visible homepage diverged:", key)
		}
	}
	_, _, about := AboutCopy(nil, nil, nil, true)
	if !strings.Contains(about, IdentityStatement()) || strings.Contains(about, "官方直充") || strings.Contains(about, "非成品账号") {
		t.Fatal("misleading fallback copy")
	}
}
