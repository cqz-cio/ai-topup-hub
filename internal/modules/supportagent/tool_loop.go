package supportagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"
)

const maxModelRounds = 3
const maxToolCalls = 6
const maxToolsPerRound = 3
const maxModelRequestBytes = 48 * 1024
const maxToolResultBytes = 12 * 1024

type modelFailure struct {
	status int
	code   string
}

func (e modelFailure) Error() string { return e.code }

func (s *Service) takeModelSlot(current *session, now time.Time) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	current.modelMinute = recent(current.modelMinute, now)
	s.modelMinute = recent(s.modelMinute, now)
	if len(current.modelMinute) >= s.config.SessionPerMinute || len(s.modelMinute) >= s.config.GlobalPerMinute {
		return false
	}
	current.modelMinute = append(current.modelMinute, now)
	s.modelMinute = append(s.modelMinute, now)
	return true
}

func (s *Service) modelReply(ctx context.Context, input chatInput, history []Message, current *session, cookie string) (Reply, ModelAnswer, error) {
	safeInput := emailPattern.ReplaceAllString(input.Message, "[邮箱已隐藏]")
	sources := modelPlanningSources(input.Locale)
	if input.Product != "" {
		sources = append(sources, Source{ID: "context:requested_product", Text: "浏览器提供的当前商品标识为 " + input.Product + "。具体价格、库存和详情必须先通过lookup_product核验；页面浏览不代表已经购买。"})
	}
	messages, err := makeMessages(input.Locale, history, safeInput, sources)
	if err != nil {
		return Reply{}, ModelAnswer{}, modelFailure{503, "knowledge_unavailable"}
	}
	executor := ToolExecutor{Knowledge: s.knowledge}
	product := current.business.Product
	if current.business.Page != input.Product {
		product = policy.Products[input.Product]
	}
	if named := detectProduct(input.Message); named != "" {
		product = named
	}
	tc := ToolContext{Input: input, History: history, Product: product}
	seenIDs := map[string]bool{}
	cached := map[string]ToolResult{}
	used := []string{}
	forceHandoff := false
	count := 0
	for round := 0; round < maxModelRounds; round++ {
		if ctx.Err() != nil {
			return Reply{}, ModelAnswer{}, modelFailure{502, "model_unavailable"}
		}
		definitions := ToolDefinitions()
		if round == maxModelRounds-1 || count >= maxToolCalls {
			definitions = nil
		}
		encoded, _ := json.Marshal(struct {
			Messages []Message        `json:"messages"`
			Tools    []ToolDefinition `json:"tools,omitempty"`
		}{messages, definitions})
		if len(encoded) > maxModelRequestBytes {
			return Reply{}, ModelAnswer{}, modelFailure{503, "knowledge_unavailable"}
		}
		now := time.Now()
		if !s.takeModelSlot(current, now) {
			return Reply{}, ModelAnswer{}, modelFailure{429, "rate_limit"}
		}
		if s.ledger == nil {
			return Reply{}, ModelAnswer{}, modelFailure{503, "budget_unavailable"}
		}
		ledgerID := hash(cookie + ":" + input.RequestID)
		if round > 0 {
			ledgerID = hash(ledgerID + ":" + strconv.Itoa(round))
		}
		// Every upstream call, including selection/final synthesis, has its own
		// durable reservation. Schemas and tool results are included in input cost.
		if err := s.ledger.Reserve(ledgerID, hash(cookie), usageCost(len(encoded)+2048, s.config.MaxOutput), s.config, now); err != nil {
			switch {
			case errors.Is(err, ErrBudget):
				return Reply{}, ModelAnswer{}, modelFailure{429, "daily_limit"}
			case errors.Is(err, ErrDuplicate):
				return Reply{}, ModelAnswer{}, modelFailure{409, "already_submitted"}
			default:
				return Reply{}, ModelAnswer{}, modelFailure{503, "budget_unavailable"}
			}
		}
		completion, providerErr := s.provider.Complete(ctx, messages, definitions)
		if completion.Input > 0 {
			if err := s.ledger.Settle(ledgerID, completion.Input, completion.Output); err != nil {
				return Reply{}, ModelAnswer{}, modelFailure{503, "budget_unavailable"}
			}
		}
		if providerErr != nil {
			log.Print("support provider request failed; no retry")
			return Reply{}, ModelAnswer{}, modelFailure{502, "model_unavailable"}
		}
		if len(completion.ToolCalls) == 0 {
			reply, err := validateAnswer(completion.Answer, sources, input.Locale)
			if err != nil {
				// validateAnswer returns fixed categories, never model/user text.
				log.Printf("support final answer rejected: %s", err)
				return Reply{}, ModelAnswer{}, modelFailure{502, "answer_unverified"}
			}
			reply.Handoff = reply.Handoff || forceHandoff
			reply.ToolsUsed = used
			return reply, completion.Answer, nil
		}
		if len(definitions) == 0 || len(completion.ToolCalls) > maxToolsPerRound || count+len(completion.ToolCalls) > maxToolCalls {
			return Reply{}, ModelAnswer{}, modelFailure{502, "answer_unverified"}
		}
		// Validate the whole batch before any tool runs. Unknown names, URL/SQL
		// arguments, malformed JSON and duplicate call IDs cannot reach adapters.
		for _, call := range completion.ToolCalls {
			if call.Type != "function" || !toolCallIDPattern.MatchString(call.ID) || seenIDs[call.ID] {
				return Reply{}, ModelAnswer{}, modelFailure{502, "answer_unverified"}
			}
			seenIDs[call.ID] = true
			if _, err := decodeToolArguments(call.Function.Name, call.Function.Arguments); err != nil {
				log.Print("support tool arguments rejected")
				return Reply{}, ModelAnswer{}, modelFailure{502, "answer_unverified"}
			}
		}
		messages = append(messages, Message{Role: "assistant", Content: completion.Content, ToolCalls: completion.ToolCalls})
		var direct *Reply
		var directSource string
		for _, call := range completion.ToolCalls {
			var normalized map[string]string
			_ = json.Unmarshal([]byte(call.Function.Arguments), &normalized)
			canonical, _ := json.Marshal(normalized)
			key := call.Function.Name + ":" + string(canonical)
			result, ok := cached[key]
			if !ok {
				var err error
				result, err = executor.Execute(ctx, call, tc)
				if err != nil {
					return Reply{}, ModelAnswer{}, modelFailure{502, "answer_unverified"}
				}
				cached[key] = result
			}
			count++
			used = append(used, call.Function.Name)
			result.SourceID = "tool:" + call.ID
			payload, err := json.Marshal(result)
			if err != nil || len(payload) > maxToolResultBytes {
				return Reply{}, ModelAnswer{}, modelFailure{503, "knowledge_unavailable"}
			}
			messages = append(messages, Message{Role: "tool", ToolCallID: call.ID, Content: string(payload)})
			sources = append(sources, Source{ID: result.SourceID, Text: string(payload), Links: result.Links})
			forceHandoff = forceHandoff || result.Status == "handoff"
			if result.Direct != nil && direct == nil {
				direct = result.Direct
				directSource = result.SourceID
			}
		}
		// Exact monetary results and handoff preparation are already complete.
		// Render the server result directly: never ask the model to recalculate it
		// or to assert that an external action happened. No extra model charge.
		if direct != nil {
			reply := *direct
			reply.Mode = "model"
			reply.ToolsUsed = used
			kind := "clarify"
			if reply.Handoff {
				kind = "handoff"
			}
			return reply, ModelAnswer{Kind: kind, Answer: reply.Text, SourceIDs: []string{directSource}}, nil
		}
	}
	return Reply{}, ModelAnswer{}, fmt.Errorf("unreachable model loop")
}
