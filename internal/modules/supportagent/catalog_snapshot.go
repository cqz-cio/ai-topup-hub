package supportagent

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const snapshotCatalogBase = "snapshot://production-public"

//go:embed catalog-snapshot.json
var catalogSnapshotBytes []byte

type catalogSection struct {
	Heading string `json:"heading"`
	Text    string `json:"text"`
}
type catalogPolicy struct {
	Eligibility string `json:"eligibility"`
	AfterSales  string `json:"after_sales"`
	Usage       string `json:"usage"`
	Delivery    string `json:"delivery"`
	Unused      string `json:"unused"`
	RefundMode  string `json:"refund_mode"`
}
type catalogSnapshot struct {
	Source        string                   `json:"source"`
	FetchedAt     string                   `json:"fetched_at"`
	Count         int                      `json:"count"`
	ContentSHA256 string                   `json:"content_sha256"`
	Products      []publicProduct          `json:"products"`
	Policies      map[string]catalogPolicy `json:"policies"`
}

var syncedCatalog = func() catalogSnapshot {
	var data catalogSnapshot
	if json.Unmarshal(catalogSnapshotBytes, &data) != nil || data.Count == 0 || data.Count != len(data.Products) || data.Count != len(data.Policies) {
		panic("invalid public catalog snapshot")
	}
	seen := map[string]bool{}
	for _, p := range data.Products {
		if seen[p.Slug] || data.Policies[p.Slug].AfterSales == "" {
			panic("incomplete public catalog snapshot")
		}
		seen[p.Slug] = true
	}
	return data
}()

func snapshotProduct(slug string) (publicProduct, bool) {
	for _, p := range syncedCatalog.Products {
		if p.Slug == slug {
			return p, true
		}
	}
	return publicProduct{}, false
}

func snapshotPolicyRule(slug, topic string) PolicyRule {
	p, ok := syncedCatalog.Policies[slug]
	if !ok {
		return PolicyRule{}
	}
	text := ""
	switch topic {
	case "eligibility":
		text = p.Eligibility
	case "warranty", "refund":
		text = p.AfterSales
	case "redemption":
		text = p.Usage
	case "delivery":
		text = p.Delivery
	case "unused":
		text = p.Unused
	default:
		text = p.Eligibility + "\n\n" + p.AfterSales + "\n\n" + p.Usage
	}
	return PolicyRule{ID: "catalog:" + slug + ":" + topic, Text: text}
}

func snapshotNotice() string {
	stamp := syncedCatalog.FetchedAt
	if at, err := time.Parse(time.RFC3339Nano, stamp); err == nil {
		stamp = at.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02 15:04") + "（北京时间）"
	}
	return "资料同步时间：" + stamp + "。价格和库存为同步快照，下单以正式商城结算页为准。"
}

func snapshotPrice(p publicProduct) string {
	stock := "同步时可售"
	if p.SoldOut {
		stock = "同步时缺货"
	}
	return fmt.Sprintf("%s｜¥%s｜%s", language(p.Title, "zh-CN"), p.Price, stock)
}

func snapshotFamily(p publicProduct) string {
	s := strings.ToLower(language(p.Title, "zh-CN"))
	for _, name := range []string{"grok", "claude", "gemini", "telegram", "codex"} {
		if strings.Contains(s, name) {
			return name
		}
	}
	if strings.Contains(s, "chatgpt") || strings.Contains(s, "gpt") {
		return "gpt"
	}
	if strings.HasPrefix(s, "x ") {
		return "x"
	}
	return ""
}
