package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type NativeToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}
type NativeMessage struct {
	Role             string           `json:"role"`
	Content          any              `json:"content,omitempty"`
	ReasoningContent string           `json:"reasoning_content,omitempty"`
	ToolCalls        []NativeToolCall `json:"tool_calls,omitempty"`
	ToolCallID       string           `json:"tool_call_id,omitempty"`
}
type NativeTool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}
type NativeRound struct {
	Message    NativeMessage
	Completion CompletionResponse
}
type NativeToolProvider interface {
	CompleteNative(context.Context, []NativeMessage, Settings, []NativeTool) (NativeRound, error)
}

func (c *DeepSeekClient) CompleteNative(ctx context.Context, messages []NativeMessage, settings Settings, tools []NativeTool) (NativeRound, error) {
	if c.APIKey == "" {
		return NativeRound{}, errors.New("DEEPSEEK_API_KEY is not set; export it before starting advent-agentd and submitting chat")
	}
	bodyObj := struct {
		Model      string          `json:"model"`
		Messages   []NativeMessage `json:"messages"`
		Tools      []NativeTool    `json:"tools,omitempty"`
		ToolChoice string          `json:"tool_choice,omitempty"`
		Thinking   struct {
			Type string `json:"type"`
		} `json:"thinking"`
		Reasoning   string   `json:"reasoning_effort,omitempty"`
		Stream      bool     `json:"stream"`
		Temperature *float64 `json:"temperature,omitempty"`
	}{Model: settings.Model, Messages: messages, Tools: tools, Stream: false, Temperature: settings.Temperature}
	bodyObj.Thinking.Type = "disabled"
	if settings.Reasoning != "none" {
		bodyObj.Thinking.Type = "enabled"
		bodyObj.Reasoning = settings.Reasoning
	}
	if len(tools) > 0 {
		bodyObj.ToolChoice = "auto"
	} else {
		bodyObj.ToolChoice = "none"
	}
	body, err := json.Marshal(bodyObj)
	if err != nil {
		return NativeRound{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return NativeRound{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	started := time.Now()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return NativeRound{}, fmt.Errorf("send request: %s", c.Logger.Redact(err.Error()))
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return NativeRound{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return NativeRound{}, fmt.Errorf("DeepSeek API returned %s: %s", resp.Status, c.Logger.Redact(string(data)))
	}
	var raw struct {
		Usage struct {
			Completion int `json:"completion_tokens"`
			Prompt     int `json:"prompt_tokens"`
			Hit        int `json:"prompt_cache_hit_tokens"`
			Miss       int `json:"prompt_cache_miss_tokens"`
			Total      int `json:"total_tokens"`
			Details    struct {
				Reasoning int `json:"reasoning_tokens"`
			} `json:"completion_tokens_details"`
		} `json:"usage"`
		Choices []struct {
			Message      NativeMessage `json:"message"`
			FinishReason string        `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return NativeRound{}, err
	}
	if len(raw.Choices) == 0 {
		return NativeRound{}, errors.New("DeepSeek response has no choices")
	}
	msg := raw.Choices[0].Message
	if len(msg.ToolCalls) == 0 && strings.TrimSpace(fmt.Sprint(msg.Content)) == "" {
		return NativeRound{}, errors.New("DeepSeek response does not contain an answer")
	}
	u := Usage{PromptTokens: raw.Usage.Prompt, PromptCacheHitTokens: raw.Usage.Hit, PromptCacheMissTokens: raw.Usage.Miss, CompletionTokens: raw.Usage.Completion, ReasoningTokens: raw.Usage.Details.Reasoning, TotalTokens: raw.Usage.Total}
	cost := estimateCost(settings.Model, u)
	duration := time.Since(started)
	call := CallMetrics{StartedAt: started.UTC(), Duration: duration, Usage: u, CostUSD: cost, FinishReason: raw.Choices[0].FinishReason}
	completion := CompletionResponse{Answer: fmt.Sprint(msg.Content), Usage: u, Metrics: Metrics{Requests: 1, Duration: duration, Usage: u, CostUSD: cost}, Call: call}
	if msg.Content == nil {
		completion.Answer = ""
	}
	return NativeRound{Message: msg, Completion: completion}, nil
}
