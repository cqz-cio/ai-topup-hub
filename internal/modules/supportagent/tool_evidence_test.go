package supportagent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolPlanningLeavesBusinessFactsWithTools(t *testing.T) {
	sources := modelPlanningSources("zh-CN")
	for _, source := range sources {
		if strings.HasPrefix(source.ID, "policy:") || source.ID == "guide:payment" || source.ID == "guide:human" || source.ID == "guide:wallet" || strings.Contains(source.Text, "1.6%") {
			t.Fatal("tool-owned fact duplicated in initial model context", source.ID)
		}
	}
	if len(sources) == 0 {
		t.Fatal("general store navigation lost")
	}
}

func TestEstimateToolAcceptsExplicitChineseDatesAndRejectsGuesses(t *testing.T) {
	a := toolArgs{Product: "qzweb-5", Action: "refund", Paid: "130.00", Currency: "CNY", Start: "2026-02-01 00:00", End: "2026-03-01 00:00", Loss: "2026-02-15 00:00"}
	message := "这单 Plus 花了130元，2026年2月1日0点开的，原本2026年3月1日0点结束，2026年2月15日0点就不能用了。没用上的那段能拿回多少钱？"
	tc := ToolContext{Input: chatInput{Product: "qzweb-5", Message: message}}
	result := estimateTool(a, "gpt_open", tc)
	estimate, ok := result.Data.(AfterSalesEstimate)
	if !ok || result.Status != "ok" || estimate.Refund != "65.00" || estimate.Executed || result.Direct == nil || !strings.Contains(result.Direct.Text, "28天") || !strings.Contains(result.Direct.Text, "14天") {
		t.Fatal(result)
	}
	for _, start := range []string{"2月1日0点", "2026年2月1日", "2026年2月30日0点", "2026年2月1日0点30秒", "2026-02-01 00:00:30", "12026年2月1日0点", "2026年2月1日0点123分"} {
		tc.Input.Message = strings.Replace(message, "2026年2月1日0点", start, 1)
		result = estimateTool(a, "gpt_open", tc)
		if result.Status != "needs_info" || len(result.Missing) == 0 || result.Missing[0] != "start_time" {
			t.Fatal("incomplete or different time accepted", start, result)
		}
	}
	tc.Input.Message = "我花了130元，时间我记不清了。"
	tc.History = []Message{{Role: "assistant", Content: message}}
	if result = estimateTool(a, "gpt_open", tc); result.Status != "needs_info" {
		t.Fatal("assistant supplied dates became user evidence")
	}
	tc.History = []Message{{Role: "user", Content: message}}
	if result = estimateTool(a, "gpt_open", tc); result.Status != "ok" {
		t.Fatal("earlier user dates were lost")
	}
	wanted, _ := parseMoment("2026-02-01 00:30")
	if !hasMomentEvidence("开通是2026年2月1日0点30分。", wanted) || !hasMomentEvidence("2026-01-31T16:30:00Z", wanted) {
		t.Fatal("explicit minute or equivalent timezone rejected")
	}
}

func TestRemainingToolsThroughNaturalServiceRequests(t *testing.T) {
	for _, test := range []struct{ message, name, args, want string }{
		{"这款 Plus 要是用着用着会员没了，你们这边一般怎么处理？", "get_product_policy", `{"product_slug":"qzweb-5","topic":"warranty"}`, "规则已读取"},
		{"我没买过U，能不能直接扫一扫把钱付了？", "get_payment_guidance", `{"scene":"order"}`, "规则已读取"},
		{"这单 Plus 花了130元，2026年2月1日0点开的，原本2026年3月1日0点结束，2026年2月15日0点就不能用了。没用上的那段能拿回多少钱？", "estimate_after_sales", `{"product_slug":"qzweb-5","action":"refund","paid_amount":"130.00","currency":"CNY","start_time":"2026-02-01 00:00","end_time":"2026-03-01 00:00","loss_time":"2026-02-15 00:00"}`, "65.00 CNY"},
		{"这个码怎么弄都不行，我自己实在搞不定了，能让店里的人帮我看一下吗？我该把什么发给他？", "prepare_handoff", `{"issue":"activation_error"}`, "未提交工单"},
	} {
		t.Run(test.name, func(t *testing.T) {
			s, _, cookie := fixture(t)
			p := &scriptedProvider{next: func(n int, m []Message, definitions []ToolDefinition) (Completion, error) {
				if n == 1 {
					if len(definitions) != 6 {
						t.Fatal("tools not available")
					}
					return callCompletion(requestedTool("selected", test.name, test.args)), nil
				}
				last := m[len(m)-1]
				if last.Role != "tool" || last.ToolCallID != "selected" {
					t.Fatal("tool result missing")
				}
				return Completion{Answer: ModelAnswer{Kind: "answer", Answer: "规则已读取", SourceIDs: []string{"tool:selected"}}, Input: 100, Output: 30}, nil
			}}
			s.provider = p
			question := input(1, test.message)
			question.Product = "qzweb-5"
			response := send(s, cookie, question)
			var reply Reply
			if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &reply) != nil || reply.Mode != "model" || len(reply.ToolsUsed) != 1 || reply.ToolsUsed[0] != test.name || !strings.Contains(reply.Text, test.want) {
				t.Fatal(response.Code, response.Body.String())
			}
			wantCalls := 2
			if test.name == "estimate_after_sales" || test.name == "prepare_handoff" {
				wantCalls = 1
			}
			if p.calls != wantCalls {
				t.Fatal("unnecessary extra model call", p.calls)
			}
		})
	}
}
