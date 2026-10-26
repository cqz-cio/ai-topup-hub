package supportagent

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type chatInput struct {
	RequestID      string `json:"request_id"`
	ConversationID string `json:"conversation_id"`
	Message        string `json:"message"`
	Locale         string `json:"locale"`
	Product        string `json:"product_slug,omitempty"`
}
type cachedReply struct {
	fingerprint string
	reply       Reply
}
type session struct {
	touched              time.Time
	busy                 bool
	minute               []time.Time
	modelMinute          []time.Time
	conversation, locale string
	history              []Message
	business             BusinessState
	cache                map[string]cachedReply
	cacheOrder           []string
}
type Service struct {
	config      Config
	provider    Provider
	knowledge   Knowledge
	ledger      *Ledger
	mu          sync.Mutex
	sessions    map[string]*session
	minute      []time.Time
	modelMinute []time.Time
	active      int
}

func NewService(c Config, p Provider, k Knowledge, l *Ledger) *Service {
	return &Service{config: c, provider: p, knowledge: k, ledger: l, sessions: map[string]*session{}}
}

var idPattern = regexp.MustCompile(`^[a-f0-9-]{32,36}$`)
var weatherPattern = regexp.MustCompile(`(?i)^(?:(?:今天|明天|现在|現在|北京|上海)的?)*天[气氣](?:怎么样|怎麼樣|咋样|如何|好吗|好嗎|预报|預報)?[?？!！。]*$|^(?:what(?:'s| is) (?:the )?weather(?: like)?(?: today)?|weather(?: today)?)[?.!]*$`)

func opaqueID() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
func hash(s string) string { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func recent(values []time.Time, now time.Time) []time.Time {
	index := 0
	for index < len(values) && now.Sub(values[index]) >= time.Minute {
		index++
	}
	return values[index:]
}
func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
func failure(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]string{"error": code})
}
func allowedOrigin(s string) bool {
	return s == "http://127.0.0.1:5175" || s == "http://localhost:5175"
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil || (host != "127.0.0.1" && host != "localhost") {
		failure(w, 403, "local_only")
		return
	}
	remote, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || !net.ParseIP(remote).IsLoopback() {
		failure(w, 403, "local_only")
		return
	}
	origin := r.Header.Get("Origin")
	if (origin != "" && !allowedOrigin(origin)) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		failure(w, 403, "origin")
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/api/v1/support/health" {
		writeJSON(w, 200, map[string]any{"ready": true, "local_only": true, "model": s.config.Model, "model_enabled": s.config.AllowModel, "policy_version": policy.Version, "tool_count": len(ToolDefinitions()), "max_model_rounds": maxModelRounds, "max_tool_calls": maxToolCalls})
		return
	}
	if r.Method == http.MethodGet && r.URL.Path == "/api/v1/support/session" {
		s.createSession(w, r)
		return
	}
	if r.Method != http.MethodPost || r.URL.Path != "/api/v1/support/chat" {
		failure(w, 404, "not_found")
		return
	}
	if !allowedOrigin(origin) {
		failure(w, 403, "origin")
		return
	}
	media, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if media != "application/json" {
		failure(w, 415, "json_required")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input chatInput
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF {
		failure(w, 400, "invalid_request")
		return
	}
	input.Message = strings.TrimSpace(input.Message)
	if !idPattern.MatchString(input.RequestID) || !idPattern.MatchString(input.ConversationID) || !utf8.ValidString(input.Message) || utf8.RuneCountInString(input.Message) < 1 || utf8.RuneCountInString(input.Message) > 500 ||
		(input.Locale != "zh-CN" && input.Locale != "zh-TW" && input.Locale != "en-US") || (input.Product != "" && !slugPattern.MatchString(input.Product)) {
		failure(w, 400, "invalid_request")
		return
	}
	cookie, err := r.Cookie("pi_support_session")
	if err != nil {
		failure(w, 401, "session_expired")
		return
	}
	now := time.Now()
	fingerprint := hash(input.Message + "\x00" + input.ConversationID + "\x00" + input.Locale + "\x00" + input.Product)
	s.mu.Lock()
	current := s.sessions[cookie.Value]
	if current == nil || now.Sub(current.touched) > 30*time.Minute {
		s.mu.Unlock()
		failure(w, 401, "session_expired")
		return
	}
	if previous, ok := current.cache[input.RequestID]; ok {
		s.mu.Unlock()
		if previous.fingerprint != fingerprint {
			failure(w, 409, "request_conflict")
			return
		}
		writeJSON(w, 200, previous.reply)
		return
	}
	current.minute = recent(current.minute, now)
	s.minute = recent(s.minute, now)
	if current.busy || s.active >= 2 {
		s.mu.Unlock()
		failure(w, 429, "busy")
		return
	}
	if len(current.minute) >= s.config.SessionPerMinute || len(s.minute) >= s.config.GlobalPerMinute {
		s.mu.Unlock()
		w.Header().Set("Retry-After", "60")
		failure(w, 429, "rate_limit")
		return
	}
	current.busy = true
	current.touched = now
	current.minute = append(current.minute, now)
	s.minute = append(s.minute, now)
	s.active++
	if current.conversation != input.ConversationID || current.locale != input.Locale {
		current.history = nil
		current.business = BusinessState{}
		current.conversation = input.ConversationID
		current.locale = input.Locale
	}
	history := append([]Message(nil), current.history...)
	s.mu.Unlock()
	defer func() { s.mu.Lock(); current.busy = false; s.active--; s.mu.Unlock() }()
	reply, local := localAnswer(input.Message, input.Locale)
	if local && reply.Handoff {
		current.business.Stage = 0
		current.business.Pending = ""
		current.business.Case = ""
	}
	if !local && (!s.config.AllowModel || requiresPolicy(input, current.business)) {
		reply = businessAnswer(input, &current.business)
		local = true
	}
	if !local {
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		var answer ModelAnswer
		reply, answer, err = s.modelReply(ctx, input, history, current, cookie.Value)
		if err != nil {
			if problem, ok := err.(modelFailure); ok {
				if problem.code == "rate_limit" {
					w.Header().Set("Retry-After", "60")
				}
				failure(w, problem.status, problem.code)
			} else {
				failure(w, 502, "model_unavailable")
			}
			return
		}
		s.mu.Lock()
		if reply.Mode == "model" && answer.Kind != "out_of_scope" {
			assistantJSON, _ := json.Marshal(answer)
			safeInput := emailPattern.ReplaceAllString(input.Message, "[邮箱已隐藏]")
			current.history = append(current.history, Message{Role: "user", Content: safeInput}, Message{Role: "assistant", Content: string(assistantJSON)})
			for len(current.history) > 12 {
				current.history = current.history[2:]
			}
			for historySize(current.history) > 9000 && len(current.history) > 2 {
				current.history = current.history[2:]
			}
		}
		s.mu.Unlock()
	}
	s.mu.Lock()
	current.cache[input.RequestID] = cachedReply{fingerprint, reply}
	current.cacheOrder = append(current.cacheOrder, input.RequestID)
	if len(current.cacheOrder) > 24 {
		delete(current.cache, current.cacheOrder[0])
		current.cacheOrder = current.cacheOrder[1:]
	}
	s.mu.Unlock()
	writeJSON(w, 200, reply)
}
func historySize(history []Message) int {
	size := 0
	for _, m := range history {
		size += len(m.Content)
	}
	return size
}
func (s *Service) createSession(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for key, value := range s.sessions {
		if !value.busy && now.Sub(value.touched) > 30*time.Minute {
			delete(s.sessions, key)
		}
	}
	if cookie, err := r.Cookie("pi_support_session"); err == nil {
		if current := s.sessions[cookie.Value]; current != nil {
			writeJSON(w, 200, map[string]bool{"ready": true})
			return
		}
	}
	if len(s.sessions) >= 200 {
		failure(w, 429, "busy")
		return
	}
	id := opaqueID()
	if id == "" {
		failure(w, 503, "session_unavailable")
		return
	}
	s.sessions[id] = &session{touched: now, cache: map[string]cachedReply{}}
	http.SetCookie(w, &http.Cookie{Name: "pi_support_session", Value: id, Path: "/api/v1/support", HttpOnly: true, SameSite: http.SameSiteStrictMode})
	writeJSON(w, 200, map[string]bool{"ready": true})
}
func localAnswer(input, locale string) (Reply, bool) {
	if secretPattern.MatchString(input) {
		return Reply{Text: pick(locale, "请不要发送密码、API Key、登录 token 或完整卡密。我没有把这条内容发给模型；你可以隐藏这些信息，只描述遇到的提示。", "Please hide passwords, API keys, tokens and full redemption codes. This message was not sent to the model. Describe the issue without those details.", "請勿傳送密碼、API Key、登入 token 或完整卡密。這則內容沒有傳給模型；請隱藏這些資訊，只描述遇到的提示。"), Mode: "local"}, true
	}
	if weatherPattern.MatchString(strings.TrimSpace(input)) {
		return scopeReply(locale), true
	}
	normalized := strings.ToLower(strings.Trim(input, " ?？!！。"))
	if normalized == "你好" || normalized == "您好" || normalized == "hi" || normalized == "hello" {
		return Reply{Text: pick(locale, "你好，我是派总，商城的 AI 客服。想了解哪个商品，或在哪一步遇到问题了？直接说就好。", "Hi, I’m Pi, the store’s AI support assistant. Which product or step would you like help with?", "你好，我是派總，商城的 AI 客服。想了解哪個商品，或在哪一步遇到問題了？直接說就好。"), Mode: "local"}, true
	}
	if normalized == "人工" || normalized == "转人工" || normalized == "轉人工" || normalized == "human" {
		return Reply{Text: pick(locale, "可以，人工客服时间为北京时间 07:00—24:00。点击下方“联系人工”查看联系方式；不要发送密码或完整卡密。", "Use Contact support below to reach a person. Do not share passwords or full codes.", "可以，點擊下方「聯絡人工」查看客服聯絡方式。請勿傳送密碼或完整卡密。"), Handoff: true, Mode: "local"}, true
	}
	return Reply{}, false
}
