package agent

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestScorerIsolationAndBudget(t *testing.T) {
	cfg := localConfig(t)
	bodies := []map[string]any{}
	p, _ := mockLocalClient(t, cfg, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show" {
			w.Write([]byte(`{}`))
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		bodies = append(bodies, body)
		w.Write([]byte(`{"done":true,"done_reason":"stop","message":{"content":"ok"}}`))
	})
	zero := 0.0
	sc := ScorerConfig{Temperature: &zero, ContextWindowTokens: 2048, OutputTokens: 96, BatchSize: 2, PromptVersion: "compact-v1"}
	for _, purpose := range []string{"rag_score", "rag_rewrite", "summary", "answer"} {
		req := localRequest(cfg)
		req.Purpose = purpose
		req.Scorer = &sc
		if _, err := p.Complete(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	for i, b := range bodies {
		opts := b["options"].(map[string]any)
		if i == 0 {
			if opts["num_ctx"] != float64(2048) || opts["num_predict"] != float64(96) || opts["temperature"] != float64(0) {
				t.Fatal(opts)
			}
		} else if opts["num_ctx"] != float64(4096) || opts["temperature"] != float64(1) {
			t.Fatal("scorer leaked", opts)
		}
	}
	req := localRequest(cfg)
	req.Purpose = "rag_score"
	req.Scorer = &sc
	req.Messages = []Message{{Role: "user", Content: strings.Repeat("x", 2048)}}
	if _, err := p.Complete(context.Background(), req); err == nil {
		t.Fatal("accepted oversized input")
	}
	if len(bodies) != 4 {
		t.Fatal("oversized request reached server")
	}
}
func TestScorerCompactBatchesAndIdentity(t *testing.T) {
	cfg := enhancedFixture(t, 5)
	cfg.Scorer = ScorerConfig{BatchSize: 4, PromptVersion: "compact-v1"}
	fake := &enhancedFake{}
	s := Settings{Provider: "ollama", Model: LocalModel, Reasoning: "none", RAG: true}
	prep, err := PrepareRAG(context.Background(), fake, cfg, s, "question", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 2 || len(prep.Scores) != 5 {
		t.Fatal("batch boundary", len(fake.calls), prep.Scores)
	}
	for _, r := range fake.calls {
		if strings.Contains(r.Messages[1].Content, "line_start") || strings.Contains(r.Messages[1].Content, "strategy") {
			t.Fatal("bookkeeping in compact prompt")
		}
		if !strings.Contains(r.Messages[1].Content, "Reference fact") {
			t.Fatal("lost text")
		}
	}
	cfg.Scorer.OutputTokens = 160
	if _, err := PrepareRAG(context.Background(), fake, cfg, s, "question", prep, nil); err == nil {
		t.Fatal("reused different scorer")
	}
}
func TestScoreBenchReplayResumeAndFailures(t *testing.T) {
	cfg := localConfig(t)
	cfg.RAG = enhancedFixture(t, 3)
	s := DefaultSettings(cfg)
	fake := &enhancedFake{}
	prep, err := PrepareRAG(context.Background(), fake, cfg.RAG, s, "secret-query", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "frozen.json")
	output := filepath.Join(dir, "scores.json")
	source := RepeatedRAGRun{Trials: []RAGTrial{{Pass: 1, QuestionID: "q1", Preparation: prep}}}
	data, _ := json.Marshal(source)
	os.WriteFile(input, data, 0600)
	cfg.RAG.Executable = "/does/not/exist" // replay must never retrieve
	fake = &enhancedFake{}
	if err = RunScoreBench(context.Background(), fake, cfg, s, input, output, "", 2); err != nil {
		t.Fatal(err)
	}
	count := len(fake.calls)
	if count != 6 {
		t.Fatal("warmup and two passes", count)
	}
	if err = RunScoreBench(context.Background(), fake, cfg, s, input, output, "", 2); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != count {
		t.Fatal("replayed completed trials")
	}
	cfg.RAG.Scorer.BatchSize = 4
	if err = RunScoreBench(context.Background(), fake, cfg, s, input, output, "", 2); err == nil {
		t.Fatal("accepted changed config")
	}
	bad := &enhancedFake{scoreOutput: `{"scores":[]}`}
	if err = RunScoreBench(context.Background(), bad, cfg, s, input, filepath.Join(dir, "bad.json"), "", 1); err == nil {
		t.Fatal("accepted missing scores")
	}
}
func TestScorerConfigValidation(t *testing.T) {
	for _, s := range []ScorerConfig{{BatchSize: -1}, {BatchSize: 101}, {OutputTokens: -1}, {ContextWindowTokens: 32}, {PromptVersion: "oops"}} {
		if s.Validate() == nil {
			t.Fatal(s)
		}
	}
	if !(ScorerConfig{}).IsDefault() {
		t.Fatal("changed defaults")
	}
}

func TestAgentScorerSurvivesCompletionAdapter(t *testing.T) {
	cfg := localConfig(t)
	cfg.RAG = enhancedFixture(t, 5)
	zero := 0.0
	cfg.RAG.Scorer = ScorerConfig{Temperature: &zero, BatchSize: 4, ContextWindowTokens: 8192, OutputTokens: 160, PromptVersion: "compact-v1"}
	fake := &enhancedFake{}
	a, err := NewAgent(cfg, fake, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if _, err = PrepareRAG(context.Background(), completionFunc(a.complete), cfg.RAG, DefaultSettings(cfg), "question", nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 2 {
		t.Fatal("batch count")
	}
	for _, r := range fake.calls {
		if r.Scorer == nil || r.Scorer.ContextWindowTokens != 8192 || *r.Scorer.Temperature != 0 {
			t.Fatalf("scorer lost through adapter: %+v", r)
		}
	}
}

func TestConfiguredRerankDefault(t *testing.T) {
	cfg := localConfig(t)
	if DefaultSettings(cfg).RAGRerank {
		t.Fatal("changed legacy rerank default")
	}
	cfg.RAG.Rerank = true
	if !DefaultSettings(cfg).RAGRerank {
		t.Fatal("ignored profile rerank")
	}
}

func TestInheritedScorerContextIdentity(t *testing.T) {
	cfg := localConfig(t)
	cfg.RAG = enhancedFixture(t, 2)
	cfg.RAG.Scorer.PromptVersion = "compact-v1"
	fake := &enhancedFake{}
	resolved := cfg.scoringConfig()
	prep, err := PrepareRAG(context.Background(), fake, resolved.RAG, DefaultSettings(cfg), "q", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Agent.ContextWindowTokens *= 2
	resolved = cfg.scoringConfig()
	if _, err = PrepareRAG(context.Background(), fake, resolved.RAG, DefaultSettings(cfg), "q", prep, nil); err == nil {
		t.Fatal("reused scores after inherited context changed")
	}
}

func TestOrderedScoringMappingAndInvalidArrays(t *testing.T) {
	cfg := enhancedFixture(t, 5)
	cfg.Scorer = ScorerConfig{BatchSize: 4, PromptVersion: "compact-v2"}
	settings := Settings{Provider: "ollama", Model: LocalModel, Reasoning: "none"}
	p, err := PrepareRAG(context.Background(), &enhancedFake{}, cfg, settings, "q", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Calls) != 2 || len(p.Scores) != 5 || p.Scores["c3"] != 3 || p.Scores["c4"] != 0 {
		t.Fatalf("ordered mapping: %+v", p.Scores)
	}
	for _, response := range []string{`{"scores":[0]}`, `{"scores":[0,1,2,3,0]}`, `{"scores":[0,1,null,3]}`, `{"scores":[0,1,4,3]}`, `{"scores":[0,1,"2",3]}`} {
		fake := &enhancedFake{scoreOutput: response}
		if _, err := PrepareRAG(context.Background(), fake, cfg, settings, "q", nil, nil); err == nil {
			t.Fatal("accepted invalid ordered scores", response)
		}
	}
}
func TestOrderedScoringSchema(t *testing.T) {
	cfg := localConfig(t)
	p, _ := mockLocalClient(t, cfg, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show" {
			w.Write([]byte(`{}`))
			return
		}
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		schema := body["format"].(map[string]any)["properties"].(map[string]any)["scores"].(map[string]any)
		if schema["minItems"] != float64(2) || schema["maxItems"] != float64(2) {
			t.Error("missing exact array length", schema)
		}
		messages := body["messages"].([]any)
		if strings.Contains(messages[0].(map[string]any)["content"].(string), localCapabilityPrompt) {
			t.Error("chat preamble in scoring request")
		}
		w.Write([]byte(`{"done":true,"done_reason":"stop","message":{"content":"{\"scores\":[3,0]}"}}`))
	})
	req := localRequest(cfg)
	req.Purpose = "rag_score"
	req.Scorer = &ScorerConfig{PromptVersion: "compact-v2"}
	req.Messages = []Message{{Role: "system", Content: compactScoresInstructions}, {Role: "user", Content: `{"question":"q","candidates":[{"id":"c1"},{"id":"c2"}]}`}}
	if _, err := p.Complete(context.Background(), req); err != nil {
		t.Fatal(err)
	}
}

// Warmup succeeds, then the measured request waits for cancellation.
type deadlineScorer struct {
	fake  enhancedFake
	calls int
}

func (p *deadlineScorer) Complete(ctx context.Context, r CompletionRequest) (CompletionResponse, error) {
	p.calls++
	if p.calls == 1 {
		return p.fake.Complete(ctx, r)
	}
	<-ctx.Done()
	return CompletionResponse{}, ctx.Err()
}
func TestScoreBenchDeadlineIsTerminalAndBoundToBaseline(t *testing.T) {
	cfg := localConfig(t)
	cfg.RAG = enhancedFixture(t, 1)
	s := DefaultSettings(cfg)
	prep, err := PrepareRAG(context.Background(), &enhancedFake{}, cfg.RAG, s, "query", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "input.json")
	baseline := filepath.Join(dir, "baseline.json")
	output := filepath.Join(dir, "bounded.json")
	source := RepeatedRAGRun{Trials: []RAGTrial{{Pass: 1, QuestionID: "q1", Preparation: prep}}}
	data, _ := json.Marshal(source)
	os.WriteFile(input, data, 0600)
	if err := RunScoreBench(context.Background(), &enhancedFake{}, cfg, s, input, baseline, "", 1); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(baseline)
	var b ScoreBenchRun
	json.Unmarshal(data, &b)
	b.Trials[0].Duration = 20 * time.Millisecond
	SaveComparison(baseline, b)
	opts := ScoreBenchOptions{Baseline: baseline, TimeoutFactor: 0.6, WarmupTimeout: time.Second}
	slow := &deadlineScorer{}
	if err := RunScoreBench(context.Background(), slow, cfg, s, input, output, "", 1, opts); err == nil {
		t.Fatal("accepted timeout")
	}
	data, _ = os.ReadFile(output)
	var got ScoreBenchRun
	json.Unmarshal(data, &got)
	trial := got.Trials[0]
	if !trial.Completed || !strings.Contains(trial.Error, "deadline exceeded") || trial.Budget != 12*time.Millisecond || trial.Evidence != nil {
		t.Fatalf("bad deadline outcome: %+v", trial)
	}
	calls := slow.calls
	if err := RunScoreBench(context.Background(), slow, cfg, s, input, output, "", 1, opts); err == nil || slow.calls != calls {
		t.Fatal("retried timed-out trial")
	}
	opts.TimeoutFactor = 0.5
	if err := RunScoreBench(context.Background(), slow, cfg, s, input, output, "", 1, opts); err == nil || slow.calls != calls {
		t.Fatal("accepted changed deadline")
	}
}
