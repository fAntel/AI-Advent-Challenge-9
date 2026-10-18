package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Usage struct {
	Prompt     int `json:"prompt_tokens"`
	Completion int `json:"completion_tokens"`
	Total      int `json:"total_tokens"`
}
type Call struct {
	Purpose  string        `json:"purpose"`
	Usage    Usage         `json:"usage"`
	Duration time.Duration `json:"duration"`
	Finish   string        `json:"finish"`
}
type CompleteFunc func(context.Context, string, []Message) (string, Call, error)

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
type ToolMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}
type ToolCompleteFunc func(context.Context, []ToolMessage, bool) (ToolMessage, Call, error)

func Provider(c Config) CompleteFunc {
	return func(ctx context.Context, purpose string, messages []Message) (string, Call, error) {
		wire := make([]ToolMessage, len(messages))
		for i, m := range messages {
			wire[i] = ToolMessage{Role: m.Role, Content: m.Content}
		}
		out, call, err := providerRequest(ctx, c, purpose, wire, false)
		return out.Content, call, err
	}
}
func ToolProvider(c Config) ToolCompleteFunc {
	return func(ctx context.Context, messages []ToolMessage, tools bool) (ToolMessage, Call, error) {
		return providerRequest(ctx, c, "agent", messages, tools)
	}
}
func providerRequest(ctx context.Context, c Config, purpose string, messages []ToolMessage, withTools bool) (ToolMessage, Call, error) {
	call := Call{Purpose: purpose}
	empty := ToolMessage{}
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		key, _ = KeychainDeepSeekKey()
	}
	if key == "" {
		return empty, call, fmt.Errorf("set DEEPSEEK_API_KEY or the advent-agent.deepseek Keychain entry")
	}
	payload := map[string]any{"model": c.Model, "messages": messages, "thinking": map[string]string{"type": "disabled"}, "stream": false, "max_tokens": 8192, "temperature": 0}
	if withTools {
		payload["tools"] = toolDefinitions()
		payload["tool_choice"] = "auto"
	} else if purpose != "summary" {
		payload["response_format"] = map[string]string{"type": "json_object"}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return empty, call, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return empty, call, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: c.Timeout(), CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("provider redirect refused") }}
	start := time.Now()
	resp, err := client.Do(req)
	call.Duration = time.Since(start)
	if err != nil {
		return empty, call, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return empty, call, err
	}
	if resp.StatusCode != 200 {
		return empty, call, fmt.Errorf("DeepSeek HTTP %d: %s", resp.StatusCode, strings.ReplaceAll(string(raw), key, "[redacted]"))
	}
	var out struct {
		Usage   Usage `json:"usage"`
		Choices []struct {
			Message ToolMessage `json:"message"`
			Finish  string      `json:"finish_reason"`
		} `json:"choices"`
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return empty, call, err
	}
	call.Usage = out.Usage
	if len(out.Choices) != 1 {
		return empty, call, fmt.Errorf("provider returned no unique answer")
	}
	result := out.Choices[0]
	call.Finish = result.Finish
	if result.Message.Role != "assistant" {
		return empty, call, fmt.Errorf("invalid provider role")
	}
	if withTools && call.Finish == "tool_calls" && len(result.Message.ToolCalls) > 0 {
		return result.Message, call, nil
	}
	if call.Finish != "stop" || len(result.Message.ToolCalls) > 0 || strings.TrimSpace(result.Message.Content) == "" {
		return result.Message, call, fmt.Errorf("incomplete provider answer (finish=%s)", call.Finish)
	}
	return result.Message, call, nil
}
