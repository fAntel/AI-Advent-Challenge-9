package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func privateConfig(t *testing.T) Config {
	c := testConfig(t)
	c.Provider.Name = "llamacpp"
	c.Agent.ContextWindowTokens = 1024
	c.Agent.ContextStrategy = "full"
	c.Agent.CompressionEnabled = false
	c.LlamaCPP.MinInterval = 0
	return c
}

const privateReply = `{"choices":[{"message":{"role":"assistant","content":"Hello, Mira."},"finish_reason":"stop"}],"usage":{"prompt_tokens":40,"completion_tokens":4,"total_tokens":44},"timings":{"prompt_ms":12,"predicted_ms":20}}`

func mockPrivate(t *testing.T, c Config, handler http.HandlerFunc) *LlamaCPPClient {
	t.Helper()
	s := httptest.NewServer(handler)
	t.Cleanup(s.Close)
	c.LlamaCPP.Endpoint = s.URL
	p, err := NewConfiguredProvider(c, s.Client(), nil, "must-not-be-sent")
	if err != nil {
		t.Fatal(err)
	}
	return p.(*LlamaCPPClient)
}

func privateRequest(c Config) CompletionRequest {
	return CompletionRequest{Settings: DefaultSettings(c), Messages: []Message{{Role: "user", Content: "Hello"}}}
}

func TestPrivateRequestHistoryAndAccounting(t *testing.T) {
	c := privateConfig(t)
	p := mockPrivate(t, c, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("sent credentials")
		}
		if r.URL.Path == "/v1/models" {
			w.Write([]byte(`{"data":[{"id":"private-smollm"}]}`))
			return
		}
		if r.URL.Path != "/v1/chat/completions" || r.Method != "POST" {
			t.Error(r.URL.Path)
		}
		var body struct {
			Model       string    `json:"model"`
			Messages    []Message `json:"messages"`
			Max         int       `json:"max_tokens"`
			Stream      bool      `json:"stream"`
			Temperature float64   `json:"temperature"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "private-smollm" || body.Max != 64 || body.Stream || body.Temperature != 0 || len(body.Messages) != 4 || body.Messages[2].Content != "Hello, Mira." || body.Messages[0].Content != privateCapabilityPrompt {
			t.Errorf("body: %+v", body)
		}
		w.Write([]byte(privateReply))
	})
	r := privateRequest(c)
	r.Messages = []Message{{Role: "user", Content: "I am Mira."}, {Role: "assistant", Content: "Hello, Mira."}, {Role: "user", Content: "What is my name?"}}
	response, err := p.Complete(context.Background(), r)
	if err != nil || response.Answer != "Hello, Mira." || response.Usage.TotalTokens != 44 || response.Call.EvalDuration != 20*time.Millisecond || response.Metrics.CostUSD != 0 {
		t.Fatalf("%+v, %v", response, err)
	}
}

func TestPrivateFailureAndTruncation(t *testing.T) {
	for _, tc := range []struct {
		name       string
		status     int
		body, want string
	}{
		{"rate", 429, `{}`, "429"},
		{"context", 400, `{"error":{"type":"exceed_context_size_error"}}`, "context window"},
		{"malformed", 200, `{`, "decode"},
		{"empty", 200, `{"choices":[]}`, "choice"},
		{"incomplete", 200, `{"choices":[{"message":{"role":"assistant","content":"hello"}}]}`, "incomplete"},
		{"tool", 200, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"hello","tool_calls":[{}]}}]}`, "tool calls"},
		{"truncated", 200, strings.Replace(privateReply, `"stop"`, `"length"`, 1), ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := privateConfig(t)
			var posts atomic.Int32
			p := mockPrivate(t, c, func(w http.ResponseWriter, r *http.Request) {
				if r.Method == "GET" {
					w.Write([]byte(`{"data":[{"id":"private-smollm"}]}`))
					return
				}
				posts.Add(1)
				w.Header().Set("Retry-After", "10")
				w.WriteHeader(tc.status)
				w.Write([]byte(tc.body))
			})
			got, err := p.Complete(context.Background(), privateRequest(c))
			if tc.want == "" {
				if err != nil || got.Call.FinishReason != "length" {
					t.Fatalf("%+v %v", got, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatal(err)
			}
			if posts.Load() != 1 {
				t.Fatal("unexpected retry")
			}
		})
	}
}

func TestPrivateModelBudgetRedirectAndPacing(t *testing.T) {
	c := privateConfig(t)
	var posts atomic.Int32
	p := mockPrivate(t, c, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			w.Write([]byte(`{"data":[{"id":"private-smollm"}]}`))
			return
		}
		posts.Add(1)
		w.Write([]byte(privateReply))
	})
	r := privateRequest(c)
	r.Settings.Model = "missing"
	if _, err := p.Complete(context.Background(), r); err == nil || !strings.Contains(err.Error(), "not available") {
		t.Fatal(err)
	}
	r = privateRequest(c)
	r.Messages[0].Content = strings.Repeat("x", 1024)
	if _, err := p.Complete(context.Background(), r); !errors.Is(err, ErrContextWindowExceeded) {
		t.Fatal(err)
	}
	if posts.Load() != 0 {
		t.Fatal("invalid request sent")
	}
	p.Config.MinInterval = Duration(time.Hour)
	p.lastStart = time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := p.Complete(ctx, privateRequest(c)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	if posts.Load() != 0 {
		t.Fatal("pacing cancellation sent request")
	}
	var redirected atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { redirected.Add(1) }))
	defer target.Close()
	p = mockPrivate(t, c, func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, target.URL, 302) })
	if _, err := p.Complete(context.Background(), privateRequest(c)); err == nil || !strings.Contains(err.Error(), "302") {
		t.Fatal(err)
	}
	if redirected.Load() != 0 {
		t.Fatal("followed redirect")
	}
}

func TestPrivateConfigurationAndLifecycle(t *testing.T) {
	c := privateConfig(t)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"https://example.com", "http://example.com", "http://127.0.0.1/v1", "http://user@127.0.0.1"} {
		bad := c
		bad.LlamaCPP.Endpoint = endpoint
		if bad.Validate() == nil {
			t.Fatal(endpoint)
		}
	}
	for _, change := range []func(*Settings){func(s *Settings) { s.RAG = true }, func(s *Settings) { s.Compression = true }, func(s *Settings) { s.Reasoning = "high" }, func(s *Settings) { s.ContextStrategy = "sliding" }} {
		s := DefaultSettings(c)
		change(&s)
		if ValidateSettings(s) == nil {
			t.Fatalf("accepted %+v", s)
		}
	}
	a, err := NewAgent(c, &fakeCompleter{}, NewDebugLogger(c.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err := a.Create(CreateSessionRequest{}); err == nil {
		t.Fatal("accepted task lifecycle")
	}
	s, err := a.Create(CreateSessionRequest{Chat: true})
	if err != nil {
		t.Fatal(err)
	}
	if s.Settings.Provider != "llamacpp" || s.Settings.Compression || s.ContextWindowTokens != 1024 {
		t.Fatalf("%+v", s)
	}
}
