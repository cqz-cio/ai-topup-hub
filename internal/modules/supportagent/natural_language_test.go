package supportagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNaturalSupportRoutingAndFocusedReplies(t *testing.T) {
	for _, test := range []struct{ message, want, forbid string }{
		{"Plus 不是 130 块吗，怎么选微信付款就变成 132.08 了？", "手续费 2.08 元，合计 132.08 元", "USDT"},
		{"标价99元，微信付款怎么多了？", "手续费 1.59 元，合计 100.59 元", "USDT"},
		{"我买的 Plus 一直没弄成功，能找个人帮我看看吗？需要我发什么给他？", "Cqz0924ttxk", "已经转交"},
	} {
		in := chatInput{Message: test.message, Locale: "zh-CN"}
		if !requiresPolicy(in, BusinessState{}) {
			t.Fatal("obvious fee/handoff request missed", test.message)
		}
		reply := businessAnswer(in, &BusinessState{})
		if reply.Mode != "local" || !strings.Contains(reply.Text, test.want) || strings.Contains(reply.Text, test.forbid) {
			t.Fatal(reply)
		}
	}
	if asksForPerson("不用找个人帮我，先说明规则") {
		t.Fatal("negative request became a handoff")
	}
	if asksAboutWeChatFee("微信手续费多少，还有哪些支付方式？") {
		t.Fatal("mixed payment question lost its other question")
	}
	for _, message := range []string{"你们自动发码的 Plus 现在多少钱？有货吗？", "我 Plus 还剩五天才到期，你们 130 块那款能直接续上吗？剩下这几天会不会没了？"} {
		if requiresPolicy(chatInput{Message: message}, BusinessState{}) {
			t.Fatal("catalog-dependent question bypassed model/tools")
		}
	}
	if detectProduct("Plus还有五天") != "gpt" || detectProduct("Claude 和 Plus 比较") != "ambiguous" || detectProduct("surplus") != "" {
		t.Fatal("family alias not scoped")
	}
}

func TestNaturalRefundRetainsAmountAndAsksForMissingFacts(t *testing.T) {
	s, provider, cookie := fixture(t)
	s.config.AllowModel = true
	questions := []string{
		"我 2 月 1 号花 130 开的 Plus，本来能用到 3 月 1 号，结果 15 号就掉了。这种能退多少？",
		"是人民币", "2026年2月1日0点", "2026年3月1日0点", "2026年2月15日0点",
	}
	for index, question := range questions {
		response := send(s, cookie, input(index+1, question))
		var reply Reply
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &reply) != nil || reply.Mode != "local" {
			t.Fatal(response.Code, response.Body.String())
		}
		if index == 0 && (!strings.Contains(reply.Text, "130.00") || !strings.Contains(reply.Text, "人民币") || strings.Contains(reply.Text, "65.00")) {
			t.Fatal("lost amount or guessed missing currency/year", reply)
		}
		if index == 1 && (!strings.Contains(reply.Text, "年份") || strings.Contains(reply.Text, "实付金额")) {
			t.Fatal("lost supplied amount/currency", reply)
		}
		if index == 4 && (!strings.Contains(reply.Text, "65.00 CNY") || !strings.Contains(reply.Text, "28天") || !strings.Contains(reply.Text, "14天") || strings.Contains(reply.Text, "2419200") || !strings.Contains(reply.Text, "未提交退款")) {
			t.Fatal("actual February duration was not used", reply)
		}
	}
	if provider.count() != 0 {
		t.Fatal("deterministic calculator unexpectedly called model")
	}
	for _, bad := range []string{"花 -130 元", "花130.001元", "花130e2元", "花100元，又花200元"} {
		if _, ok := beginNaturalCalculation(bad, &BusinessState{}); ok {
			t.Fatal("ambiguous/invalid amount accepted", bad)
		}
	}
	state := BusinessState{Stage: 1}
	if _, ok := beginNaturalCalculation("花130 USDT", &state); !ok || state.Currency != "USDT" {
		t.Fatal("USDT parsed as USD", state)
	}
	for _, date := range []string{"2月1日0点", "2026年2月30日0点", "2026年2月1日25点", "2026年2月1日"} {
		if _, err := parseMoment(date); err == nil {
			t.Fatal("incomplete/invalid date accepted", date)
		}
	}
	for _, test := range []struct {
		seconds int64
		want    string
	}{{0, "0天"}, {60, "1分钟"}, {90061, "1天1小时1分钟1秒"}, {2419200, "28天"}} {
		if got := readableDuration(test.seconds); got != test.want {
			t.Fatal("duration was rounded or lost precision", got, test.want)
		}
	}
}

func TestCatalogSearchIncludesConditionsAndRespectsNamedVariant(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": []any{
			map[string]any{"slug": "qzweb-5", "title": map[string]string{"zh-CN": "ChatGPT Plus 兑换码｜自动发码"}, "price_amount": "130.00"},
			map[string]any{"slug": "86gamestore-88", "title": map[string]string{"zh-CN": "ChatGPT Plus｜代充｜人工发货"}, "price_amount": "145.00"},
			map[string]any{"slug": "qzweb-7", "title": map[string]string{"zh-CN": "ChatGPT Pro 10x 续费绑卡｜自动发码"}, "price_amount": "1200.00"},
		}})
	}))
	defer server.Close()
	catalog := NewCatalog()
	catalog.base = server.URL
	for _, test := range []struct {
		query string
		count int
	}{{"Plus", 2}, {"自动发码的Plus", 1}} {
		result, err := catalog.Lookup(context.Background(), "zh-CN", test.query, "")
		if err != nil || len(result.Products) != test.count {
			t.Fatal(result, err)
		}
		for _, item := range result.Products {
			if item.Slug == "qzweb-7" {
				t.Fatal("Plus query included Pro")
			}
			if item.Slug == "qzweb-5" {
				data, _ := json.Marshal(item.ConfirmedRules)
				if !strings.Contains(string(data), "至少 10 分钟") {
					t.Fatal("search result omitted known condition")
				}
			}
		}
	}
	result, err := (ToolExecutor{}).Execute(context.Background(), requestedTool("eligibility", "get_product_policy", `{"product_slug":"qzweb-5","topic":"eligibility"}`), ToolContext{})
	data, _ := json.Marshal(result)
	if err != nil || !strings.Contains(string(data), "至少 10 分钟") || strings.Contains(string(data), "质保订阅") {
		t.Fatal("eligibility tool mixed unrelated policy", string(data), err)
	}
}
