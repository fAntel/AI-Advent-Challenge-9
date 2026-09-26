package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestDeepSeekNativeToolCallsAndReasoningReplay(t *testing.T) {
	cfg := DefaultConfig()
	var sent []map[string]any
	client := DeepSeekClient{HTTP: &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		var payload map[string]any
		if err := json.NewDecoder(req.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		sent = append(sent, payload)
		if len(sent) == 1 {
			return response(200, `{"choices":[{"message":{"role":"assistant","content":null,"reasoning_content":"choose a tool","tool_calls":[{"id":"call-1","type":"function","function":{"name":"list_mcp_tools","arguments":"{\"server\":\"fixture\"}"}}]},"finish_reason":"tool_calls"}],"usage":{"prompt_tokens":10,"completion_tokens":4,"total_tokens":14}}`), nil
		}
		return response(200, `{"choices":[{"message":{"role":"assistant","content":"Done"},"finish_reason":"stop"}],"usage":{"prompt_tokens":15,"completion_tokens":2,"total_tokens":17}}`), nil
	})}, Endpoint: "https://example.invalid", APIKey: "secret", Logger: NewDebugLogger(cfg.Daemon, "secret")}
	messages := []NativeMessage{{Role: "user", Content: "find tools"}}
	first, err := client.CompleteNative(context.Background(), messages, DefaultSettings(cfg), mcpNativeTools())
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Message.ToolCalls) != 1 || first.Message.ReasoningContent != "choose a tool" || first.Completion.Usage.PromptTokens != 10 {
		t.Fatalf("round=%+v", first)
	}
	messages = append(messages, first.Message, NativeMessage{Role: "tool", ToolCallID: "call-1", Content: `{"tools":[]}`})
	second, err := client.CompleteNative(context.Background(), messages, DefaultSettings(cfg), mcpNativeTools())
	if err != nil {
		t.Fatal(err)
	}
	if second.Completion.Answer != "Done" {
		t.Fatalf("round=%+v", second)
	}
	replay := sent[1]["messages"].([]any)[1].(map[string]any)
	if replay["reasoning_content"] != "choose a tool" {
		t.Fatalf("reasoning not replayed: %+v", replay)
	}
}

func TestDeepSeekRequiresKeyOnlyAtCompletion(t *testing.T) {
	cfg := DefaultConfig()
	client := DeepSeekClient{APIKey: "", Logger: NewDebugLogger(cfg.Daemon, "")}
	_, err := client.Complete(context.Background(), CompletionRequest{Settings: DefaultSettings(cfg)})
	if err == nil || !strings.Contains(err.Error(), "DEEPSEEK_API_KEY") {
		t.Fatalf("completion error: %v", err)
	}
	_, err = client.CompleteNative(context.Background(), []NativeMessage{{Role: "user", Content: "hi"}}, DefaultSettings(cfg), nil)
	if err == nil || !strings.Contains(err.Error(), "DEEPSEEK_API_KEY") {
		t.Fatalf("native completion error: %v", err)
	}
}

func TestDeepSeekCompletionKeepsUsageAndFinishReason(t *testing.T) {
	cfg := DefaultConfig()
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}],"usage":{"prompt_tokens":100,"prompt_cache_hit_tokens":20,"prompt_cache_miss_tokens":80,"completion_tokens":30,"total_tokens":130,"completion_tokens_details":{"reasoning_tokens":7}}}`
		return response(http.StatusOK, body), nil
	})}
	client := DeepSeekClient{HTTP: httpClient, Endpoint: "https://example.invalid", APIKey: "secret", Logger: NewDebugLogger(cfg.Daemon, "secret")}
	completion, err := client.Complete(context.Background(), CompletionRequest{Messages: []Message{{Role: "user", Content: "hello"}}, Settings: DefaultSettings(cfg)})
	if err != nil {
		t.Fatal(err)
	}
	if completion.Call.FinishReason != "length" || completion.Usage.PromptTokens != 100 || completion.Usage.CompletionTokens != 30 || completion.Usage.ReasoningTokens != 7 {
		t.Fatalf("completion=%+v", completion)
	}
}

func TestDeepSeekClassifiesContextWindowError(t *testing.T) {
	cfg := DefaultConfig()
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusBadRequest, `{"error":{"message":"maximum context length exceeded by token count"}}`), nil
	})}
	client := DeepSeekClient{HTTP: httpClient, Endpoint: "https://example.invalid", APIKey: "secret", Logger: NewDebugLogger(cfg.Daemon, "secret")}
	_, err := client.Complete(context.Background(), CompletionRequest{Messages: []Message{{Role: "user", Content: "hello"}}, Settings: DefaultSettings(cfg)})
	if !errors.Is(err, ErrContextWindowExceeded) {
		t.Fatalf("error=%v", err)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestCurrentDeepSeekPricing(t *testing.T) {
	u := Usage{PromptCacheHitTokens: 100, PromptCacheMissTokens: 200, CompletionTokens: 300}
	wantFlash := (100*0.0028 + 200*0.14 + 300*0.28) / 1_000_000
	wantPro := (100*0.003625 + 200*0.435 + 300*0.87) / 1_000_000
	if got := estimateCost(FlashModel, u); math.Abs(got-wantFlash) > 1e-15 {
		t.Fatalf("flash cost=%g want=%g", got, wantFlash)
	}
	if got := estimateCost(ProModel, u); math.Abs(got-wantPro) > 1e-15 {
		t.Fatalf("pro cost=%g want=%g", got, wantPro)
	}
}
