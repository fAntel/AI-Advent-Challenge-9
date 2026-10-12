package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLocalRAGBatchesAndCheckpointIdentity(t *testing.T) {
	cfg := enhancedFixture(t, 5)
	s := Settings{Provider: "ollama", Model: LocalModel, Reasoning: "none", RAG: true}
	fake := &enhancedFake{}
	prep, err := PrepareRAG(context.Background(), fake, cfg, s, "question", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 3 {
		t.Fatalf("want 3 batches, got %d", len(fake.calls))
	}
	for _, r := range fake.calls {
		if r.Settings.Provider != "ollama" || r.Settings.Model != LocalModel || r.Purpose != "rag_score" {
			t.Fatalf("wrong routing: %+v", r)
		}
		var input struct {
			Candidates []EvidenceChunk `json:"candidates"`
		}
		json.Unmarshal([]byte(r.Messages[1].Content), &input)
		if len(input.Candidates) > 2 {
			t.Fatal("oversized batch")
		}
	}
	if _, err = PrepareRAG(context.Background(), fake, cfg, s, "question", prep, nil); err != nil {
		t.Fatal(err)
	}
	if len(fake.calls) != 3 {
		t.Fatal("repeated completed scores")
	}
	s.Model = "different"
	if _, err = PrepareRAG(context.Background(), fake, cfg, s, "question", prep, nil); err == nil {
		t.Fatal("reused scores from different model")
	}
}

func TestRepeatedRAGRetainsFailuresAndRunsIndependently(t *testing.T) {
	cfg := localConfig(t)
	cfg.RAG = enhancedFixture(t, 4)
	suite := filepath.Join(t.TempDir(), "suite.json")
	output := filepath.Join(t.TempDir(), "answers.json")
	os.WriteFile(suite, []byte(`{"questions":[{"id":"q1","query":"Kotlin?"}]}`), 0600)
	fake := &enhancedFake{scoreOutput: `{"scores":[]}`}
	if RunRepeatedRAG(context.Background(), fake, cfg, DefaultSettings(cfg), suite, output, 3) == nil {
		t.Fatal("missing failure status")
	}
	if len(fake.calls) != 3 {
		t.Fatal("trials did not independently attempt scoring")
	}
	var run RepeatedRAGRun
	data, _ := os.ReadFile(output)
	json.Unmarshal(data, &run)
	if len(run.Trials) != 3 {
		t.Fatal("missing trials")
	}
	for _, trial := range run.Trials {
		if !trial.Completed || trial.Error == "" || trial.Preparation == nil {
			t.Fatal("failure lost")
		}
	}
	RunRepeatedRAG(context.Background(), fake, cfg, DefaultSettings(cfg), suite, output, 3)
	if len(fake.calls) != 3 {
		t.Fatal("terminal failures retried")
	}
	cfg.Agent.ContextWindowTokens *= 2
	if RunRepeatedRAG(context.Background(), fake, cfg, DefaultSettings(cfg), suite, output, 3) == nil {
		t.Fatal("accepted changed context")
	}
}

func TestLocalRAGGroundedFlow(t *testing.T) {
	cfg := localConfig(t)
	cfg.Agent.ContextWindowTokens = 16384
	cfg.RAG = enhancedFixture(t, 3)
	scoreCalls, answerCalls := 0, 0
	p, _ := mockLocalClient(t, cfg, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("unexpected cloud credential")
		}
		if r.URL.Path == "/api/show" {
			w.Write([]byte(`{}`))
			return
		}
		var body struct {
			Messages []Message `json:"messages"`
			Options  struct {
				Output int `json:"num_predict"`
			} `json:"options"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		content := ""
		if strings.Contains(body.Messages[0].Content, scoreInstructions) {
			scoreCalls++
			if body.Options.Output != 256 {
				t.Errorf("scoring output budget %d", body.Options.Output)
			}
			var input struct {
				Candidates []EvidenceChunk `json:"candidates"`
			}
			json.Unmarshal([]byte(body.Messages[1].Content), &input)
			scores := []map[string]any{}
			for _, c := range input.Candidates {
				scores = append(scores, map[string]any{"id": c.ID, "score": 3})
			}
			raw, _ := json.Marshal(map[string]any{"scores": scores})
			content = string(raw)
		} else {
			answerCalls++
			content = fixtureGroundedAnswer(body.Messages)
		}
		json.NewEncoder(w).Encode(map[string]any{"done": true, "done_reason": "stop", "message": map[string]string{"content": content}})
	})
	s := DefaultSettings(cfg)
	s.RAG = true
	evidence, err := Retrieve(context.Background(), cfg.RAG, s, "Kotlin?", p)
	if err != nil {
		t.Fatal(err)
	}
	record, err := completeComparisonAnswer(context.Background(), p, ComparisonQuestion{ID: "q", Query: "Kotlin?"}, "grounded", s, evidence, ComparisonMessages("Kotlin?", evidence))
	if err != nil {
		t.Fatal(err)
	}
	if scoreCalls != 2 || answerCalls != 1 || record.Result == nil || record.Result.Status != "answered" {
		t.Fatalf("calls %d/%d result %+v", scoreCalls, answerCalls, record)
	}
}

func TestLocalRAGTruncationRetainsRawResponse(t *testing.T) {
	cfg := localConfig(t)
	p, _ := mockLocalClient(t, cfg, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show" {
			w.Write([]byte(`{}`))
			return
		}
		w.Write([]byte(`{"done":true,"done_reason":"length","message":{"content":"partial JSON"}}`))
	})
	request := localRequest(cfg)
	request.Purpose = "rag_score"
	response, err := p.Complete(context.Background(), request)
	if err == nil || response.Answer != "partial JSON" {
		t.Fatalf("lost truncated response: %+v %v", response, err)
	}
}

func TestLocalEmptyEvidenceSkipsGenerationAndContextFitting(t *testing.T) {
	cfg := localConfig(t)
	cfg.RAG = enhancedFixture(t, 1)
	fake := &enhancedFake{scoreOutput: `{"scores":[{"id":"c1","score":0}]}`}
	a, err := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	settings := DefaultSettings(cfg)
	settings.RAG = true
	session, err := a.Create(CreateSessionRequest{Chat: true, Profile: "local", Settings: &settings, SystemPrompt: strings.Repeat("Long instructions. ", 1000)})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := a.Attach(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Submit(session.ID, lease, MessageRequest{Content: "Unsupported?"}); err != nil {
		t.Fatal(err)
	}
	saved := waitState(t, a, session.ID, "completed")
	if saved.Operation.RAGResult == nil || saved.Operation.RAGResult.Status != "unknown" {
		t.Fatal("missing deterministic unknown")
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.calls) != 1 || fake.calls[0].Purpose != "rag_score" {
		t.Fatal("empty evidence caused generation")
	}
}

type localRAGPartialProvider struct{ scorer enhancedFake }

func (p *localRAGPartialProvider) Complete(ctx context.Context, r CompletionRequest) (CompletionResponse, error) {
	if r.Purpose == "rag_score" {
		return p.scorer.Complete(ctx, r)
	}
	return CompletionResponse{Answer: "partial grounded JSON", Metrics: Metrics{Requests: 1, Usage: Usage{TotalTokens: 7}}, Call: CallMetrics{FinishReason: "length", Usage: Usage{TotalTokens: 7}}}, fmt.Errorf("local RAG response reached the output token limit")
}
func TestLocalRAGFailedCompletionRetainsUsage(t *testing.T) {
	cfg := localConfig(t)
	cfg.Agent.ContextWindowTokens = 16384
	cfg.RAG = enhancedFixture(t, 1)
	provider := &localRAGPartialProvider{scorer: enhancedFake{scoreOutput: `{"scores":[{"id":"c1","score":3}]}`}}
	a, err := NewAgent(cfg, provider, NewDebugLogger(cfg.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	settings := DefaultSettings(cfg)
	settings.RAG = true
	session, err := a.Create(CreateSessionRequest{Chat: true, Profile: "local", Settings: &settings})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := a.Attach(session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Submit(session.ID, lease, MessageRequest{Content: "Kotlin?"}); err != nil {
		t.Fatal(err)
	}
	saved := waitState(t, a, session.ID, "failed")
	if saved.Operation.RAGRawAnswer != "partial grounded JSON" || len(saved.Calls) != 2 || saved.Calls[1].Usage.TotalTokens != 7 {
		t.Fatalf("lost failed completion: %+v", saved.Operation)
	}
}
