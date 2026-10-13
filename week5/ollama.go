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
	"sync"
	"time"
)

const LocalModel = "hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M"
const purposeContextKey contextKey = "completion-purpose"
const ragInvariantEnvelopeKey contextKey = "rag-invariant-envelope"
const ragEvidenceKey contextKey = "rag-evidence"

// Ollama constrains syntax; the existing validator still checks citations and quotes.
var localGroundedSchema = json.RawMessage(`{"type":"object","properties":{"status":{"type":"string","enum":["answered","unknown"]},"answer":{"type":"string"},"quotes":{"type":"array","items":{"type":"object","properties":{"chunk_id":{"type":"string"},"text":{"type":"string"}},"required":["chunk_id","text"],"additionalProperties":false}},"clarification":{"type":"string"}},"required":["status","answer","quotes","clarification"],"additionalProperties":false}`)
var localScoreSchema = json.RawMessage(`{"type":"object","properties":{"scores":{"type":"array","items":{"type":"object","properties":{"id":{"type":"string"},"score":{"type":"integer","minimum":0,"maximum":3}},"required":["id","score"],"additionalProperties":false}}},"required":["scores"],"additionalProperties":false}`)

const localCapabilityPrompt = "You are an offline local chat assistant. You have no tools, file access, shell access, internet access, or live documentation lookup. When retrieved references are supplied, use only that evidence and follow the requested grounded JSON format. Otherwise answer using the supplied conversation and your learned knowledge. Return text, never tool-call syntax. Do not claim to have searched or read files. For questions requiring current information, including the latest software release, state that you cannot verify it offline; do not invent a current version."

func withLocalCapabilities(messages []Message) []Message {
	out := append([]Message(nil), messages...)
	if len(out) > 0 && out[0].Role == "system" {
		if !strings.HasPrefix(out[0].Content, localCapabilityPrompt) {
			out[0].Content = localCapabilityPrompt + "\n\n" + out[0].Content
		}
		return out
	}
	return append([]Message{{Role: "system", Content: localCapabilityPrompt}}, out...)
}

func completionPurpose(ctx context.Context) string {
	purpose, _ := ctx.Value(purposeContextKey).(string)
	return purpose
}

func validateOllamaSettings(s Settings) error {
	if strings.TrimSpace(s.Model) == "" {
		return errors.New("Ollama model is required")
	}
	if strings.HasSuffix(strings.ToLower(s.Model), ":cloud") {
		return errors.New("Ollama cloud models are not supported by the local provider")
	}
	if s.Reasoning != "none" {
		return errors.New("local provider requires reasoning=none")
	}
	if s.Temperature != nil && (*s.Temperature < 0 || *s.Temperature > 2) {
		return errors.New("temperature must be between 0 and 2")
	}
	return nil
}

func (c OllamaConfig) Validate(window int) error {
	u, err := url.Parse(c.Endpoint)
	if err != nil || u.Scheme != "http" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return errors.New("ollama.endpoint must be a local HTTP server URL")
	}
	ip := net.ParseIP(u.Hostname())
	if u.Hostname() != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("ollama.endpoint must use a loopback host")
	}
	if c.OutputTokens <= 0 || c.SummaryTokens <= 0 || c.OutputTokens+256 >= window || c.SummaryTokens+256 >= window {
		return errors.New("Ollama output and summary limits must leave room for input context")
	}
	if d, err := time.ParseDuration(c.KeepAlive); err != nil || d < 0 {
		return errors.New("ollama.keep_alive must be a nonnegative duration")
	}
	return validateOllamaSettings(Settings{Provider: "ollama", Model: c.Model, Reasoning: "none"})
}

// This is deliberately conservative, not a tokenizer. Instructions and the
// current user turn are never silently trimmed to pass this guard.
func localInputEstimate(messages []Message) int {
	n := 128
	for _, m := range messages {
		n += len(m.Content) + len(m.Role) + 64
	}
	return n
}
func checkLocalBudget(messages []Message, window, output int) error {
	estimate := localInputEstimate(messages)
	if estimate > window-output {
		return fmt.Errorf("%w: conservative local input estimate %d exceeds budget %d; shorten instructions, memory or the current message, or start a new session", ErrContextWindowExceeded, estimate, window-output)
	}
	return nil
}

type OllamaClient struct {
	HTTP          *http.Client
	Config        OllamaConfig
	ContextWindow int
	Logger        *DebugLogger
	mu            sync.Mutex
	gate          chan struct{}
}

func NewConfiguredProvider(cfg Config, httpClient *http.Client, logger *DebugLogger, key string) (Provider, error) {
	switch cfg.Provider.Name {
	case "llamacpp":
		if err := cfg.LlamaCPP.Validate(cfg.Agent.ContextWindowTokens); err != nil {
			return nil, err
		}
		return &LlamaCPPClient{HTTP: httpClient, Config: cfg.LlamaCPP, ContextWindow: cfg.Agent.ContextWindowTokens, gate: make(chan struct{}, 1)}, nil
	case "ollama":
		if err := cfg.Ollama.Validate(cfg.Agent.ContextWindowTokens); err != nil {
			return nil, err
		}
		return &OllamaClient{HTTP: httpClient, Config: cfg.Ollama, ContextWindow: cfg.Agent.ContextWindowTokens, Logger: logger, gate: make(chan struct{}, 1)}, nil
	case "deepseek":
		return &DeepSeekClient{HTTP: httpClient, Endpoint: cfg.DeepSeek.Endpoint, APIKey: key, Logger: logger}, nil
	default:
		return nil, fmt.Errorf("unknown provider %q", cfg.Provider.Name)
	}
}

func (c *OllamaClient) post(ctx context.Context, path string, body any, out any) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", strings.TrimRight(c.Config.Endpoint, "/")+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	// Never follow redirects to an external service.
	httpClient := *c.HTTP
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama unavailable or request interrupted; check ollama serve: %w", err)
	}
	defer resp.Body.Close()
	data, err = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("read Ollama response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Ollama %s: HTTP %d: %s", path, resp.StatusCode, data)
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("decode Ollama %s: %w", path, err)
	}
	return nil
}

func (c *OllamaClient) inspect(ctx context.Context, model string) error {
	var metadata struct {
		RemoteModel string `json:"remote_model"`
		RemoteHost  string `json:"remote_host"`
	}
	if err := c.post(ctx, "/api/show", map[string]string{"model": model}, &metadata); err != nil {
		return fmt.Errorf("cannot inspect local model %q; install it with ollama pull: %w", model, err)
	}
	if metadata.RemoteModel != "" || metadata.RemoteHost != "" {
		return errors.New("Ollama model is remote; local provider refuses cloud generation")
	}
	return nil
}

func (c *OllamaClient) Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error) {
	if request.Settings.Provider != "ollama" {
		return CompletionResponse{}, errors.New("Ollama provider received nonlocal settings")
	}
	if err := validateOllamaSettings(request.Settings); err != nil {
		return CompletionResponse{}, err
	}
	if request.Purpose == "" {
		request.Purpose = completionPurpose(ctx)
	}
	if request.Purpose != "rag_score" || !compactScorer(request.Scorer) {
		request.Messages = withLocalCapabilities(request.Messages)
	}
	output := c.Config.OutputTokens
	if request.Purpose == "rag_score" || request.Purpose == "rag_rewrite" {
		output = 256
	}
	if request.Purpose == "summary" {
		output = c.Config.SummaryTokens
	}
	contextWindow := c.ContextWindow
	if request.Purpose == "rag_score" && request.Scorer != nil {
		if err := request.Scorer.Validate(); err != nil {
			return CompletionResponse{}, err
		}
		scoring := request.Scorer.Effective()
		output = scoring.OutputTokens
		if scoring.ContextWindowTokens > 0 {
			contextWindow = scoring.ContextWindowTokens
		}
		request.Settings.Temperature = scoring.Temperature
	}
	if err := checkLocalBudget(request.Messages, contextWindow, output); err != nil {
		return CompletionResponse{}, err
	}
	c.mu.Lock()
	if c.gate == nil {
		c.gate = make(chan struct{}, 1)
	}
	gate := c.gate
	c.mu.Unlock()
	select {
	case gate <- struct{}{}:
		defer func() { <-gate }()
	case <-ctx.Done():
		return CompletionResponse{}, ctx.Err()
	}
	if err := c.inspect(ctx, request.Settings.Model); err != nil {
		return CompletionResponse{}, err
	}
	options := map[string]any{"num_ctx": contextWindow, "num_predict": output, "temperature": 1.0, "top_p": 0.95, "top_k": -1}
	if request.Settings.Temperature != nil {
		options["temperature"] = *request.Settings.Temperature
	}
	body := map[string]any{"model": request.Settings.Model, "messages": request.Messages, "stream": false, "think": false, "keep_alive": c.Config.KeepAlive, "options": options}
	if request.Settings.RAG || strings.HasPrefix(request.Purpose, "rag_") {
		body["format"] = "json"
		if request.Purpose == "rag_score" {
			body["format"] = localScoreSchema
			if request.Scorer != nil && request.Scorer.PromptVersion == "compact-v2" {
				schema, err := orderedScoreSchema(request.Messages)
				if err != nil {
					return CompletionResponse{}, err
				}
				body["format"] = schema
			}
		}
		if request.Settings.RAG && ctx.Value(ragInvariantEnvelopeKey) != true {
			body["format"] = groundedLocalSchema(ctx)
		}
	}
	var raw struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Done           bool          `json:"done"`
		DoneReason     string        `json:"done_reason"`
		Error          string        `json:"error"`
		Prompt         int           `json:"prompt_eval_count"`
		Generated      int           `json:"eval_count"`
		LoadDuration   time.Duration `json:"load_duration"`
		PromptDuration time.Duration `json:"prompt_eval_duration"`
		EvalDuration   time.Duration `json:"eval_duration"`
	}
	started := time.Now()
	if err := c.post(ctx, "/api/chat", body, &raw); err != nil {
		return CompletionResponse{}, err
	}
	if raw.Error != "" {
		return CompletionResponse{}, fmt.Errorf("Ollama generation: %s", raw.Error)
	}
	if !raw.Done || raw.DoneReason == "" {
		return CompletionResponse{}, errors.New("Ollama returned an incomplete response")
	}
	if strings.TrimSpace(raw.Message.Content) == "" {
		return CompletionResponse{}, errors.New("Ollama returned an empty answer")
	}
	usage := Usage{PromptTokens: raw.Prompt, CompletionTokens: raw.Generated, TotalTokens: raw.Prompt + raw.Generated}
	call := CallMetrics{LoadDuration: raw.LoadDuration, PromptDuration: raw.PromptDuration, EvalDuration: raw.EvalDuration, StartedAt: started.UTC(), Duration: time.Since(started), Usage: usage, FinishReason: raw.DoneReason}
	response := CompletionResponse{Answer: raw.Message.Content, Usage: usage, Metrics: Metrics{Requests: 1, Duration: call.Duration, Usage: usage}, Call: call}
	if raw.DoneReason == "length" && (request.Settings.RAG || strings.HasPrefix(request.Purpose, "rag_")) {
		return response, errors.New("local RAG response reached the output token limit")
	}
	return response, nil
}

// Restrict generated source IDs to the actual immutable evidence, not text parsed
// from potentially hostile source content. Quote text still requires validation.
func groundedLocalSchema(ctx context.Context) any {
	e, _ := ctx.Value(ragEvidenceKey).(*Evidence)
	if e == nil || len(e.Chunks) == 0 {
		return localGroundedSchema
	}
	var schema map[string]any
	_ = json.Unmarshal(localGroundedSchema, &schema)
	ids := []string{}
	for _, c := range e.Chunks {
		ids = append(ids, c.ID)
	}
	properties := schema["properties"].(map[string]any)
	quotes := properties["quotes"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)
	quotes["chunk_id"].(map[string]any)["enum"] = ids
	return schema
}
