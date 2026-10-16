package supportagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeProvider struct {
	mu      sync.Mutex
	calls   [][]Message
	fail    bool
	wait    chan struct{}
	started chan struct{}
}

func (p *fakeProvider) Complete(ctx context.Context, m []Message) (Completion, error) {
	p.mu.Lock()
	p.calls = append(p.calls, append([]Message(nil), m...))
	p.mu.Unlock()
	if p.started != nil {
		p.started <- struct{}{}
	}
	if p.wait != nil {
		select {
		case <-p.wait:
		case <-ctx.Done():
			return Completion{}, ctx.Err()
		}
	}
	if p.fail {
		return Completion{}, errors.New("unknown network result")
	}
	return Completion{ModelAnswer{"answer", "请在订单交付说明查看兑换步骤。", []string{"guide:redeem"}}, 120, 40}, nil
}
func (p *fakeProvider) count() int { p.mu.Lock(); defer p.mu.Unlock(); return len(p.calls) }

type fakeKnowledge struct{}

func (fakeKnowledge) Sources(_ context.Context, locale, query, slug string) ([]Source, error) {
	return baseSources(locale), nil
}
func testConfig() Config {
	return Config{APIKey: "test-only-not-a-real-key", Model: "deepseek-flash", MaxOutput: 800, DailyBudget: 250000000, DailyCalls: 100, SessionDailyCalls: 50, SessionPerMinute: 6, GlobalPerMinute: 20}
}
func fixture(t *testing.T) (*Service, *fakeProvider, *http.Cookie) {
	t.Helper()
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ledger.Close() })
	provider := &fakeProvider{}
	service := NewService(testConfig(), provider, fakeKnowledge{}, ledger)
	return service, provider, newSession(t, service)
}
func newSession(t *testing.T, s *Service) *http.Cookie {
	t.Helper()
	request := httptest.NewRequest("GET", "http://127.0.0.1:8085/api/v1/support/session", nil)
	request.RemoteAddr = "127.0.0.1:12345"
	response := httptest.NewRecorder()
	s.ServeHTTP(response, request)
	if response.Code != 200 {
		t.Fatal(response.Code, response.Body.String())
	}
	cookie := response.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("unsafe cookie")
	}
	return cookie
}
func input(id int, message string) chatInput {
	return chatInput{RequestID: fmt.Sprintf("%032x", id), ConversationID: strings.Repeat("a", 32), Message: message, Locale: "zh-CN"}
}
func send(s *Service, cookie *http.Cookie, value any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(value)
	r := httptest.NewRequest("POST", "http://127.0.0.1:8085/api/v1/support/chat", strings.NewReader(string(b)))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Origin", "http://127.0.0.1:5175")
	r.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		r.AddCookie(cookie)
	}
	w := httptest.NewRecorder()
	s.ServeHTTP(w, r)
	return w
}
func TestNaturalContextAndIdempotency(t *testing.T) {
	s, p, c := fixture(t)
	first := input(1, "买了 GPT，兑换码不知道往哪填")
	if w := send(s, c, first); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w := send(s, c, first); w.Code != 200 || p.count() != 1 {
		t.Fatal("repeat billed", w.Body.String())
	}
	first.Message = "different"
	if w := send(s, c, first); w.Code != 409 {
		t.Fatal("request ID reused with different payload")
	}
	if w := send(s, c, input(2, "我已经拿到了")); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if len(p.calls[1]) != 4 || !strings.Contains(p.calls[1][1].Content, "兑换码") {
		t.Fatal("context lost")
	}
	if !json.Valid([]byte(p.calls[1][2].Content)) {
		t.Fatal("assistant history must preserve JSON response format")
	}
	third := input(3, "重新问个问题")
	third.ConversationID = strings.Repeat("b", 32)
	if w := send(s, c, third); w.Code != 200 || len(p.calls[2]) != 2 {
		t.Fatal("new conversation retained old context")
	}
}
func TestNarrowLocalGateDoesNotBlockShopLanguage(t *testing.T) {
	s, p, c := fixture(t)
	if w := send(s, c, input(1, "今天天气咋样")); w.Code != 200 || p.count() != 0 {
		t.Fatal("weather invoked model")
	}
	if w := send(s, c, input(2, "你们卖的 GPT 能查天气吗")); w.Code != 200 || p.count() != 1 {
		t.Fatal("related question blocked")
	}
	if w := send(s, c, input(3, "这个怎么用")); w.Code != 200 || p.count() != 2 {
		t.Fatal("ambiguous message blocked")
	}
	if w := send(s, c, input(4, "测试凭据 sk-abcdefghijklmnopqrstuvwxyz123456")); w.Code != 200 || p.count() != 2 {
		t.Fatal("credential forwarded")
	}
	if w := send(s, c, input(5, "邮箱 someone@example.com 已经收到邮件了")); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	last := p.calls[len(p.calls)-1]
	if strings.Contains(last[len(last)-1].Content, "someone@example.com") {
		t.Fatal("email forwarded")
	}
}
func TestRequestBoundariesBeforeBilling(t *testing.T) {
	for _, name := range []string{"unknown", "long", "missing-session", "origin", "host", "remote", "trailing"} {
		t.Run(name, func(t *testing.T) {
			s, p, c := fixture(t)
			b, _ := json.Marshal(input(1, "怎么充值"))
			body := string(b)
			if name == "unknown" {
				body = strings.TrimSuffix(body, "}") + `,"model":"expensive-model"}`
			}
			if name == "long" {
				v := input(1, strings.Repeat("a", 501))
				b, _ = json.Marshal(v)
				body = string(b)
			}
			if name == "trailing" {
				body += "{}"
			}
			r := httptest.NewRequest("POST", "http://127.0.0.1:8085/api/v1/support/chat", strings.NewReader(body))
			r.RemoteAddr = "127.0.0.1:1234"
			r.Header.Set("Origin", "http://127.0.0.1:5175")
			r.Header.Set("Content-Type", "application/json")
			if name != "missing-session" {
				r.AddCookie(c)
			}
			if name == "origin" {
				r.Header.Set("Origin", "https://attacker.example")
			}
			if name == "host" {
				r.Host = "attacker.example:8085"
			}
			if name == "remote" {
				r.RemoteAddr = "192.0.2.5:1234"
			}
			w := httptest.NewRecorder()
			s.ServeHTTP(w, r)
			if w.Code < 400 || p.count() != 0 {
				t.Fatalf("%d calls=%d", w.Code, p.count())
			}
		})
	}
}
func TestDailyBudgetAndFailureReservation(t *testing.T) {
	s, p, c := fixture(t)
	s.config.DailyBudget = 1
	if w := send(s, c, input(1, "怎么充值")); w.Code != 429 || p.count() != 0 {
		t.Fatal("budget did not fail closed")
	}
	s.config = testConfig()
	p.fail = true
	if w := send(s, c, input(2, "怎么充值")); w.Code != 502 {
		t.Fatal(w.Body.String())
	}
	if w := send(s, c, input(2, "怎么充值")); w.Code != 409 || p.count() != 1 {
		t.Fatal("failed request retried")
	}
	var cost int64
	var state string
	if err := s.ledger.db.QueryRow("SELECT cost,state FROM support_usage").Scan(&cost, &state); err != nil || cost <= 0 || state != "reserved" {
		t.Fatal("unknown usage refunded", err, cost, state)
	}
}
func TestLedgerSurvivesRestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.db")
	l, err := OpenLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	c := testConfig()
	c.DailyCalls = 1
	if err := l.Reserve("one", "session", 500, c, time.Now()); err != nil {
		t.Fatal(err)
	}
	l.Close()
	l, err = OpenLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if err := l.Reserve("two", "new-session", 500, c, time.Now()); !errors.Is(err, ErrBudget) {
		t.Fatal("restart reset cap", err)
	}
	if err := l.Reserve("one", "session", 500, c, time.Now()); !errors.Is(err, ErrDuplicate) {
		t.Fatal("restart reset idempotency")
	}
}
func TestRateAndConcurrency(t *testing.T) {
	t.Run("rate", func(t *testing.T) {
		s, p, c := fixture(t)
		s.config.SessionPerMinute = 1
		send(s, c, input(1, "你好"))
		if w := send(s, c, input(2, "怎么充值")); w.Code != 429 || p.count() != 0 {
			t.Fatal("rate not enforced")
		}
	})
	t.Run("concurrent", func(t *testing.T) {
		s, p, c := fixture(t)
		p.wait = make(chan struct{})
		p.started = make(chan struct{}, 3)
		done := make(chan *httptest.ResponseRecorder, 2)
		go func() { done <- send(s, c, input(1, "怎么充值")) }()
		<-p.started
		if w := send(s, c, input(2, "怎么充值")); w.Code != 429 {
			t.Fatal("session concurrency not enforced")
		}
		c2 := newSession(t, s)
		go func() { done <- send(s, c2, input(3, "怎么充值")) }()
		<-p.started
		c3 := newSession(t, s)
		if w := send(s, c3, input(4, "怎么充值")); w.Code != 429 {
			t.Fatal("global concurrency not enforced")
		}
		close(p.wait)
		for i := 0; i < 2; i++ {
			if w := <-done; w.Code != 200 {
				t.Fatal(w.Body.String())
			}
		}
	})
}
func TestModelOutputBoundary(t *testing.T) {
	for _, a := range []ModelAnswer{
		{"answer", "到 https://example.com 付款", []string{"guide:redeem"}},
		{"answer", "普通答案", []string{"invented"}},
		{"answer", "普通答案", nil},
		{"answer", "<script>alert(1)</script>", []string{"guide:redeem"}},
		{"tool_calls", "test", nil},
	} {
		if _, err := validateAnswer(a, baseSources("zh-CN"), "zh-CN"); err == nil {
			t.Fatalf("unsafe answer accepted: %s", a.Kind)
		}
	}
	r, err := validateAnswer(ModelAnswer{"answer", "查看订单。", []string{"guide:redeem"}}, baseSources("zh-CN"), "zh-CN")
	if err != nil || len(r.Links) != 2 || r.Links[0].To != "/me/orders" {
		t.Fatal("trusted links lost")
	}
}
func TestCatalogWhitelistAndPrivacy(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/public/products" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" {
			t.Error("catalog requested nonpublic data")
		}
		json.NewEncoder(w).Encode(map[string]any{"status_code": 0, "data": []any{map[string]any{"slug": "test-gpt", "title": map[string]string{"zh-CN": "GPT Plus 测试"}, "content": map[string]string{"zh-CN": "<script>private-script</script><p>按订单操作 https://hidden.example.com/redeem</p>"}, "delivery_content": "PRIVATE-ORDER-CODE", "price_amount": "100.00"}}})
	}))
	defer server.Close()
	c := NewCatalog()
	c.base = server.URL
	sources, err := c.Sources(context.Background(), "zh-CN", "GPT", "test-gpt")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(sources)
	if !strings.Contains(string(b), "context:page") || !strings.Contains(string(b), "GPT Plus 测试") {
		t.Fatal("current product context missing")
	}
	for _, forbidden := range []string{"PRIVATE-ORDER-CODE", "hidden.example.com", "private-script"} {
		if strings.Contains(string(b), forbidden) {
			t.Fatal("private/raw material leaked", forbidden)
		}
	}
}
func TestDeepSeekWireContract(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		if payload["tools"] != nil || payload["stream"] != false || payload["model"] != "deepseek-flash" || payload["max_tokens"] != float64(800) {
			t.Error("unsafe wire settings")
		}
		if payload["thinking"].(map[string]any)["type"] != "disabled" {
			t.Error("thinking enabled")
		}
		if r.Header.Get("Authorization") != "Bearer test-only-not-a-real-key" {
			t.Error("missing server auth")
		}
		answer, _ := json.Marshal(ModelAnswer{"answer", "查看订单。", []string{"guide:redeem"}})
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"finish_reason": "stop", "message": map[string]any{"content": string(answer)}}}, "usage": map[string]int{"prompt_tokens": 123, "completion_tokens": 45}})
	}))
	defer server.Close()
	provider := NewDeepSeek(testConfig())
	provider.endpoint = server.URL
	answer, err := provider.Complete(context.Background(), []Message{{"system", "JSON"}, {"user", "怎么充值"}})
	if err != nil || answer.Input != 123 || answer.Answer.Kind != "answer" {
		t.Fatal(err, answer)
	}
}
