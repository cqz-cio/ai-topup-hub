package supportagent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

// Historical products remain test fixtures, never current recommendations.
func historicalCatalog(t *testing.T) {
	t.Helper()
	before := syncedCatalog
	b, err := os.ReadFile("testdata/catalog-20261026.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &syncedCatalog); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { syncedCatalog = before })
}

func snapshotAsk(t *testing.T, c *Catalog, s *BusinessState, slug, message string) Reply {
	t.Helper()
	r, handled := c.OfflineAnswer(chatInput{Product: slug, Message: message, Locale: "zh-CN"}, s)
	if !handled || r.Mode != "local" {
		t.Fatalf("not offline: %+v", r)
	}
	return r
}

func TestSyncedCatalogCompleteAndQueryableOffline(t *testing.T) {
	c := NewCatalog()
	c.client = nil // any accidental network read would panic
	if syncedCatalog.Count == 0 || len(syncedCatalog.Policies) != syncedCatalog.Count || len(syncedCatalog.Products) != syncedCatalog.Count {
		t.Fatal("incomplete production snapshot")
	}
	for _, p := range syncedCatalog.Products {
		t.Run(p.Slug, func(t *testing.T) {
			if policy.Products[p.Slug] == "" || len(p.Sections) == 0 || len(p.SKUs) == 0 {
				t.Fatal("missing mapping/details/specifications")
			}
			r := snapshotAsk(t, c, &BusinessState{}, p.Slug, "价格和完整详情")
			for _, want := range []string{language(p.Title, "zh-CN"), p.Price, "同步快照", language(p.Content, "zh-CN")} {
				if !strings.Contains(r.Text, want) {
					t.Fatalf("missing product data %q", want)
				}
			}
			rules := snapshotAsk(t, c, &BusinessState{}, p.Slug, "质保与退款规则")
			if !strings.Contains(rules.Text, syncedCatalog.Policies[p.Slug].AfterSales) {
				t.Fatal("missing exact policy")
			}
			lookup, err := c.Lookup(context.Background(), "zh-CN", "", p.Slug)
			if err != nil || len(lookup.Products) != 1 || !lookup.Products[0].PolicyKnown || lookup.ObservedAt != syncedCatalog.FetchedAt {
				t.Fatal(lookup, err)
			}
		})
	}
}

func TestSyncedPolicyMatrix(t *testing.T) {
	historicalCatalog(t)
	for _, tc := range []struct{ slug, q, want, forbid string }{
		{"ellaix-claude-pro", "质保多久", "不做任何质保", "实际订阅总时长"},
		{"ellaix-claude-max-5x", "退款试算", "不做任何质保", "请提供这笔订单的实际付款金额"},
		{"ellaix-claude-max-20x", "账号条件", "已通过 KYC", "未到期勿拍"},
		{"ellaix-grok-premium-monthly-half", "半价能用吗", "请勿将半价兑换码用于普通全价套餐", "剩余订阅时长 ÷"},
		{"ellaix-grok-super-monthly", "掉订阅退多少", "没有已确认的退款比例", "请提供这笔订单的实际付款金额"},
		{"ellaix-x-plus-monthly-ng", "地区条件", "尼日利亚", "保证无条件"},
		{"ellaix-x-super-monthly-ng", "未使用能退款吗", "未确认本款未使用", "销毁后再办理"},
		{"ellaix-gpt-plus-account-30d", "退款试算", "未给出按剩余时长退款", "请提供这笔订单的实际付款金额"},
		{"ellaix-gpt-plus-account-30d", "怎么用", "自动发货不等于自动完成充值", "收到后自动开通"},
		{"qzweb-52", "质保规则", "成功激活", "不做任何质保"},
		{"qzweb-53", "退款规则", "不提供激活后的持续订阅质保", "全额退款"},
		{"86gamestore-10004", "质保", "激活后不提供质保", "按剩余时长"},
		{"qzweb-5", "未兑换退款", "销毁后再办理退款", "已经退款"},
	} {
		t.Run(tc.slug+tc.q, func(t *testing.T) {
			s := BusinessState{}
			r := snapshotAsk(t, NewCatalog(), &s, tc.slug, tc.q)
			if !strings.Contains(r.Text, tc.want) || strings.Contains(r.Text, tc.forbid) || s.Stage != 0 {
				t.Fatal(r.Text, s)
			}
		})
	}
}

func TestSyncedSearchAndConversationIdentity(t *testing.T) {
	historicalCatalog(t)
	c := NewCatalog()
	s := BusinessState{}
	r := snapshotAsk(t, c, &s, "", "全部商品")
	if len(s.CatalogChoices) != 35 || !strings.Contains(r.Text, "35 款") || strings.Contains(r.Text, "Codex 250") {
		t.Fatal(r.Text)
	}
	r = snapshotAsk(t, c, &s, "", "Claude Pro多少钱")
	if s.CatalogSlug != "ellaix-claude-pro" || !strings.Contains(r.Text, "168.88") || strings.Contains(r.Text, "788.00") {
		t.Fatal(r.Text)
	}
	r = snapshotAsk(t, c, &s, "", "这个掉订阅可以退多少")
	if s.Stage != 0 || !strings.Contains(r.Text, "不做任何质保") {
		t.Fatal(r.Text)
	}
	r = snapshotAsk(t, c, &s, "", "Grok Premium 月付半价多少钱")
	if s.CatalogSlug != "ellaix-grok-premium-monthly-half" || !strings.Contains(r.Text, "138.00") {
		t.Fatal(r.Text)
	}
	r = snapshotAsk(t, c, &s, "", "X Plus月付")
	if s.CatalogSlug != "ellaix-x-plus-monthly-ng" || !strings.Contains(r.Text, "68.00") {
		t.Fatal(r.Text)
	}
	r = snapshotAsk(t, c, &s, "", "Gemini掉订阅怎么退")
	if len(s.CatalogChoices) != 2 {
		t.Fatal(r.Text)
	}
	r = snapshotAsk(t, c, &s, "", "无质保款")
	if s.CatalogSlug != "qzweb-53" || !strings.Contains(r.Text, "不提供激活后的持续订阅质保") {
		t.Fatal(r.Text)
	}
	r = snapshotAsk(t, c, &s, "", "Grok月付半价")
	if len(s.CatalogChoices) != 3 {
		t.Fatal(r.Text)
	}
	chosen := s.CatalogChoices[1]
	r = snapshotAsk(t, c, &s, "", "2")
	if s.CatalogSlug != chosen {
		t.Fatal("choice lost", r.Text)
	}
	r = snapshotAsk(t, c, &s, "ellaix-claude-max-20x", "这个有质保吗")
	if s.CatalogSlug != "ellaix-claude-max-20x" || !strings.Contains(r.Text, "不做任何质保") {
		t.Fatal("page switch lost", r.Text)
	}
	r = snapshotAsk(t, c, &s, "ellaix-claude-max-20x", "qzweb-73多少钱")
	if !strings.Contains(r.Text, "不在本次同步") || s.CatalogSlug != "" {
		t.Fatal("retired product reused", r.Text)
	}
}

func TestSyncedProportionalCalculationAndProductSwitch(t *testing.T) {
	historicalCatalog(t)
	c := NewCatalog()
	s := BusinessState{}
	for i, q := range []string{"qzweb-5 退款试算", "130 CNY", "2026-02-01 00:00", "2026-03-01 00:00", "2026-02-15 00:00"} {
		r := snapshotAsk(t, c, &s, "", q)
		if i == 4 && !strings.Contains(r.Text, "65.00 CNY") {
			t.Fatal(r.Text)
		}
	}
	snapshotAsk(t, c, &s, "", "qzweb-5退款试算")
	r := snapshotAsk(t, c, &s, "", "Claude Pro退款试算")
	if s.Stage != 0 || !strings.Contains(r.Text, "不做任何质保") {
		t.Fatal("old calculator leaked", r.Text)
	}
	r = snapshotAsk(t, c, &s, "", "130 CNY")
	if s.Stage != 0 || strings.Contains(r.Text, "实际订阅成功开始时间") {
		t.Fatal("accepted money for no warranty", r.Text)
	}
}

func TestSyncedAllProductsThroughHTTPNeverUseModelOrLedger(t *testing.T) {
	config := testConfig()
	config.AllowModel = false
	config.SessionPerMinute = 200
	config.GlobalPerMinute = 200
	provider := &fakeProvider{}
	service := NewService(config, provider, NewCatalog(), nil)
	cookie := newSession(t, service)
	for i, p := range syncedCatalog.Products {
		v := input(i+1, "价格和售后规则")
		v.Product = p.Slug
		v.ConversationID = fmt.Sprintf("%032x", i+1)
		w := send(service, cookie, v)
		var reply Reply
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &reply) != nil || reply.Mode != "local" || !strings.Contains(reply.Text, p.Price) {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if provider.count() != 0 {
		t.Fatal("offline called model")
	}
	// A reset cannot inherit a previously selected product or calculation.
	v := input(99, "130 CNY")
	v.ConversationID = strings.Repeat("f", 32)
	if w := send(service, cookie, v); !strings.Contains(w.Body.String(), "具体商品") {
		t.Fatal(w.Body.String())
	}
}

func TestSyncedToolsDoNotBorrowOldClaudePolicy(t *testing.T) {
	for _, p := range syncedCatalog.Products {
		call := requestedTool("policy", "get_product_policy", fmt.Sprintf(`{"product_slug":%q,"topic":"warranty"}`, p.Slug))
		r, err := (ToolExecutor{}).Execute(context.Background(), call, ToolContext{})
		b, _ := json.Marshal(r)
		if err != nil || !strings.Contains(string(b), syncedCatalog.Policies[p.Slug].AfterSales[:12]) {
			t.Fatal(p.Slug, string(b), err)
		}
		if strings.HasPrefix(p.Slug, "ellaix-claude-") || strings.HasPrefix(p.Slug, "ellaix-grok-") || strings.HasPrefix(p.Slug, "ellaix-x-") || p.Slug == "ellaix-gpt-plus-account-30d" {
			r = estimateTool(toolArgs{Product: p.Slug, Action: "refund"}, policy.Products[p.Slug], ToolContext{})
			if r.Status != "handoff" || r.Direct == nil {
				t.Fatal("unsupported refund allowed", p.Slug, r)
			}
		}
	}
}

func TestSyncedAdditionalDetailsAndVariantSearch(t *testing.T) {
	historicalCatalog(t)
	c := NewCatalog()
	for _, tc := range []struct{ slug, q, want string }{
		{"86gamestore-10004", "未激活可以退款吗", "作废链接后办理"},
		{"86gamestore-10004", "已是会员还能叠加吗", "暂无法确认"},
		{"qzweb-52", "未发货想退款", "申请退款"},
		{"qzweb-5", "填错UID了", "不支持因此退款"},
	} {
		r := snapshotAsk(t, c, &BusinessState{}, tc.slug, tc.q)
		if !strings.Contains(r.Text, tc.want) {
			t.Fatal(r.Text)
		}
	}
	s := BusinessState{}
	snapshotAsk(t, c, &s, "", "Claude Pro价格")
	r := snapshotAsk(t, c, &s, "", "那Max20x呢")
	if s.CatalogSlug != "ellaix-claude-max-20x" || !strings.Contains(r.Text, "1588.00") {
		t.Fatal(r.Text)
	}
	for _, tc := range []struct {
		q     string
		count int
	}{
		{"Grok多少钱", 11}, {"X Plus月付", 1}, {"Claude Pro价格", 1}, {"Grok Premium月付半价", 1}, {"GPT Pro 5x", 3},
	} {
		lookup, err := c.Lookup(context.Background(), "zh-CN", tc.q, "")
		if err != nil || lookup.TotalMatches != tc.count {
			t.Fatal(tc.q, lookup, err)
		}
	}
}
