package publiccontent

import (
	"testing"
	"time"
)

func TestPublicDescriptionsUseIndependentContent(t *testing.T) {
	seen := map[string]bool{}
	for _, key := range []string{"terms", "privacy", "about", "products", "notice"} {
		text := PageDescription(key)
		n := len([]rune(text))
		if n < 70 || n > 130 || seen[text] {
			t.Fatal(key, n, text)
		}
		seen[text] = true
	}
	for _, text := range []string{BlogDescription([]string{"购买、交付与售后说明"}), CatalogDescription("Gemini", []string{"Gemini 18 个月质保商品"})} {
		if len([]rune(text)) < 70 || len([]rune(text)) > 130 {
			t.Fatal(text)
		}
	}
	if CatalogDescription("空分类", nil) != "" {
		t.Fatal("invented empty category")
	}
}

func TestLegalDatesComeFromSavedCopyOnly(t *testing.T) {
	dates := LegalModifiedDates(map[string]any{"terms": map[string]any{"zh-CN": "<strong>更新日期：2026 年 10 月 14 日</strong>"}, "privacy": map[string]any{"zh-CN": "更新日期：2026-10-13"}})
	if dates["/terms"].Format("2006-01-02") != "2026-10-14" || dates["/privacy"].Format("2006-01-02") != "2026-10-13" {
		t.Fatal(dates)
	}
	for _, raw := range []string{"未知", "更新日期：2026 年 02 月 31 日"} {
		if !LegalModifiedDates(map[string]any{"terms": raw})["/terms"].Equal(time.Time{}) {
			t.Fatal("fabricated editorial date")
		}
	}
}
