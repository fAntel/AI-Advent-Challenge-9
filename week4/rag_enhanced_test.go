package agent

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type enhancedFake struct {
	mu          sync.Mutex
	calls       []CompletionRequest
	failAnswer  bool
	scoreOutput string
}

func (f *enhancedFake) Complete(ctx context.Context, r CompletionRequest) (CompletionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, r)
	answer := "answer"
	switch r.Messages[0].Content {
	case rewriteInstructions:
		answer = `{"query":"rewritten Kotlin query"}`
	case scoreInstructions:
		if f.scoreOutput != "" {
			answer = f.scoreOutput
			break
		}
		var input struct {
			Candidates []EvidenceChunk `json:"candidates"`
		}
		if err := json.Unmarshal([]byte(r.Messages[1].Content), &input); err != nil {
			return CompletionResponse{}, err
		}
		scores := []map[string]any{}
		for i, c := range input.Candidates {
			scores = append(scores, map[string]any{"id": c.ID, "score": i % 4})
		}
		data, _ := json.Marshal(map[string]any{"scores": scores})
		answer = string(data)
	default:
		if f.failAnswer {
			f.failAnswer = false
			return CompletionResponse{}, errors.New("answer failed")
		}
	}
	call := CallMetrics{StartedAt: time.Now(), Duration: time.Millisecond, Usage: Usage{TotalTokens: 10}, CostUSD: 0.01}
	return CompletionResponse{Answer: answer, Call: call, Metrics: Metrics{Requests: 1, Duration: call.Duration, Usage: call.Usage, CostUSD: call.CostUSD}}, nil
}
func enhancedFixture(t *testing.T, n int) RAGConfig {
	var base []EvidenceChunk
	_ = json.Unmarshal([]byte(fixtureEvidence), &base)
	chunks := []EvidenceChunk{}
	for i := 0; i < n; i++ {
		c := base[0]
		c.ID = fmt.Sprintf("c%d", i)
		chunks = append(chunks, c)
	}
	data, _ := json.Marshal(chunks)
	return ragFixture(t, "printf '%s' '"+string(data)+"'\n")
}
func TestEnhancedSelectionMatrix(t *testing.T) {
	cfg := enhancedFixture(t, 6)
	ctx := context.Background()
	for _, mode := range EnhancedModes() {
		t.Run(mode.Name, func(t *testing.T) {
			f := &enhancedFake{}
			s := Settings{RAGRewrite: mode.Rewrite, RAGRerank: mode.Rerank, RAGFilter: mode.Filter, RAGLimit: 3, RAGCandidates: 6}
			e, err := Retrieve(ctx, cfg, s, "original question", f)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"c0", "c1", "c2"}
			if mode.Rerank {
				want = []string{"c3", "c2", "c1"}
			}
			if mode.Filter {
				want = []string{"c2", "c3"}
				if mode.Rerank {
					want = []string{"c3", "c2"}
				}
			}
			if len(e.Chunks) != len(want) {
				t.Fatalf("chunks: %+v", e.Chunks)
			}
			for i, c := range e.Chunks {
				if c.ID != want[i] {
					t.Fatalf("%d: %s != %s", i, c.ID, want[i])
				}
			}
			if e.Query != "original question" || e.Preparation.Query != e.Query {
				t.Fatal("original question lost")
			}
			if mode.Rewrite && e.Preparation.SearchQuery != "rewritten Kotlin query" {
				t.Fatal("missing rewrite")
			}
			expected := 0
			if mode.Rewrite {
				expected++
			}
			if mode.Rerank || mode.Filter {
				expected += 2
			}
			if len(f.calls) != expected {
				t.Fatalf("%d calls != %d", len(f.calls), expected)
			}
			for _, r := range f.calls {
				if r.Settings.Model != FlashModel || r.Settings.Reasoning != "none" || r.Settings.Temperature != nil || r.Settings.RAG {
					t.Fatal("auxiliary settings")
				}
				if r.Messages[0].Content == scoreInstructions && !strings.Contains(r.Messages[1].Content, "original question") {
					t.Fatal("scored rewrite")
				}
			}
		})
	}
}
func TestEnhancedValidationAndCutoffs(t *testing.T) {
	ctx := context.Background()
	cfg := enhancedFixture(t, 4)
	for _, bad := range []string{`not json`, `{}`, `{"scores":[{"id":"c0","score":4}]}`, `{"scores":[{"id":"unknown","score":2}]}`, `{"scores":[{"id":"c0"}]}`, `{"scores":[{"id":"c0","score":1},{"id":"c0","score":2}]}`} {
		if _, err := Retrieve(ctx, cfg, Settings{RAGFilter: true}, "q", &enhancedFake{scoreOutput: bad}); err == nil {
			t.Fatal("accepted", bad)
		}
	}
	for threshold := 0; threshold <= 3; threshold++ {
		e, err := Retrieve(ctx, cfg, Settings{RAGFilter: true, RAGThreshold: &threshold}, "q", &enhancedFake{})
		if err != nil || len(e.Chunks) != 4-threshold {
			t.Fatal(threshold, e, err)
		}
	}
	tiny := Settings{RAGBudget: 1, RAGFilter: true}
	e, err := Retrieve(ctx, cfg, tiny, "q", &enhancedFake{})
	if err != nil || len(e.Chunks) != 0 || !strings.Contains(e.Prompt(), "insufficient") {
		t.Fatal(e, err)
	}
	empty := enhancedFixture(t, 0)
	f := &enhancedFake{}
	e, err = Retrieve(ctx, empty, Settings{RAGFilter: true}, "q", f)
	if err != nil || len(e.Chunks) != 0 || len(f.calls) != 0 {
		t.Fatal(e, err)
	}
	for _, s := range []Settings{{RAGCandidates: 2, RAGLimit: 3}, {RAGCandidates: 101}} {
		if validateRAG(s) == nil {
			t.Fatal("accepted invalid settings")
		}
	}
}
func TestEnhancedComparisonResume(t *testing.T) {
	cfg := enhancedFixture(t, 6)
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.json")
	output := filepath.Join(dir, "answers.json")
	_ = os.WriteFile(suite, []byte(`{"questions":[{"id":"q","query":"question","expected_facts":"SECRET"}]}`), 0600)
	s := Settings{Model: FlashModel, Reasoning: "none"}
	first := &enhancedFake{failAnswer: true}
	if RunEnhancedComparison(context.Background(), first, cfg, s, suite, output) == nil {
		t.Fatal("expected failure")
	}
	next := &enhancedFake{}
	if err := RunEnhancedComparison(context.Background(), next, cfg, s, suite, output); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(output)
	var run EnhancedComparisonRun
	_ = json.Unmarshal(data, &run)
	if len(run.Answers) != 8 || len(run.Preparations) != 2 {
		t.Fatal("incomplete matrix")
	}
	rewrite, score := 0, 0
	for _, r := range next.calls {
		switch r.Messages[0].Content {
		case rewriteInstructions:
			rewrite++
		case scoreInstructions:
			score++
		}
		data, _ := json.Marshal(r.Messages)
		if strings.Contains(string(data), "SECRET") {
			t.Fatal("label leakage")
		}
	}
	if rewrite != 1 || score != 4 {
		t.Fatalf("rewrite=%d scoring=%d", rewrite, score)
	}
	before := len(next.calls)
	if err := RunEnhancedComparison(context.Background(), next, cfg, s, suite, output); err != nil || len(next.calls) != before {
		t.Fatal("regenerated", err)
	}
	s.RAGCandidates = 21
	if RunEnhancedComparison(context.Background(), next, cfg, s, suite, output) == nil {
		t.Fatal("accepted settings drift")
	}
}
func TestEnhancedAgentRetryAccountingAndPlain(t *testing.T) {
	cfg := testConfig(t)
	cfg.RAG = enhancedFixture(t, 4)
	f := &enhancedFake{failAnswer: true}
	a, err := NewAgent(cfg, f, NewDebugLogger(cfg.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	settings := DefaultSettings(cfg)
	settings.ContextStrategy = "full"
	settings.Compression = false
	settings.RAG = true
	settings.RAGRewrite = true
	settings.RAGRerank = true
	settings.RAGFilter = true
	s, err := a.Create(CreateSessionRequest{Chat: true, Settings: &settings})
	if err != nil {
		t.Fatal(err)
	}
	lease, _ := a.Attach(s.ID)
	if err = a.Submit(s.ID, lease, MessageRequest{Content: "question"}); err != nil {
		t.Fatal(err)
	}
	failed := waitState(t, a, s.ID, "failed")
	if failed.Operation.Evidence == nil || failed.Operation.Metrics.Requests != 2 {
		t.Fatal("preparation not saved/accounted")
	}
	_ = os.WriteFile(cfg.RAG.Executable, []byte("#!/bin/sh\nexit 9\n"), 0700)
	if err = a.Retry(s.ID, lease); err != nil {
		t.Fatal(err)
	}
	result := waitState(t, a, s.ID, "completed")
	if result.Operation.Metrics.Requests != 3 {
		t.Fatalf("calls duplicated: %+v", result.Operation.Metrics)
	}
	if _, err = a.CreateCheckpoint(s.ID, lease, "enhanced"); err != nil {
		t.Fatal(err)
	}
	branch, err := a.CreateBranch(s.ID, lease, BranchRequest{Name: "enhanced", Checkpoint: "enhanced"})
	if err != nil || !branch.Settings.RAGFilter || !branch.Settings.RAGRewrite || !branch.Settings.RAGRerank {
		t.Fatal("branch settings", err)
	}
	settings.RAG = false
	plain, err := a.Create(CreateSessionRequest{Chat: true, Settings: &settings})
	if err != nil {
		t.Fatal(err)
	}
	token, _ := a.Attach(plain.ID)
	_ = a.Submit(plain.ID, token, MessageRequest{Content: "plain"})
	p := waitState(t, a, plain.ID, "completed")
	if p.Operation.Evidence != nil || p.Operation.RAGPreparation != nil {
		t.Fatal("plain performed retrieval")
	}
}

func TestRAGFlagsBoundsAndExplicitZeroThreshold(t *testing.T) {
	for _, args := range [][]string{{"--rag-candidates", "0"}, {"--rag-limit", "0"}, {"--rag-budget", "0"}, {"--rag-threshold", "4"}} {
		s := Settings{}
		fs := flag.NewFlagSet("test", flag.ContinueOnError)
		RAGFlags(fs, &s, Settings{})
		if err := fs.Parse(args); err == nil {
			t.Fatal("accepted", args)
		}
	}
	s := Settings{}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	RAGFlags(fs, &s, Settings{})
	if err := fs.Parse([]string{"--rag-threshold", "0"}); err != nil || *normalizeRAG(s).RAGThreshold != 0 {
		t.Fatal("lost zero threshold", err)
	}
}
func TestScoringCheckpointResumeAndStableTies(t *testing.T) {
	cfg := enhancedFixture(t, 7)
	s := Settings{RAGRerank: true, RAGLimit: 7}
	f := &enhancedFake{}
	var saved *RAGPreparation
	_, err := PrepareRAG(context.Background(), f, cfg, s, "q", nil, func(p *RAGPreparation) error {
		saved = clonePreparation(p)
		if len(p.Scores) == 5 {
			return errors.New("checkpoint interruption")
		}
		return nil
	})
	if err == nil || len(saved.Scores) != 5 {
		t.Fatal("did not checkpoint first batch")
	}
	next := &enhancedFake{}
	p, err := PrepareRAG(context.Background(), next, cfg, s, "q", saved, nil)
	if err != nil || len(next.calls) != 1 {
		t.Fatal("repeated completed scoring", err)
	}
	// Equal scores retain original retrieval order, including across batches.
	for id := range p.Scores {
		p.Scores[id] = 2
	}
	e, err := SelectRAG(p, s)
	if err != nil {
		t.Fatal(err)
	}
	for i, c := range e.Chunks {
		if c.ID != fmt.Sprintf("c%d", i) {
			t.Fatal("unstable tie")
		}
	}
}
func TestEnhancedCapabilityRejectsOldDaemon(t *testing.T) {
	// The transport uses a Unix-domain socket, just like a running daemon.
	dir, err := os.MkdirTemp("/tmp", "rag-cap-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })
	socket := filepath.Join(dir, "daemon.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, `{"rag":true}`) })}
	go server.Serve(listener)
	defer server.Close()
	client := NewAPIClient(socket, time.Second)
	if err := client.RequireRAG(); err != nil {
		t.Fatal(err)
	}
	if err := client.RequireRAG(true); err == nil {
		t.Fatal("old daemon silently accepted enhancements")
	}
}
