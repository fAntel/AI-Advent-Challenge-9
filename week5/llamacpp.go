package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// LlamaCPPConfig describes a private server reached through a loopback SSH tunnel.
// It deliberately has no API key or fallback provider.
type LlamaCPPConfig struct {
	Endpoint     string   `toml:"endpoint"`
	Model        string   `toml:"model"`
	OutputTokens int      `toml:"output_tokens"`
	MinInterval  Duration `toml:"min_interval"`
}

func (c LlamaCPPConfig) Validate(window int) error {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("llamacpp.endpoint must be a loopback HTTP server URL without a path")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("llamacpp.endpoint must use a loopback SSH tunnel")
	}
	if strings.TrimSpace(c.Model) == "" || c.OutputTokens <= 0 || c.OutputTokens+256 >= window || c.MinInterval < 0 {
		return errors.New("invalid llama.cpp model, output budget or minimum request interval")
	}
	return nil
}

func validateLlamaCPPSettings(s Settings) error {
	if strings.TrimSpace(s.Model) == "" || s.Reasoning != "none" {
		return errors.New("llama.cpp requires a model and reasoning=none")
	}
	if s.Temperature != nil && (*s.Temperature < 0 || *s.Temperature > 2) {
		return errors.New("temperature must be between 0 and 2")
	}
	if s.RAG || s.Compression || (s.ContextStrategy != "" && s.ContextStrategy != "full") {
		return errors.New("private llama.cpp chat requires full history without RAG or compression; start a new session when context is full")
	}
	return nil
}

const privateCapabilityPrompt = "You are a chat assistant. Reply briefly in plain text. You cannot use tools or access files or the internet."

func privateMessages(messages []Message) []Message {
	out := append([]Message(nil), messages...)
	if len(out) > 0 && out[0].Role == "system" {
		out[0].Content = privateCapabilityPrompt + "\n\n" + out[0].Content
		return out
	}
	return append([]Message{{Role: "system", Content: privateCapabilityPrompt}}, out...)
}

type LlamaCPPClient struct {
	HTTP          *http.Client
	Config        LlamaCPPConfig
	ContextWindow int
	gate          chan struct{}
	lastStart     time.Time // Only accessed while holding gate.
}

func (c *LlamaCPPClient) request(ctx context.Context, method, path string, body any, out any) error {
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(c.Config.Endpoint, "/")+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := *c.HTTP
	client.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	// Loopback tunnel traffic must not be forwarded through an environment proxy.
	if client.Transport == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = nil
		client.Transport = transport
		defer transport.CloseIdleConnections()
	} else if transport, ok := client.Transport.(*http.Transport); ok {
		copy := transport.Clone()
		copy.Proxy = nil
		client.Transport = copy
		defer copy.CloseIdleConnections()
	}
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("private LLM unavailable; check the SSH tunnel: %w", err)
	}
	defer response.Body.Close()
	data, err = io.ReadAll(io.LimitReader(response.Body, (2<<20)+1))
	if err != nil {
		return fmt.Errorf("read private LLM response: %w", err)
	}
	if len(data) > 2<<20 {
		return errors.New("private LLM response exceeds 2 MiB")
	}
	if response.StatusCode != http.StatusOK {
		if response.StatusCode == http.StatusTooManyRequests {
			return fmt.Errorf("private LLM HTTP 429: rate limit exceeded or server busy (Retry-After: %s); wait and use /retry", response.Header.Get("Retry-After"))
		}
		var envelope struct {
			Error struct {
				Type string `json:"type"`
			} `json:"error"`
		}
		_ = json.Unmarshal(data, &envelope)
		if response.StatusCode == 400 && envelope.Error.Type == "exceed_context_size_error" {
			return fmt.Errorf("%w: private LLM rejected the prompt: %s", ErrContextWindowExceeded, data)
		}
		return fmt.Errorf("private LLM %s HTTP %d: %s", path, response.StatusCode, data)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode private LLM response: %w", err)
	}
	return nil
}

func (c *LlamaCPPClient) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	if request.Settings.Provider != "llamacpp" {
		return CompletionResponse{}, errors.New("llama.cpp provider received mismatched settings")
	}
	if err := validateLlamaCPPSettings(request.Settings); err != nil {
		return CompletionResponse{}, err
	}
	if request.Purpose != "" && request.Purpose != "answer" {
		return CompletionResponse{}, errors.New("private llama.cpp provider supports chat answers only")
	}
	messages := privateMessages(request.Messages)
	if err := checkLocalBudget(messages, c.ContextWindow, c.Config.OutputTokens); err != nil {
		return CompletionResponse{}, err
	}
	select {
	case c.gate <- struct{}{}:
		defer func() { <-c.gate }()
	case <-ctx.Done():
		return CompletionResponse{}, ctx.Err()
	}
	var models struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := c.request(ctx, "GET", "/v1/models", nil, &models); err != nil {
		return CompletionResponse{}, err
	}
	found := false
	for _, model := range models.Data {
		if model.ID == request.Settings.Model {
			found = true
		}
	}
	if !found {
		return CompletionResponse{}, fmt.Errorf("model %q is not available on the private server", request.Settings.Model)
	}
	if delay := time.Until(c.lastStart.Add(time.Duration(c.Config.MinInterval))); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return CompletionResponse{}, ctx.Err()
		}
	}
	temperature := 0.0
	if request.Settings.Temperature != nil {
		temperature = *request.Settings.Temperature
	}
	body := map[string]any{"model": request.Settings.Model, "messages": messages, "stream": false, "max_tokens": c.Config.OutputTokens, "temperature": temperature}
	var raw struct {
		Choices []struct {
			Message struct {
				Role      string          `json:"role"`
				Content   string          `json:"content"`
				ToolCalls json.RawMessage `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage struct {
			Prompt     int `json:"prompt_tokens"`
			Completion int `json:"completion_tokens"`
			Total      int `json:"total_tokens"`
		} `json:"usage"`
		Timings struct {
			PromptMS    float64 `json:"prompt_ms"`
			PredictedMS float64 `json:"predicted_ms"`
		} `json:"timings"`
	}
	started := time.Now()
	c.lastStart = started
	if err := c.request(ctx, "POST", "/v1/chat/completions", body, &raw); err != nil {
		return CompletionResponse{}, err
	}
	if len(raw.Choices) != 1 {
		return CompletionResponse{}, errors.New("private LLM returned no single chat choice")
	}
	choice := raw.Choices[0]
	if choice.Message.Role != "assistant" || strings.TrimSpace(choice.Message.Content) == "" || (choice.FinishReason != "stop" && choice.FinishReason != "length") {
		return CompletionResponse{}, errors.New("private LLM returned an empty, incomplete or unsupported chat response")
	}
	if len(choice.Message.ToolCalls) > 0 && string(choice.Message.ToolCalls) != "null" && string(choice.Message.ToolCalls) != "[]" {
		return CompletionResponse{}, errors.New("private LLM attempted unsupported tool calls")
	}
	usage := Usage{PromptTokens: raw.Usage.Prompt, CompletionTokens: raw.Usage.Completion, TotalTokens: raw.Usage.Total}
	call := CallMetrics{StartedAt: started.UTC(), Duration: time.Since(started), Usage: usage, FinishReason: choice.FinishReason, PromptDuration: time.Duration(raw.Timings.PromptMS * float64(time.Millisecond)), EvalDuration: time.Duration(raw.Timings.PredictedMS * float64(time.Millisecond))}
	return CompletionResponse{Answer: choice.Message.Content, Usage: usage, Call: call, Metrics: Metrics{Requests: 1, Duration: call.Duration, Usage: usage}}, nil
}
