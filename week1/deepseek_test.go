package agent

import (
	"context"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
)

func TestDeepSeekCompletionKeepsUsageAndFinishReason(t *testing.T) {
	cfg := DefaultConfig()
	httpClient := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		body := `{"choices":[{"message":{"content":"partial"},"finish_reason":"length"}],"usage":{"prompt_tokens":100,"prompt_cache_hit_tokens":20,"prompt_cache_miss_tokens":80,"completion_tokens":30,"total_tokens":130,"completion_tokens_details":{"reasoning_tokens":7}}}`
		return response(http.StatusOK, body), nil
	})}
	client := DeepSeekClient{HTTP: httpClient, Endpoint: "https://example.invalid", APIKey: "secret", Logger: NewDebugLogger(cfg.Daemon, "secret")}
	completion, err := client.Complete(context.Background(), []Message{{Role: "user", Content: "hello"}}, DefaultSettings(cfg))
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
	_, err := client.Complete(context.Background(), []Message{{Role: "user", Content: "hello"}}, DefaultSettings(cfg))
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
