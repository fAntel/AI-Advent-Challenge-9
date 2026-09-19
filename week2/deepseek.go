package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	FlashModel = "deepseek-v4-flash"
	ProModel   = "deepseek-v4-pro"
)

func DeepSeekAPIKey() (string, error) {
	key := os.Getenv("DEEPSEEK_API_KEY")
	if key == "" {
		return "", errors.New("DEEPSEEK_API_KEY is not set")
	}
	return key, nil
}

func ValidateDeepSeekSettings(s Settings) error {
	if s.Model != FlashModel && s.Model != ProModel {
		return fmt.Errorf("unknown DeepSeek model %q", s.Model)
	}
	switch s.Reasoning {
	case "none", "low", "high", "max":
	default:
		return fmt.Errorf("unknown DeepSeek reasoning %q", s.Reasoning)
	}
	if s.Temperature != nil && (*s.Temperature < 0 || *s.Temperature > 2) {
		return errors.New("temperature must be between 0 and 2")
	}
	if s.Temperature != nil && s.Reasoning != "none" {
		return errors.New("temperature cannot be used when reasoning is enabled")
	}
	return nil
}

type DeepSeekClient struct {
	HTTP             *http.Client
	Endpoint, APIKey string
	Logger           *DebugLogger
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Thinking struct {
		Type string `json:"type"`
	} `json:"thinking"`
	Reasoning   string   `json:"reasoning_effort,omitempty"`
	Stream      bool     `json:"stream"`
	Temperature *float64 `json:"temperature,omitempty"`
}
type chatResponse struct {
	Usage struct {
		Completion, Prompt, Hit, Miss, Total int
		Details                              struct {
			Reasoning int `json:"reasoning_tokens"`
		} `json:"completion_tokens_details"`
	} `json:"-"`
	Choices []struct {
		Message struct {
			Content *string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func (c *DeepSeekClient) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	messages, settings := request.Messages, request.Settings
	var reqBody chatRequest
	reqBody.Model, reqBody.Messages, reqBody.Temperature = settings.Model, messages, settings.Temperature
	reqBody.Thinking.Type = "disabled"
	if settings.Reasoning != "none" {
		reqBody.Thinking.Type, reqBody.Reasoning = "enabled", settings.Reasoning
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return Completion{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Completion{}, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	diagnostic := dumpRequest(req, body, c.Logger)
	sessionID, _ := ctx.Value(sessionContextKey).(string)
	operationID, _ := ctx.Value(operationContextKey).(string)
	if c.Logger.Enabled() {
		c.Logger.Log("http.request", sessionID, operationID, diagnostic)
	}
	started := time.Now()
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return Completion{Diagnostics: diagnostic}, fmt.Errorf("send request: %s", c.Logger.Redact(err.Error()))
	}
	responseBody, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	duration := time.Since(started)
	diagnostic += dumpResponse(resp, responseBody, c.Logger)
	if c.Logger.Enabled() {
		c.Logger.Log("http.response", sessionID, operationID, dumpResponse(resp, responseBody, c.Logger))
	}
	if readErr != nil {
		return Completion{Diagnostics: diagnostic}, fmt.Errorf("read response: %s", c.Logger.Redact(readErr.Error()))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		lower := strings.ToLower(string(responseBody))
		if strings.Contains(lower, "context") && (strings.Contains(lower, "length") || strings.Contains(lower, "token")) {
			return Completion{Diagnostics: diagnostic}, fmt.Errorf("%w: DeepSeek API returned %s: %s", ErrContextWindowExceeded, resp.Status, c.Logger.Redact(string(responseBody)))
		}
		return Completion{Diagnostics: diagnostic}, fmt.Errorf("DeepSeek API returned %s: %s", resp.Status, c.Logger.Redact(string(responseBody)))
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
			Message struct {
				Content *string `json:"content"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &raw); err != nil {
		return Completion{Diagnostics: diagnostic}, fmt.Errorf("decode response: %w", err)
	}
	if len(raw.Choices) == 0 || raw.Choices[0].Message.Content == nil || *raw.Choices[0].Message.Content == "" {
		return Completion{Diagnostics: diagnostic}, errors.New("DeepSeek response does not contain an answer")
	}
	u := Usage{PromptTokens: raw.Usage.Prompt, PromptCacheHitTokens: raw.Usage.Hit, PromptCacheMissTokens: raw.Usage.Miss, CompletionTokens: raw.Usage.Completion, ReasoningTokens: raw.Usage.Details.Reasoning, TotalTokens: raw.Usage.Total}
	cost := estimateCost(settings.Model, u)
	call := CallMetrics{StartedAt: started.UTC(), Duration: duration, Usage: u, CostUSD: cost, FinishReason: raw.Choices[0].FinishReason}
	return Completion{Answer: *raw.Choices[0].Message.Content, Usage: u, Metrics: Metrics{Requests: 1, Duration: duration, Usage: u, CostUSD: cost}, Call: call, Diagnostics: diagnostic}, nil
}

func dumpRequest(req *http.Request, body []byte, logger *DebugLogger) string {
	var b strings.Builder
	fmt.Fprintf(&b, "> %s %s\n", req.Method, req.URL)
	writeHeaders(&b, req.Header)
	fmt.Fprintf(&b, ">\n%s\n", body)
	return logger.Redact(b.String())
}
func dumpResponse(resp *http.Response, body []byte, logger *DebugLogger) string {
	var b strings.Builder
	fmt.Fprintf(&b, "< %s\n", resp.Status)
	writeHeaders(&b, resp.Header)
	fmt.Fprintf(&b, "<\n%s\n", body)
	return logger.Redact(b.String())
}
func writeHeaders(b *strings.Builder, h http.Header) {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range h.Values(k) {
			fmt.Fprintf(b, "%s: %s\n", k, v)
		}
	}
}

func estimateCost(model string, u Usage) float64 {
	rates := [3]float64{0.0028, 0.14, 0.28}
	if model == ProModel {
		rates = [3]float64{0.003625, 0.435, 0.87}
	}
	return (float64(u.PromptCacheHitTokens)*rates[0] + float64(u.PromptCacheMissTokens)*rates[1] + float64(u.CompletionTokens)*rates[2]) / 1_000_000
}
