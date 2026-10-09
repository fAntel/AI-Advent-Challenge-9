package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func localConfig(t *testing.T) Config {
	c := testConfig(t)
	c.Provider.Name = "ollama"
	c.Agent.ContextWindowTokens = 4096
	c.Agent.RecentMessages = 4
	c.Agent.SummaryBatchMessages = 2
	c.Profile = "local"
	return c
}
func localRequest(c Config) CompletionRequest {
	return CompletionRequest{Settings: DefaultSettings(c), Messages: []Message{{Role: "user", Content: "hello"}}}
}

type onlyLocalTransport struct {
	base http.RoundTripper
	host string
}

func (t onlyLocalTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	if r.URL.Host != t.host {
		return nil, errors.New("test forbids external request")
	}
	return t.base.RoundTrip(r)
}
func mockLocalClient(t *testing.T, c Config, h http.HandlerFunc) (*OllamaClient, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	c.Ollama.Endpoint = server.URL
	client := server.Client()
	client.Transport = onlyLocalTransport{http.DefaultTransport, strings.TrimPrefix(server.URL, "http://")}
	p, err := NewConfiguredProvider(c, client, NewDebugLogger(c.Daemon, ""), "")
	if err != nil {
		t.Fatal(err)
	}
	return p.(*OllamaClient), server
}

func TestOllamaRequestAndUsage(t *testing.T) {
	c := localConfig(t)
	var bodies []map[string]any
	p, _ := mockLocalClient(t, c, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("local request has credentials")
		}
		if r.URL.Path == "/api/show" {
			w.Write([]byte(`{}`))
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		bodies = append(bodies, body)
		w.Write([]byte(`{"message":{"content":"local answer"},"done":true,"done_reason":"stop","prompt_eval_count":12,"eval_count":5}`))
	})
	request := localRequest(c)
	for _, purpose := range []string{"answer", "summary"} {
		request.Purpose = purpose
		got, err := p.Complete(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if got.Answer != "local answer" || got.Usage.TotalTokens != 17 || got.Metrics.CostUSD != 0 || got.Call.Duration <= 0 {
			t.Fatalf("response: %+v", got)
		}
	}
	for i, b := range bodies {
		if b["stream"] != false || b["think"] != false || b["model"] != LocalModel || b["keep_alive"] != "10m" {
			t.Fatalf("request: %v", b)
		}
		messages := b["messages"].([]any)
		first := messages[0].(map[string]any)
		if first["role"] != "system" || !strings.Contains(first["content"].(string), "You have no tools") {
			t.Fatalf("missing local capability boundary: %v", messages)
		}
		opts := b["options"].(map[string]any)
		want := 1024
		if i == 1 {
			want = 256
		}
		if opts["num_predict"] != float64(want) || opts["num_ctx"] != float64(4096) {
			t.Fatalf("options: %v", opts)
		}
	}
}

func TestOllamaFailures(t *testing.T) {
	cases := []struct {
		name, show, chat string
		status           int
		want             string
	}{
		{"missing model", `not installed`, ``, 404, "ollama pull"},
		{"remote alias", `{"remote_model":"model","remote_host":"https://cloud.example"}`, ``, 200, "remote"},
		{"invalid JSON", `{}`, `{`, 200, "decode"},
		{"incomplete", `{}`, `{"done":false}`, 200, "incomplete"},
		{"empty", `{}`, `{"done":true,"done_reason":"stop","message":{"content":" "}}`, 200, "empty"},
		{"generation error", `{}`, `{"error":"out of memory"}`, 200, "out of memory"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := localConfig(t)
			p, _ := mockLocalClient(t, c, func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/show" {
					w.WriteHeader(tc.status)
					w.Write([]byte(tc.show))
					return
				}
				w.Write([]byte(tc.chat))
			})
			_, err := p.Complete(context.Background(), localRequest(c))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error: %v", err)
			}
		})
	}
}

func TestOllamaTruncationAndCancellation(t *testing.T) {
	c := localConfig(t)
	p, _ := mockLocalClient(t, c, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show" {
			w.Write([]byte(`{}`))
			return
		}
		w.Write([]byte(`{"done":true,"done_reason":"length","message":{"content":"partial"}}`))
	})
	got, err := p.Complete(context.Background(), localRequest(c))
	if err != nil || got.Call.FinishReason != "length" || got.Answer != "partial" {
		t.Fatalf("%+v %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = p.Complete(ctx, localRequest(c))
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	slow, _ := mockLocalClient(t, c, func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-time.After(100 * time.Millisecond):
		}
	})
	ctx, cancel = context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = slow.Complete(ctx, localRequest(c))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
}

func TestLocalValidationAndBudget(t *testing.T) {
	c := localConfig(t)
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, endpoint := range []string{"https://127.0.0.1:11434", "http://cloud.example", "http://127.0.0.1:11434/path"} {
		bad := c
		bad.Ollama.Endpoint = endpoint
		if bad.Validate() == nil {
			t.Fatalf("accepted %s", endpoint)
		}
	}
	for _, change := range []func(*Settings){func(s *Settings) { s.RAG = true }, func(s *Settings) { s.Reasoning = "high" }, func(s *Settings) { s.Model = "foo:cloud" }} {
		s := DefaultSettings(c)
		change(&s)
		if ValidateSettings(s) == nil {
			t.Fatalf("accepted %+v", s)
		}
	}
	calls := 0
	p, _ := mockLocalClient(t, c, func(w http.ResponseWriter, r *http.Request) { calls++ })
	req := localRequest(c)
	req.Messages[0].Content = strings.Repeat("界", 1100)
	if _, err := p.Complete(context.Background(), req); !errors.Is(err, ErrContextWindowExceeded) {
		t.Fatal(err)
	}
	if calls != 0 {
		t.Fatal("oversized request sent")
	}
}

func TestLocalConversationSummaryRestartAndMemory(t *testing.T) {
	c := localConfig(t)
	var mu sync.Mutex
	var received [][]Message
	summaryCalls := 0
	p, _ := mockLocalClient(t, c, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show" {
			w.Write([]byte(`{}`))
			return
		}
		var body struct {
			Messages []Message `json:"messages"`
			Options  struct {
				Predict int `json:"num_predict"`
			} `json:"options"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		received = append(received, body.Messages)
		if body.Options.Predict == 256 {
			summaryCalls++
		}
		mu.Unlock()
		answer := "Use Go; the project is named Cedar."
		json.NewEncoder(w).Encode(map[string]any{"done": true, "done_reason": "stop", "message": map[string]string{"content": answer}, "prompt_eval_count": 20, "eval_count": 10})
	})
	logger := NewDebugLogger(c.Daemon, "")
	a, err := NewAgent(c, p, logger)
	if err != nil {
		t.Fatal(err)
	}
	s, err := a.Create(CreateSessionRequest{Chat: true, Profile: "local"})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := a.Attach(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.MutateMemory(s.ID, lease, MemoryMutationRequest{Action: "create", Scope: "working", Key: "language", Value: "Go"}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"Our project is Cedar. Use Go.", "Which language?", "Which project?"} {
		if err := a.Submit(s.ID, lease, MessageRequest{Content: q}); err != nil {
			t.Fatal(err)
		}
		waitState(t, a, s.ID, "completed")
	}
	saved, _ := a.Get(s.ID)
	if saved.Summary == "" || saved.SummarizedMessages != 2 || len(saved.Messages) != 4 {
		t.Fatalf("summary: %+v", saved)
	}
	a.Close()
	a, err = NewAgent(c, p, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	lease, err = a.Attach(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "Recall our project and language."}); err != nil {
		t.Fatal(err)
	}
	waitState(t, a, s.ID, "completed")
	mu.Lock()
	defer mu.Unlock()
	if summaryCalls < 2 {
		t.Fatal("no local summary calls")
	}
	lastAnswer := received[len(received)-2]
	encoded, _ := json.Marshal(lastAnswer)
	if !strings.Contains(string(encoded), "Cedar") || !strings.Contains(string(encoded), "language = Go") {
		t.Fatalf("resumed context: %s", encoded)
	}
	wrong := DefaultSettings(c)
	wrong.Provider = "deepseek"
	wrong.Model = FlashModel
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "wrong provider", Settings: &wrong}); err == nil {
		t.Fatal("provider mismatch accepted")
	}
	if _, err := a.Create(CreateSessionRequest{Chat: false}); err == nil {
		t.Fatal("local task accepted")
	}
}

func TestLocalFailedSummaryRetainsHistory(t *testing.T) {
	for _, reason := range []string{"error", "length"} {
		t.Run(reason, func(t *testing.T) {
			c := localConfig(t)
			fake := &fakeCompleter{answers: []string{"one", "two", "three", "bad summary"}}
			if reason == "error" {
				fake.failures = []error{nil, nil, nil, errors.New("summary failed")}
			} else {
				fake.finishReasons = []string{"stop", "stop", "stop", "length"}
			}
			a, err := NewAgent(c, fake, NewDebugLogger(c.Daemon, ""))
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			s, err := a.Create(CreateSessionRequest{Chat: true})
			if err != nil {
				t.Fatal(err)
			}
			lease, _ := a.Attach(s.ID)
			for _, q := range []string{"one", "two", "three"} {
				if err := a.Submit(s.ID, lease, MessageRequest{Content: q}); err != nil {
					t.Fatal(err)
				}
				waitState(t, a, s.ID, "completed")
			}
			saved, _ := a.Get(s.ID)
			if len(saved.Messages) != 6 || saved.Summary != "" || saved.Operation.Warning == "" {
				t.Fatalf("history lost: %+v", saved)
			}
		})
	}
}

func TestLocalEarlyCompressionAndDiscard(t *testing.T) {
	c := localConfig(t)
	c.Agent.RecentMessages = 8 // Avoid normal post-answer compression.
	fake := &fakeCompleter{answers: []string{strings.Repeat("a", 700), strings.Repeat("b", 700), "compact summary"}, failures: []error{nil, nil, nil, errors.New("answer failed")}}
	a, err := NewAgent(c, fake, NewDebugLogger(c.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s, err := a.Create(CreateSessionRequest{Chat: true})
	if err != nil {
		t.Fatal(err)
	}
	lease, _ := a.Attach(s.ID)
	for _, q := range []string{strings.Repeat("x", 550), strings.Repeat("y", 550)} {
		if err := a.Submit(s.ID, lease, MessageRequest{Content: q}); err != nil {
			t.Fatal(err)
		}
		waitState(t, a, s.ID, "completed")
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: strings.Repeat("z", 300)}); err != nil {
		t.Fatal(err)
	}
	saved := waitState(t, a, s.ID, "failed")
	if saved.SummarizedMessages != 2 || saved.Operation.BaseCount != 2 {
		t.Fatalf("early compression: %+v", saved)
	}
	if err := a.Discard(s.ID, lease); err != nil {
		t.Fatal(err)
	}
	saved, _ = a.Get(s.ID)
	if len(saved.Messages) != 2 || saved.Summary != "compact summary" {
		t.Fatalf("discard: %+v", saved)
	}
}

func TestLocalOversizedInstructionsPreserveTurn(t *testing.T) {
	c := localConfig(t)
	fake := &fakeCompleter{}
	a, err := NewAgent(c, fake, NewDebugLogger(c.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s, err := a.Create(CreateSessionRequest{Chat: true, SystemPrompt: strings.Repeat("i", 3100)})
	if err != nil {
		t.Fatal(err)
	}
	lease, _ := a.Attach(s.ID)
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "current turn"}); err != nil {
		t.Fatal(err)
	}
	saved := waitState(t, a, s.ID, "failed")
	if len(fake.requests) != 0 || saved.Messages[0].Content != "current turn" || !strings.Contains(saved.Operation.Error, "context") {
		t.Fatalf("oversized request: %+v", saved)
	}
	if err := a.Discard(s.ID, lease); err != nil {
		t.Fatal(err)
	}
}

func TestLocalCompressionCrashRecoversPendingTurn(t *testing.T) {
	c := localConfig(t)
	store := Store{Dir: c.Daemon.SessionDir}
	s := &Session{ID: randomID(), Profile: "local", Chat: true, Settings: DefaultSettings(c), Messages: []Message{{Role: "user", Content: "pending question"}}, Operation: &Operation{State: "compressing", CompressionBeforeAnswer: true}}
	if err := store.Save(s); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(); err != nil {
		t.Fatal(err)
	}
	saved, err := store.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Operation.State != "interrupted" || saved.Messages[0].Content != "pending question" {
		t.Fatalf("recovery: %+v", saved)
	}
}

func TestLocalCapabilityPromptPreservesInputs(t *testing.T) {
	original := []Message{{Role: "system", Content: "Project instructions: read README.md"}, {Role: "user", Content: "What is the latest release?"}}
	adapted := withLocalCapabilities(original)
	if original[0].Content != "Project instructions: read README.md" || adapted[1] != original[1] {
		t.Fatal("mutated original messages")
	}
	if !strings.HasPrefix(adapted[0].Content, localCapabilityPrompt) || !strings.Contains(adapted[0].Content, "read README.md") {
		t.Fatal("lost capability boundary or project instructions")
	}
	twice := withLocalCapabilities(adapted)
	if twice[0] != adapted[0] {
		t.Fatal("duplicated capability prompt")
	}
	if localInputEstimate(adapted) <= localInputEstimate(original) {
		t.Fatal("capability prompt missing from budget")
	}
}

func TestLocalRetryCorrectsToolAttemptWithoutPublishingIt(t *testing.T) {
	c := localConfig(t)
	fake := &fakeCompleter{answers: []string{`<tool_call>{"name":"read_file"}</tool_call>`, "I cannot verify the latest release offline."}}
	a, err := NewAgent(c, fake, NewDebugLogger(c.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s, err := a.Create(CreateSessionRequest{Chat: true})
	if err != nil {
		t.Fatal(err)
	}
	lease, _ := a.Attach(s.ID)
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "What is the last released Kotlin version?"}); err != nil {
		t.Fatal(err)
	}
	failed := waitState(t, a, s.ID, "failed")
	if len(failed.Messages) != 1 || !strings.Contains(failed.Operation.RejectedAnswer, "read_file") {
		t.Fatalf("rejected output handling: %+v", failed)
	}
	if err := a.Retry(s.ID, lease); err != nil {
		t.Fatal(err)
	}
	saved := waitState(t, a, s.ID, "completed")
	if len(saved.Messages) != 2 || saved.Messages[1].Content != "I cannot verify the latest release offline." {
		t.Fatalf("retry failed: %+v", saved)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	encoded, _ := json.Marshal(fake.requests[1])
	if !strings.Contains(string(encoded), "previous response was rejected") {
		t.Fatalf("retry lacks correction: %s", encoded)
	}
}
