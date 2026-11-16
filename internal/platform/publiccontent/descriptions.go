package publiccontent

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

//go:embed page-descriptions.json
var descriptionsJSON []byte

func PageDescription(key string) string {
	var descriptions map[string]string
	if err := json.Unmarshal(descriptionsJSON, &descriptions); err != nil {
		panic(err)
	}
	return descriptions[key]
}

// Names are the current anonymous public projection, never a hard-coded catalog.
func CatalogDescription(name string, names []string) string {
	if len(names) == 0 {
		return ""
	}
	first := []rune(names[0])
	if len(first) > 36 {
		first = first[:36]
	}
	return name + "分类提供「" + string(first) + "」等当前在售商品。请按页面核对具体套餐、交付方式与适用条件，售价和库存以商品详情为准。使用期限与质保范围请分别确认，购买前阅读兑换及售后说明。"
}

func BlogDescription(names []string) string {
	intro := "浏览本站已发布的文章与购买指引。"
	if len(names) > 0 {
		first := []rune(names[0])
		if len(first) > 32 {
			first = first[:32]
		}
		intro = "浏览本站文章列表，阅读「" + string(first) + "」等已发布内容。"
	}
	return intro + "可通过文章了解商品选购、使用与订单相关说明，具体内容以各篇正文为准。下单前请同时核对商品页面的适用条件、交付方式和售后范围。"
}

func NeedsDescription(description, title string) bool {
	return len([]rune(strings.TrimSpace(description))) < 60 || strings.TrimSpace(description) == strings.TrimSpace(title)
}

// Parse the explicit editorial date in the actual saved policy, not the server
// clock or the date unrelated settings (e.g. payment configuration) changed.
func LegalModifiedDates(legal any) map[string]time.Time {
	result := map[string]time.Time{}
	raw, _ := json.Marshal(legal)
	var policies map[string]any
	_ = json.Unmarshal(raw, &policies)
	pattern := regexp.MustCompile(`更新日期[：:\s]*([0-9]{4})\s*(?:年|-)\s*([0-9]{1,2})\s*(?:月|-)\s*([0-9]{1,2})`)
	for _, key := range []string{"terms", "privacy"} {
		body := language(policies[key])
		if body == "" {
			body = text(policies[key])
		}
		match := pattern.FindStringSubmatch(body)
		if len(match) != 4 {
			continue
		}
		var y, m, d int
		_, _ = fmt.Sscanf(strings.Join(match[1:], " "), "%d %d %d", &y, &m, &d)
		date := time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
		if date.Year() == y && int(date.Month()) == m && date.Day() == d {
			result["/"+key] = date
		}
	}
	return result
}
