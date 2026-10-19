package supportagent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}
type ModelAnswer struct {
	Kind      string   `json:"kind"`
	Answer    string   `json:"answer"`
	SourceIDs []string `json:"source_ids"`
}
type Completion struct {
	Answer        ModelAnswer
	Input, Output int
	ToolCalls     []ToolCall
	Content       string
}
type Provider interface {
	Complete(context.Context, []Message, []ToolDefinition) (Completion, error)
}
type DeepSeek struct {
	config   Config
	client   *http.Client
	endpoint string
}

func NewDeepSeek(c Config) *DeepSeek {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	return &DeepSeek{config: c, endpoint: "https://api.deepseek.com/chat/completions", client: &http.Client{
		Transport: transport, Timeout: 35 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}
func (p *DeepSeek) Complete(ctx context.Context, messages []Message, tools []ToolDefinition) (Completion, error) {
	result := Completion{}
	// Only the server-owned, read-only registry is exposed. There is no arbitrary
	// URL, SQL, shell, payment execution or client-selected model tool.
	payload := struct {
		Model      string            `json:"model"`
		Messages   []Message         `json:"messages"`
		MaxTokens  int               `json:"max_tokens"`
		Stream     bool              `json:"stream"`
		Thinking   map[string]string `json:"thinking"`
		Format     map[string]string `json:"response_format"`
		Tools      []ToolDefinition  `json:"tools,omitempty"`
		ToolChoice string            `json:"tool_choice"`
	}{Model: p.config.Model, Messages: messages, MaxTokens: p.config.MaxOutput,
		Thinking: map[string]string{"type": "disabled"}, Format: map[string]string{"type": "json_object"},
		Tools: tools, ToolChoice: "none"}
	if len(tools) > 0 {
		payload.ToolChoice = "auto"
	}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return result, errors.New("provider request unavailable")
	}
	request.Header.Set("Authorization", "Bearer "+p.config.APIKey)
	request.Header.Set("Content-Type", "application/json")
	response, err := p.client.Do(request)
	if err != nil {
		return result, errors.New("provider unavailable or timed out")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("provider HTTP status %d", response.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(response.Body, 96*1024+1))
	if err != nil || len(b) > 96*1024 {
		return result, errors.New("invalid provider response")
	}
	var data struct {
		Choices []struct {
			Message struct {
				Content string     `json:"content"`
				Tools   []ToolCall `json:"tool_calls"`
			} `json:"message"`
			Finish string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			Input  int `json:"prompt_tokens"`
			Output int `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(b, &data) != nil || len(data.Choices) != 1 {
		return result, errors.New("invalid provider response")
	}
	result.Input = data.Usage.Input
	result.Output = data.Usage.Output
	if data.Choices[0].Finish == "tool_calls" && len(data.Choices[0].Message.Tools) > 0 && len(tools) > 0 {
		result.ToolCalls = data.Choices[0].Message.Tools
		result.Content = data.Choices[0].Message.Content
		return result, nil
	}
	if data.Choices[0].Finish != "stop" || len(data.Choices[0].Message.Tools) > 0 {
		return result, errors.New("incomplete provider answer")
	}
	if err := json.Unmarshal([]byte(data.Choices[0].Message.Content), &result.Answer); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			return result, errors.New("invalid provider answer field type")
		}
		return result, errors.New("invalid provider answer JSON")
	}
	return result, nil
}
