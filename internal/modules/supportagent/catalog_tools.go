package supportagent

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"time"
)

// Only public, customer-facing fields are decoded. No supplier code, cost,
// delivery secrets, account-specific pricing or private order data is forwarded.
type publicSKU struct {
	Specs   map[string]any `json:"spec_values"`
	Price   string         `json:"price_amount"`
	Active  bool           `json:"is_active"`
	SoldOut bool           `json:"is_sold_out"`
}
type ProductVariant struct {
	Specs   map[string]string `json:"specifications"`
	Price   string            `json:"public_price,omitempty"`
	SoldOut bool              `json:"sold_out"`
}
type ProductInfo struct {
	Slug           string           `json:"slug"`
	Title          string           `json:"title"`
	Description    string           `json:"description"`
	Price          string           `json:"public_price,omitempty"`
	Currency       string           `json:"currency"`
	SoldOut        bool             `json:"sold_out"`
	Variants       []ProductVariant `json:"variants,omitempty"`
	Truncated      bool             `json:"details_truncated,omitempty"`
	PolicyKnown    bool             `json:"policy_known"`
	ConfirmedRules []PolicyRule     `json:"confirmed_rules,omitempty"`
}
type ProductLookup struct {
	Products     []ProductInfo `json:"products"`
	TotalMatches int           `json:"total_matches"`
	HasMore      bool          `json:"has_more"`
	Source       string        `json:"source"`
	ObservedAt   string        `json:"observed_at"`
	Note         string        `json:"note"`
}

func (c *Catalog) Lookup(ctx context.Context, locale, query, slug string) (ProductLookup, error) {
	result := ProductLookup{Products: []ProductInfo{}, Source: "local_storefront_public_api", Note: "本地商城公开数据；抓取时间不是生产库存同步时间，库存不保证生产实时一致。公开价格及可售状态以结账重新核验为准。"}
	products, err := c.load(ctx)
	if err != nil {
		return result, err
	}
	c.mu.Lock()
	result.ObservedAt = c.expires.Add(-time.Minute).UTC().Format(time.RFC3339)
	c.mu.Unlock()
	type candidate struct {
		p     publicProduct
		score int
	}
	matches := []candidate{}
	q := strings.ToLower(strings.TrimSpace(query))
	seen := map[string]bool{}
	for _, p := range products {
		if !slugPattern.MatchString(p.Slug) || seen[p.Slug] {
			continue
		}
		seen[p.Slug] = true
		title := strings.ToLower(plain(language(p.Title, locale), 120))
		if title == "" {
			continue
		}
		score := 0
		if slug != "" {
			if p.Slug != slug {
				continue
			}
			score = 100
		} else {
			if !matchesRequestedProduct(q, title) {
				continue
			}
			if strings.Contains(title, q) || strings.Contains(q, p.Slug) {
				score += 30
			}
			for _, term := range []string{"gpt", "plus", "pro", "codex", "gemini", "claude", "telegram", "ios", "5x", "10x", "25x", "续费", "开通", "点数", "菲区", "智利"} {
				if strings.Contains(q, term) && strings.Contains(title, term) {
					score += 2
				}
			}
			if contains(q, "全部商品", "所有商品", "all products") {
				score++
			}
		}
		if score > 0 {
			matches = append(matches, candidate{p, score})
		}
	}
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].score > matches[j].score })
	result.TotalMatches = len(matches)
	result.HasMore = len(matches) > 5
	for i, item := range matches {
		if i >= 5 {
			break
		}
		p := item.p
		detail := language(p.Description, locale)
		detailLimit, productBytes := 200, 1800
		if slug != "" {
			detail = language(p.Content, locale)
			detailLimit, productBytes = 700, 8000
		}
		info := ProductInfo{Slug: p.Slug, Title: plain(language(p.Title, locale), 120), Description: plain(detail, detailLimit), Currency: "CNY", SoldOut: p.SoldOut}
		info.PolicyKnown = policy.Products[p.Slug] != ""
		if info.PolicyKnown {
			ids := eligibilityRuleIDs(policy.Products[p.Slug])
			if slug != "" {
				ids = productRuleIDs(policy.Products[p.Slug])
			}
			info.ConfirmedRules = policyRules(ids...)
		}
		if pricePattern.MatchString(p.Price) {
			info.Price = p.Price
		}
		for _, sku := range p.SKUs {
			if !sku.Active {
				continue
			}
			if len(info.Variants) >= 8 {
				info.Truncated = true
				break
			}
			v := ProductVariant{Specs: map[string]string{}, SoldOut: sku.SoldOut}
			if pricePattern.MatchString(sku.Price) {
				v.Price = sku.Price
			}
			keys := []string{}
			for k := range sku.Specs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if len(v.Specs) >= 8 {
					break
				}
				if value, ok := sku.Specs[k].(string); ok {
					v.Specs[plain(k, 60)] = plain(value, 120)
				}
			}
			info.Variants = append(info.Variants, v)
			encoded, _ := json.Marshal(info)
			if len(encoded) > productBytes {
				info.Variants = info.Variants[:len(info.Variants)-1]
				info.Truncated = true
				break
			}
		}
		result.Products = append(result.Products, info)
	}
	return result, nil
}
