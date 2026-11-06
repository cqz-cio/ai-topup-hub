package supportagent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func requestedTool(id, name, args string) ToolCall {
	return ToolCall{ID: id, Type: "function", Function: FunctionCall{Name: name, Arguments: args}}
}
func callCompletion(calls ...ToolCall) Completion {
	return Completion{ToolCalls: calls, Input: 100, Output: 30}
}

type scriptedProvider struct {
	calls int
	next  func(int, []Message, []ToolDefinition) (Completion, error)
}

func (p *scriptedProvider) Complete(_ context.Context, m []Message, d []ToolDefinition) (Completion, error) {
	p.calls++
	return p.next(p.calls, m, d)
}

func TestToolSchemasAndValidation(t *testing.T) {
	if len(ToolDefinitions()) != 6 {
		t.Fatal("tool scope changed")
	}
	for _, test := range []struct{ name, args string }{
		{"shell", "{}"}, {"lookup_product", `{"url":"https://example.com"}`}, {"lookup_product", `{"product_slug":"../private"}`},
		{"lookup_product", `{"query":null}`}, {"lookup_product", `{"query":123}`}, {"lookup_product", `{"query":"a","query":"b"}`},
		{"lookup_product", `{"query":"https://example.com"}`}, {"lookup_product", `{} {}`}, {"lookup_product", `[]`},
		{"get_product_policy", `{"topic":"execute_refund"}`}, {"get_payment_guidance", `{"scene":"wallet","user_id":"123"}`},
	} {
		if _, err := decodeToolArguments(test.name, test.args); err == nil {
			t.Fatalf("accepted %s %s", test.name, test.args)
		}
	}
	for _, d := range ToolDefinitions() {
		if _, err := decodeToolArguments(d.Function.Name, "{}"); err != nil {
			t.Fatal(d.Function.Name, err)
		}
	}
}

func TestSharedPolicyPaymentTroubleshootingAndHandoff(t *testing.T) {
	e := ToolExecutor{}
	for _, test := range []struct{ name, args, want, forbid string }{
		{"get_product_policy", `{"product_slug":"qzweb-51","topic":"warranty"}`, "成功激活", "实际掉订阅时的剩余订阅时长"},
		{"get_product_policy", `{"product_slug":"86gamestore-82","topic":"eligibility"}`, "不限制原充值渠道", "未到期勿拍"},
		{"get_payment_guidance", `{"scene":"wallet"}`, "只入账本金", "已确认到账"},
		{"get_payment_guidance", `{"scene":"order"}`, "1.6%", "免手续费"},
		{"get_troubleshooting_guide", `{"issue":"payment_pending"}`, "支付成功截图", "已确认到账"},
		{"get_troubleshooting_guide", `{"issue":"wrong_account"}`, "不支持因此退款", "已经转移"},
		{"prepare_handoff", `{"issue":"activation_error"}`, "Cqz0924ttxk", "已经提交"},
	} {
		r, err := e.Execute(context.Background(), requestedTool("test", test.name, test.args), ToolContext{Input: chatInput{Locale: "zh-CN", Message: "请帮我处理"}})
		b, _ := json.Marshal(r)
		if err != nil || !strings.Contains(string(b), test.want) || strings.Contains(string(b), test.forbid) {
			t.Fatalf("%s: %s %v", test.name, b, err)
		}
		if test.name == "prepare_handoff" && (r.Direct == nil || !r.Direct.Handoff || !strings.Contains(string(b), `"submitted":false`)) {
			t.Fatal("handoff claimed submission")
		}
	}
	for _, args := range []string{`{}`, `{"product_slug":"unknown-product"}`} {
		r, err := e.Execute(context.Background(), requestedTool("unknown", "get_product_policy", args), ToolContext{})
		if err != nil || r.Status != "needs_info" {
			t.Fatal(r, err)
		}
	}
}

func TestToolEstimateRequiresUserEvidenceAndKeepsProductRules(t *testing.T) {
	args := toolArgs{Product: "qzweb-5", Action: "reopen", Paid: "100.00", Currency: "CNY", Start: "2026-02-01 00:00", End: "2026-03-01 00:00", Loss: "2026-02-15 00:00", ReopenPrice: "150.00"}
	tc := ToolContext{Input: chatInput{Product: "qzweb-5", Message: "实付100 CNY，开始2026-02-01 00:00，到期2026-03-01 00:00，掉订阅2026-02-15 00:00，重开150 CNY"}}
	r := estimateTool(args, "gpt_open", tc)
	e, ok := r.Data.(AfterSalesEstimate)
	if !ok || e.Refund != "50.00" || e.Difference != "100.00" || e.Executed || r.Direct == nil {
		t.Fatal(r)
	}
	tc.Input.Message = "请帮我算一下"
	r = estimateTool(args, "gpt_open", tc)
	if r.Status != "needs_info" || r.Missing[0] != "paid_amount" {
		t.Fatal("model invented input accepted", r)
	}
	tc.History = []Message{{Role: "assistant", Content: "实付100 CNY，开始2026-02-01 00:00，到期2026-03-01 00:00，掉订阅2026-02-15 00:00，重开150 CNY"}}
	if r = estimateTool(args, "gpt_open", tc); r.Status != "needs_info" {
		t.Fatal("assistant claims used as evidence")
	}
	tc.History = nil
	tc.Input.Message = "-100 CNY"
	if r = estimateTool(args, "gpt_open", tc); r.Status != "needs_info" {
		t.Fatal("negative amount evidence accepted")
	}
	for _, p := range []string{"gemini_warranty", "gemini_none", "codex", "telegram"} {
		if r = estimateTool(args, p, tc); r.Status != "handoff" {
			t.Fatal("cross-product refund rule", p, r)
		}
	}
}

func TestCatalogToolWhitelistFreshnessAndFailures(t *testing.T) {
	available := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/public/products" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("nonpublic lookup")
		}
		if !available {
			http.Error(w, "private internal detail", 503)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": []any{map[string]any{
			"slug": "qzweb-5", "title": map[string]string{"zh-CN": "GPT Pro"}, "content": map[string]string{"zh-CN": "<script>private-script</script>真实说明 https://redemption.example.com"}, "price_amount": "130.00", "delivery_content": "PRIVATE-CODE", "cost_amount": "70.00",
			"skus": []any{map[string]any{"is_active": true, "price_amount": "130.00", "spec_values": map[string]string{"地区": "菲区"}, "supplier_secret": "PRIVATE-SECRET"}},
		}}})
	}))
	defer server.Close()
	c := NewCatalog()
	c.base = server.URL
	r, err := c.Lookup(context.Background(), "zh-CN", "", "qzweb-5")
	b, _ := json.Marshal(r)
	if err != nil || len(r.Products) != 1 || r.Products[0].Price != "130.00" || r.Products[0].Variants[0].Specs["地区"] != "菲区" || r.ObservedAt == "" || r.Source != "local_storefront_public_api" {
		t.Fatal(r, err)
	}
	if !r.Products[0].PolicyKnown || !strings.Contains(string(b), "至少 10 分钟") {
		t.Fatal("detail lookup omitted confirmed policy")
	}
	for _, forbidden := range []string{"PRIVATE-CODE", "PRIVATE-SECRET", "70.00", "redemption.example.com", "private-script"} {
		if strings.Contains(string(b), forbidden) {
			t.Fatal("leaked", forbidden)
		}
	}
	if empty, err := c.Lookup(context.Background(), "zh-CN", "", "unknown"); err != nil || len(empty.Products) != 0 {
		t.Fatal("invented product", empty, err)
	}
	available = false
	c.expires = c.expires.Add(-2 * time.Minute)
	e := ToolExecutor{Knowledge: c}
	out, err := e.Execute(context.Background(), requestedTool("x", "lookup_product", `{"product_slug":"qzweb-5"}`), ToolContext{Input: chatInput{Locale: "zh-CN"}})
	if err != nil || out.Status != "unavailable" {
		t.Fatal("stale catalog returned", out, err)
	}
}

func TestDeepSeekToolProtocolThroughService(t *testing.T) {
	catalogRequests := 0
	catalogServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		catalogRequests++
		json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": []any{map[string]any{"slug": "qzweb-5", "title": map[string]string{"zh-CN": "GPT Pro"}, "price_amount": "130.00"}}})
	}))
	defer catalogServer.Close()
	catalog := NewCatalog()
	catalog.base = catalogServer.URL
	requests := 0
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		var body struct {
			Messages []Message        `json:"messages"`
			Tools    []ToolDefinition `json:"tools"`
			Choice   string           `json:"tool_choice"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil {
			t.Error("invalid wire body")
		}
		finish := "tool_calls"
		var msg any
		switch requests {
		case 1:
			if len(body.Tools) != 6 || body.Choice != "auto" {
				t.Error("missing registry")
			}
			msg = Message{Role: "assistant", ToolCalls: []ToolCall{requestedTool("call_product", "lookup_product", `{"product_slug":"qzweb-5"}`)}}
		case 2:
			last := body.Messages[len(body.Messages)-1]
			if last.Role != "tool" || last.ToolCallID != "call_product" || !strings.Contains(last.Content, "130.00") || len(body.Messages[len(body.Messages)-2].ToolCalls) != 1 {
				t.Error("tool result protocol broken")
			}
			msg = Message{Role: "assistant", ToolCalls: []ToolCall{requestedTool("call_policy", "get_product_policy", `{"product_slug":"qzweb-5","topic":"eligibility"}`)}}
		case 3:
			if len(body.Tools) != 0 || body.Choice != "none" {
				t.Error("last round still permits tools")
			}
			last := body.Messages[len(body.Messages)-1]
			if last.ToolCallID != "call_policy" || !strings.Contains(last.Content, "至少 10 分钟") {
				t.Error("wrong policy")
			}
			finish = "stop"
			answer, _ := json.Marshal(ModelAnswer{Kind: "answer", Answer: "公开参考价130元；请先确认账号满足开通条件，结账为准。", SourceIDs: []string{"tool:call_product", "tool:call_policy"}})
			msg = Message{Role: "assistant", Content: string(answer)}
		default:
			t.Error("unexpected model call")
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": finish, "message": msg}}, "usage": map[string]int{"prompt_tokens": 100, "completion_tokens": 30}})
	}))
	defer modelServer.Close()
	s, _, cookie := fixture(t)
	p := NewDeepSeek(testConfig())
	p.endpoint = modelServer.URL
	s.provider = p
	s.knowledge = catalog
	question := input(1, "看看这个的具体资料")
	question.Product = "qzweb-5"
	w := send(s, cookie, question)
	var reply Reply
	_ = json.Unmarshal(w.Body.Bytes(), &reply)
	if w.Code != 200 || len(reply.ToolsUsed) != 2 || requests != 3 || catalogRequests != 1 || len(reply.Links) < 1 {
		t.Fatal(w.Code, w.Body.String(), requests, catalogRequests)
	}
	var count int
	var cost int64
	s.ledger.db.QueryRow("SELECT COUNT(*),SUM(cost) FROM support_usage WHERE state='completed'").Scan(&count, &cost)
	if count != 3 || cost != 3*usageCost(100, 30) {
		t.Fatal("rounds not billed", count, cost)
	}
	if again := send(s, cookie, question); again.Body.String() != w.Body.String() || requests != 3 {
		t.Fatal("replay billed again")
	}
}

func TestToolLoopLimitsAndFailureReservation(t *testing.T) {
	for _, mode := range []string{"daily", "budget", "minute", "unknown_usage", "loop", "unknown_tool", "duplicate_id", "invented_source", "too_many_tools"} {
		t.Run(mode, func(t *testing.T) {
			s, _, cookie := fixture(t)
			p := &scriptedProvider{next: func(n int, m []Message, d []ToolDefinition) (Completion, error) {
				if mode == "budget" && n == 1 {
					s.config.DailyBudget = usageCost(100, 30) + 1
				}
				if mode == "unknown_usage" && n == 2 {
					return Completion{}, errors.New("simulated unknown result")
				}
				if mode == "invented_source" && n == 2 {
					return Completion{Answer: ModelAnswer{Kind: "answer", Answer: "说明", SourceIDs: []string{"tool:never_called"}}, Input: 100, Output: 30}, nil
				}
				name := "get_payment_guidance"
				id := "call_" + strings.Repeat("x", n)
				if mode == "unknown_tool" {
					name = "execute_refund"
				}
				if mode == "duplicate_id" {
					id = "repeated"
				}
				c := callCompletion(requestedTool(id, name, `{"scene":"order"}`))
				if mode == "too_many_tools" {
					c.ToolCalls = append(c.ToolCalls, c.ToolCalls[0], c.ToolCalls[0], c.ToolCalls[0])
				}
				return c, nil
			}}
			s.provider = p
			if mode == "daily" {
				s.config.DailyCalls = 1
			}
			if mode == "minute" {
				s.config.SessionPerMinute = 1
			}
			w := send(s, cookie, input(1, "介绍一下这里的服务"))
			want := 502
			if mode == "daily" || mode == "budget" || mode == "minute" {
				want = 429
			}
			if w.Code != want || p.calls > 3 {
				t.Fatal(mode, w.Code, w.Body.String(), p.calls)
			}
			if (mode == "daily" || mode == "budget" || mode == "minute") && p.calls != 1 {
				t.Fatal("second round bypassed limit")
			}
			if mode == "unknown_usage" {
				var count int
				var cost int64
				s.ledger.db.QueryRow("SELECT COUNT(*),SUM(cost) FROM support_usage WHERE state='reserved'").Scan(&count, &cost)
				if count != 1 || cost <= 0 {
					t.Fatal("unknown second call refunded")
				}
				if retry := send(s, cookie, input(1, "介绍一下这里的服务")); retry.Code != 409 || p.calls != 2 {
					t.Fatal("failed multi-round request retried")
				}
			}
		})
	}
}

func TestModelSelectedEstimateIsRenderedByServer(t *testing.T) {
	s, _, cookie := fixture(t)
	p := &scriptedProvider{next: func(_ int, _ []Message, _ []ToolDefinition) (Completion, error) {
		return callCompletion(requestedTool("calc", "estimate_after_sales", `{"product_slug":"qzweb-5","action":"refund","paid_amount":"100.00","currency":"CNY","start_time":"2026-02-01 00:00","end_time":"2026-03-01 00:00","loss_time":"2026-02-15 00:00"}`)), nil
	}}
	s.provider = p
	q := input(1, "金额100 CNY，开始2026-02-01 00:00，到期2026-03-01 00:00，实际结束2026-02-15 00:00，请帮我核算")
	q.Product = "qzweb-5"
	w := send(s, cookie, q)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "50.00 CNY") || !strings.Contains(w.Body.String(), "未提交退款") || p.calls != 1 {
		t.Fatal(w.Code, w.Body.String(), p.calls)
	}
}
