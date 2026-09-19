package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeCompleter struct {
	mu            sync.Mutex
	answers       []string
	usages        []Usage
	finishReasons []string
	failures      []error
	requests      [][]Message
	entered       chan struct{}
	release       chan struct{}
}

func (f *fakeCompleter) Complete(ctx context.Context, m []Message, s Settings) (Completion, error) {
	if f.entered != nil {
		select {
		case f.entered <- struct{}{}:
		default:
		}
	}
	if f.release != nil {
		select {
		case <-f.release:
		case <-ctx.Done():
			return Completion{}, ctx.Err()
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.requests = append(f.requests, append([]Message(nil), m...))
	if len(f.failures) > 0 {
		failure := f.failures[0]
		f.failures = f.failures[1:]
		if failure != nil {
			return Completion{}, failure
		}
	}
	answer := "ok"
	if len(f.answers) > 0 {
		answer = f.answers[0]
		f.answers = f.answers[1:]
	}
	u := Usage{PromptTokens: 1, CompletionTokens: 2, TotalTokens: 3}
	if len(f.usages) > 0 {
		u = f.usages[0]
		f.usages = f.usages[1:]
	}
	finishReason := "stop"
	if len(f.finishReasons) > 0 {
		finishReason = f.finishReasons[0]
		f.finishReasons = f.finishReasons[1:]
	}
	call := CallMetrics{Usage: u, FinishReason: finishReason}
	return Completion{Answer: answer, Usage: u, Call: call, Metrics: Metrics{Requests: 1, Usage: u}}, nil
}

func testConfig(t *testing.T) Config {
	t.Helper()
	c := DefaultConfig()
	c.Daemon.SessionDir = filepath.Join(t.TempDir(), "sessions")
	c.Daemon.EvictAfter = Duration(10 * time.Millisecond)
	c.Daemon.LeaseTimeout = Duration(time.Second)
	c.Daemon.RequestTimeout = Duration(time.Second)
	return c
}

func TestAgentPersistsBackgroundAnswerAfterDetach(t *testing.T) {
	cfg := testConfig(t)
	entered := make(chan struct{}, 1)
	release := make(chan struct{})
	fake := &fakeCompleter{entered: entered, release: release}
	logger := NewDebugLogger(cfg.Daemon, "secret")
	a, err := NewAgent(cfg, fake, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s, err := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := a.Attach(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "hello"}); err != nil {
		t.Fatal(err)
	}
	<-entered
	if err := a.Detach(s.ID, lease); err != nil {
		t.Fatal(err)
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		saved, e := a.store.Load(s.ID)
		if e == nil && saved.Operation != nil && saved.Operation.State == "completed" {
			if got := saved.Messages[len(saved.Messages)-1].Content; got != "ok" {
				t.Fatalf("answer=%q", got)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("background answer was not persisted")
}

func TestClarificationUsesPersistedWorkflow(t *testing.T) {
	cfg := testConfig(t)
	fake := &fakeCompleter{answers: []string{"Which city?", "Final answer"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	settings := DefaultSettings(cfg)
	settings.Stop = "READY"
	s, _ := a.Create(CreateSessionRequest{Settings: &settings})
	lease, _ := a.Attach(s.ID)
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "Plan a trip"}); err != nil {
		t.Fatal(err)
	}
	waitState(t, a, s.ID, "awaiting_input")
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "Paris READY"}); err != nil {
		t.Fatal(err)
	}
	got := waitState(t, a, s.ID, "completed")
	if len(got.Messages) != 4 || got.Messages[3].Content != "Final answer" {
		t.Fatalf("messages=%+v", got.Messages)
	}
}

func TestLoggerRedactsAndRotates(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Daemon.Debug = true
	cfg.Daemon.LogPath = filepath.Join(t.TempDir(), "agentd.log")
	cfg.Daemon.LogMaxBytes = 80
	cfg.Daemon.LogBackups = 2
	l := NewDebugLogger(cfg.Daemon, "very-secret")
	l.Log("http.request", "s", "o", "Authorization: Bearer very-secret")
	l.Log("answer", "s", "o", strings.Repeat("x", 100))
	for _, p := range []string{cfg.Daemon.LogPath, cfg.Daemon.LogPath + ".1"} {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), "very-secret") {
			t.Fatalf("secret leaked in %s", p)
		}
	}
	if _, err := os.Stat(cfg.Daemon.LogPath + ".1"); err != nil {
		t.Fatal("expected rotated log")
	}
}

func TestValidateSettings(t *testing.T) {
	s := DefaultSettings(DefaultConfig())
	v := 1.0
	s.Temperature = &v
	s.Reasoning = "high"
	if ValidateSettings(s) == nil {
		t.Fatal("expected temperature/reasoning conflict")
	}
}

func TestAgentPersistsPerCallAndWholeDialogTokenCounts(t *testing.T) {
	cfg := testConfig(t)
	fake := &fakeCompleter{
		answers: []string{"short answer", "long answer"},
		usages: []Usage{
			{PromptTokens: 20, PromptCacheMissTokens: 20, CompletionTokens: 5, TotalTokens: 25},
			{PromptTokens: 75, PromptCacheHitTokens: 15, PromptCacheMissTokens: 60, CompletionTokens: 10, TotalTokens: 85},
		},
	}
	a, err := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(s.ID)
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "first"}); err != nil {
		t.Fatal(err)
	}
	waitState(t, a, s.ID, "completed")
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "second"}); err != nil {
		t.Fatal(err)
	}
	got := waitState(t, a, s.ID, "completed")
	if len(got.Calls) != 2 {
		t.Fatalf("calls=%d", len(got.Calls))
	}
	if got.Calls[1].Usage.PromptTokens != 75 {
		t.Fatalf("latest prompt tokens=%d", got.Calls[1].Usage.PromptTokens)
	}
	if got.Metrics.Requests != 2 || got.Metrics.Usage.PromptTokens != 95 || got.Metrics.Usage.CompletionTokens != 15 || got.Metrics.Usage.TotalTokens != 110 {
		t.Fatalf("session metrics=%+v", got.Metrics)
	}
	saved, err := a.store.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.Calls) != 2 || saved.Metrics.Usage.TotalTokens != 110 {
		t.Fatalf("persisted stats=%+v calls=%d", saved.Metrics, len(saved.Calls))
	}
}

func TestAgentMarksLengthLimitedAnswerAsTruncated(t *testing.T) {
	cfg := testConfig(t)
	fake := &fakeCompleter{answers: []string{"partial"}, finishReasons: []string{"length"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	s, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(s.ID)
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "write a lot"}); err != nil {
		t.Fatal(err)
	}
	got := waitState(t, a, s.ID, "truncated")
	if got.Operation.Warning == "" || got.Calls[0].FinishReason != "length" {
		t.Fatalf("operation=%+v calls=%+v", got.Operation, got.Calls)
	}
}

func TestAgentCompressesOldMessagesAndPersistsSummary(t *testing.T) {
	cfg := testConfig(t)
	cfg.Agent.RecentMessages = 2
	cfg.Agent.SummaryBatchMessages = 2
	fake := &fakeCompleter{answers: []string{"answer one", "answer two", "summary of the first turn"}}
	a, err := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	s, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(s.ID)
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "question one"}); err != nil {
		t.Fatal(err)
	}
	waitState(t, a, s.ID, "completed")
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "question two"}); err != nil {
		t.Fatal(err)
	}
	got := waitState(t, a, s.ID, "completed")
	if got.Summary != "summary of the first turn" || got.SummaryTokens != 2 || got.SummarizedMessages != 2 {
		t.Fatalf("summary=%q tokens=%d summarized=%d", got.Summary, got.SummaryTokens, got.SummarizedMessages)
	}
	if len(got.Messages) != 2 || got.Messages[0].Content != "question two" || got.Messages[1].Content != "answer two" {
		t.Fatalf("verbatim messages=%+v", got.Messages)
	}
	if len(got.Calls) != 3 || got.Calls[2].Purpose != "summary" || got.Metrics.Requests != 3 {
		t.Fatalf("calls=%+v metrics=%+v", got.Calls, got.Metrics)
	}
	if len(fake.requests) != 3 || !strings.Contains(fake.requests[2][1].Content, "question one") || !strings.Contains(fake.requests[2][1].Content, "answer one") {
		t.Fatalf("summary request=%+v", fake.requests)
	}
	saved, err := a.store.Load(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Summary != got.Summary || len(saved.Messages) != 2 {
		t.Fatalf("persisted session=%+v", saved)
	}
}

func TestAgentCanDisableHistoryCompression(t *testing.T) {
	cfg := testConfig(t)
	cfg.Agent.RecentMessages = 2
	cfg.Agent.SummaryBatchMessages = 2
	settings := DefaultSettings(cfg)
	settings.Compression = false
	settings.ContextStrategy = "full"
	fake := &fakeCompleter{answers: []string{"answer one", "answer two"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	s, _ := a.Create(CreateSessionRequest{Settings: &settings})
	lease, _ := a.Attach(s.ID)
	_ = a.Submit(s.ID, lease, MessageRequest{Content: "question one"})
	waitState(t, a, s.ID, "completed")
	_ = a.Submit(s.ID, lease, MessageRequest{Content: "question two"})
	got := waitState(t, a, s.ID, "completed")
	if got.Summary != "" || len(got.Messages) != 4 || len(got.Calls) != 2 {
		t.Fatalf("unexpected compression: summary=%q messages=%d calls=%d", got.Summary, len(got.Messages), len(got.Calls))
	}
}

func TestFailedCompressionRetainsRawMessages(t *testing.T) {
	cfg := testConfig(t)
	cfg.Agent.RecentMessages = 2
	cfg.Agent.SummaryBatchMessages = 2
	fake := &fakeCompleter{
		answers:  []string{"answer one", "answer two"},
		failures: []error{nil, nil, errors.New("summary unavailable")},
	}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	s, _ := a.Create(CreateSessionRequest{Settings: ptrSettings(DefaultSettings(cfg))})
	lease, _ := a.Attach(s.ID)
	_ = a.Submit(s.ID, lease, MessageRequest{Content: "question one"})
	waitState(t, a, s.ID, "completed")
	_ = a.Submit(s.ID, lease, MessageRequest{Content: "question two"})
	got := waitState(t, a, s.ID, "completed")
	if got.Summary != "" || len(got.Messages) != 4 || !strings.Contains(got.Operation.Warning, "summary unavailable") {
		t.Fatalf("session=%+v", got)
	}
}

func TestRequestMessagesUseSummaryBeforeVerbatimHistory(t *testing.T) {
	settings := DefaultSettings(DefaultConfig())
	messages := requestMessages("system", "Earlier fact", nil, []Message{{Role: "user", Content: "Newest question"}}, settings, false)
	if len(messages) != 2 || !strings.Contains(messages[0].Content, "Earlier fact") || messages[1].Content != "Newest question" {
		t.Fatalf("messages=%+v", messages)
	}
}

func TestRecoveryKeepsMessagesWhenCompressionWasInterrupted(t *testing.T) {
	store := Store{Dir: filepath.Join(t.TempDir(), "sessions")}
	session := &Session{
		ID:       strings.Repeat("a", 32),
		Messages: []Message{{Role: "user", Content: "keep me"}},
		Operation: &Operation{
			State: "compressing",
		},
	}
	if err := store.Save(session); err != nil {
		t.Fatal(err)
	}
	if err := store.Recover(); err != nil {
		t.Fatal(err)
	}
	got, err := store.Load(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Operation.State != "completed" || got.Messages[0].Content != "keep me" || got.Operation.Warning == "" {
		t.Fatalf("recovered session=%+v", got)
	}
}

func TestSlidingWindowDropsMessagesOutsideRecentLimit(t *testing.T) {
	cfg := testConfig(t)
	cfg.Agent.RecentMessages = 2
	settings := DefaultSettings(cfg)
	settings.ContextStrategy = "sliding"
	settings.Compression = false
	fake := &fakeCompleter{answers: []string{"a1", "a2", "a3"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	s, _ := a.Create(CreateSessionRequest{Settings: &settings})
	lease, _ := a.Attach(s.ID)
	for _, question := range []string{"q1", "q2", "q3"} {
		if err := a.Submit(s.ID, lease, MessageRequest{Content: question}); err != nil {
			t.Fatal(err)
		}
		waitState(t, a, s.ID, "completed")
	}
	got, _ := a.Get(s.ID)
	if len(got.Messages) != 2 || got.Messages[0].Content != "q3" || got.Messages[1].Content != "a3" {
		t.Fatalf("messages=%+v", got.Messages)
	}
	thirdRequest := fake.requests[2]
	if len(thirdRequest) != 2 || thirdRequest[0].Content != "a2" || thirdRequest[1].Content != "q3" {
		t.Fatalf("third request=%+v", thirdRequest)
	}
}

func TestStickyFactsStrategyIsRemoved(t *testing.T) {
	settings := DefaultSettings(DefaultConfig())
	settings.ContextStrategy = "sticky-facts"
	if ValidateSettings(settings) == nil {
		t.Fatal("sticky-facts must be rejected in favor of explicit memory")
	}
}

func TestBranchesContinueIndependentlyFromCheckpoint(t *testing.T) {
	cfg := testConfig(t)
	settings := DefaultSettings(cfg)
	settings.ContextStrategy = "branching"
	settings.Compression = false
	fake := &fakeCompleter{answers: []string{"base answer", "branch A answer"}}
	a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, "secret"))
	defer a.Close()
	root, _ := a.Create(CreateSessionRequest{Settings: &settings})
	rootLease, _ := a.Attach(root.ID)
	_ = a.Submit(root.ID, rootLease, MessageRequest{Content: "shared question"})
	waitState(t, a, root.ID, "completed")
	checkpoint, err := a.CreateCheckpoint(root.ID, rootLease, "fork")
	if err != nil {
		t.Fatal(err)
	}
	branchA, err := a.CreateBranch(root.ID, rootLease, BranchRequest{Name: "option-a", Checkpoint: checkpoint.Name})
	if err != nil {
		t.Fatal(err)
	}
	branchB, err := a.CreateBranch(root.ID, rootLease, BranchRequest{Name: "option-b", Checkpoint: checkpoint.ID})
	if err != nil {
		t.Fatal(err)
	}
	if branchA.ID == branchB.ID || branchA.RootSessionID != root.ID || branchB.RootSessionID != root.ID || len(branchA.Messages) != 2 || branchA.Metrics.Requests != 0 {
		t.Fatalf("branchA=%+v branchB=%+v", branchA, branchB)
	}
	branchLease, _ := a.Attach(branchA.ID)
	_ = a.Submit(branchA.ID, branchLease, MessageRequest{Content: "take option A"})
	waitState(t, a, branchA.ID, "completed")
	unchanged, _ := a.Get(branchB.ID)
	if len(unchanged.Messages) != 2 {
		t.Fatalf("branch B changed: %+v", unchanged.Messages)
	}
	related, err := a.RelatedBranches(branchA.ID)
	if err != nil || len(related) != 3 {
		t.Fatalf("related=%+v error=%v", related, err)
	}
}

func ptrSettings(s Settings) *Settings { return &s }
func waitState(t *testing.T, a *Agent, id, state string) *Session {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		s, e := a.Get(id)
		if e == nil && s.Operation != nil && s.Operation.State == state {
			return s
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("state %s not reached", state)
	return nil
}
