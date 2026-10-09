package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGroundedAnswerValidation(t *testing.T) {
	var chunks []EvidenceChunk
	_ = json.Unmarshal([]byte(fixtureEvidence), &chunks)
	e := PackEvidence("q", "semantic", "fp", chunks, 6000)
	valid := fixtureGroundedAnswer([]Message{{Content: e.Prompt()}})
	r, err := ValidateRAGAnswer(valid, e)
	if err != nil || len(r.Sources) != 1 || r.Sources[0].Source != chunks[0].Source || !strings.Contains(r.Render(), "chunk_id: c1") {
		t.Fatal(r, err)
	}
	for _, bad := range []string{
		`{}`, valid + `{}`, strings.Replace(valid, `"answered"`, `"other"`, 1),
		strings.Replace(valid, `"chunk_id":"c1"`, `"chunk_id":"missing"`, 1),
		strings.Replace(valid, `"text":"Reference fact αβ"`, `"text":"invented fact"`, 1),
		strings.Replace(valid, `"text":"Reference fact αβ"`, `"text":" "`, 1),
		strings.Replace(valid, `[1]`, `[2]`, 1), strings.Replace(valid, `[1]`, ``, 1),
		strings.Replace(valid, `[1]`, `[0]`, 1),
		`{"status":"unknown","answer":"Unsupported advice","quotes":[],"clarification":"Which version?"}`,
		`{"status":"unknown","answer":"","quotes":[],"clarification":""}`,
	} {
		if _, err := ValidateRAGAnswer(bad, e); err == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
	for _, raw := range []string{strings.Replace(valid, `"clarification":""`, `"clarification":"Which platform do you need?"`, 1), `{"status":"unknown","answer":"","quotes":[],"clarification":"Which platform?"}`} {
		if _, err := ValidateRAGAnswer(raw, e); err != nil {
			t.Fatal(err)
		}
	}
}

func TestMandatoryFilteringAndUnknownTask(t *testing.T) {
	for _, tc := range []struct {
		name          string
		count, budget int
	}{{"low relevance", 1, 6000}, {"empty search", 0, 6000}, {"budget exhausted", 4, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.RAG = enhancedFixture(t, tc.count)
			provider := &enhancedFake{failAnswer: true}
			a, err := NewAgent(cfg, provider, NewDebugLogger(cfg.Daemon, ""))
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			settings := DefaultSettings(cfg)
			settings.RAG = true
			settings.RAGFilter = false
			settings.RAGBudget = tc.budget
			settings.ContextStrategy = "full"
			settings.Compression = false
			s, err := a.Create(CreateSessionRequest{Settings: &settings})
			if err != nil {
				t.Fatal(err)
			}
			lease, _ := a.Attach(s.ID)
			if err = a.Submit(s.ID, lease, MessageRequest{Content: "question"}); err != nil {
				t.Fatal(err)
			}
			got := waitState(t, a, s.ID, "completed")
			if got.Operation.RAGResult == nil || got.Operation.RAGResult.Status != "unknown" || got.Task.Status != TaskStatusBlocked || len(got.Operation.MCPEvents) != 0 {
				t.Fatalf("%+v", got)
			}
			wantCalls := 1
			if tc.count == 0 {
				wantCalls = 0
			}
			if got.Operation.Metrics.Requests != wantCalls || !provider.failAnswer {
				t.Fatal("answer was called or accounting is wrong")
			}
			if err = a.ContinueTask(s.ID, lease); err == nil {
				t.Fatal("unknown advanced task")
			}
			for _, call := range got.Operation.Calls {
				if call.Purpose != "rag_score" {
					t.Fatal(call)
				}
			}
		})
	}
	// A legacy false flag cannot disable filtering at the default threshold.
	e, err := Retrieve(context.Background(), enhancedFixture(t, 4), Settings{RAGFilter: false}, "question", &enhancedFake{})
	if err != nil || !e.Filtered || len(e.Chunks) != 2 {
		t.Fatal(e, err)
	}
}

func TestGroundingInvariantsAndRetry(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		cfg := testConfig(t)
		cfg.RAG = ragFixture(t, "printf '%s' '"+fixtureEvidence+"'\n")
		fake := &fakeCompleter{}
		a, err := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, ""))
		if err != nil {
			t.Fatal(err)
		}
		defer a.Close()
		settings := DefaultSettings(cfg)
		settings.RAG = true
		settings.ContextStrategy = "full"
		settings.Compression = false
		s, err := a.Create(CreateSessionRequest{Settings: &settings})
		if err != nil {
			t.Fatal(err)
		}
		if err = a.memory.MutateInvariant(s.Profile, s.ProjectID, "create", "language", "Use Go"); err != nil {
			t.Fatal(err)
		}
		var chunks []EvidenceChunk
		_ = json.Unmarshal([]byte(fixtureEvidence), &chunks)
		payload := fixtureGroundedAnswer([]Message{{Content: PackEvidence("q", "semantic", "fp", chunks, 6000).Prompt()}})
		d := invariantDecision{Decision: "allow", ConsideredInvariants: []string{"language"}, ViolatedInvariants: []string{}, Explanation: "Compatible", Answer: payload}
		if refuse {
			d.Decision = "refuse"
			d.ViolatedInvariants = []string{"language"}
			d.Answer = ""
		}
		data, _ := json.Marshal(d)
		fake.answers = []string{string(data)}
		lease, _ := a.Attach(s.ID)
		_ = a.Submit(s.ID, lease, MessageRequest{Content: "question"})
		got := waitState(t, a, s.ID, "completed")
		if refuse {
			if got.Task.Status != TaskStatusBlocked || got.Operation.RAGResult != nil {
				t.Fatal("invariant refusal lost")
			}
		} else if got.Operation.RAGResult == nil {
			t.Fatal("missing nested grounding result")
		}
	}
}

func TestGroundedComparisonUnknownResumeAndVersion(t *testing.T) {
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.json")
	out := filepath.Join(dir, "answers.json")
	_ = os.WriteFile(suite, []byte(`{"questions":[{"id":"q","query":"question","expected_facts":"SECRET"}]}`), 0600)
	cfg := enhancedFixture(t, 1)
	p := &enhancedFake{failAnswer: true}
	if err := RunGroundedComparison(context.Background(), p, cfg, Settings{Model: FlashModel, Reasoning: "none"}, suite, out); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(out)
	var run EnhancedComparisonRun
	_ = json.Unmarshal(data, &run)
	if len(run.Answers) != 1 || !run.Answers[0].AnswerSkipped || run.Answers[0].RawAnswer != "" {
		t.Fatal("unknown not recorded")
	}
	before := len(p.calls)
	if err := RunGroundedComparison(context.Background(), p, cfg, Settings{Model: FlashModel, Reasoning: "none"}, suite, out); err != nil || len(p.calls) != before {
		t.Fatal("resume repeated calls", err)
	}
	run.Version = "old"
	_ = SaveComparison(out, run)
	if err := RunGroundedComparison(context.Background(), p, cfg, Settings{Model: FlashModel, Reasoning: "none"}, suite, out); err == nil {
		t.Fatal("accepted incompatible checkpoint")
	}
}

func TestInvalidQuoteFailsWithoutPublishingAndRetriesSavedEvidence(t *testing.T) {
	cfg := testConfig(t)
	cfg.RAG = ragFixture(t, "printf '%s' '"+fixtureEvidence+"'\n")
	fake := &fakeCompleter{answers: []string{`{"status":"answered","answer":"Invented [1].","quotes":[{"chunk_id":"c1","text":"invented quote"}],"clarification":""}`}}
	a, err := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	settings := DefaultSettings(cfg)
	settings.RAG = true
	settings.ContextStrategy = "full"
	settings.Compression = false
	s, err := a.Create(CreateSessionRequest{Chat: true, Settings: &settings})
	if err != nil {
		t.Fatal(err)
	}
	lease, _ := a.Attach(s.ID)
	_ = a.Submit(s.ID, lease, MessageRequest{Content: "question"})
	failed := waitState(t, a, s.ID, "failed")
	if failed.Operation.RAGResult != nil || !strings.Contains(failed.Operation.RAGRawAnswer, "invented quote") || failed.Operation.Metrics.Requests != 2 {
		t.Fatal("missing failure provenance")
	}
	for _, m := range failed.Messages {
		if m.Role == "assistant" {
			t.Fatal("published unvalidated answer")
		}
	}
	_ = os.WriteFile(cfg.RAG.Executable, []byte("#!/bin/sh\nexit 9\n"), 0700)
	if err = a.Retry(s.ID, lease); err != nil {
		t.Fatal(err)
	}
	got := waitState(t, a, s.ID, "completed")
	if got.Operation.RAGResult == nil || got.Operation.Metrics.Requests != 3 {
		t.Fatal("retry lost evidence or repeated scoring")
	}
}

func TestWeakContextSkipsNativeTools(t *testing.T) {
	cfg := testConfig(t)
	cfg.RAG = ragFixture(t, "printf '%s' '"+fixtureEvidence+"'\n")
	p := &scriptedNative{}
	a, err := NewAgent(cfg, p, NewDebugLogger(cfg.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err = a.MCP.Add("fixture", "fixture", []string{fixtureBinary(t)}); err != nil {
		t.Fatal(err)
	}
	settings := DefaultSettings(cfg)
	settings.RAG = true
	settings.RAGBudget = 1
	settings.ContextStrategy = "full"
	settings.Compression = false
	s, err := a.Create(CreateSessionRequest{Chat: true, Settings: &settings})
	if err != nil {
		t.Fatal(err)
	}
	lease, _ := a.Attach(s.ID)
	_ = a.Submit(s.ID, lease, MessageRequest{Content: "question"})
	got := waitState(t, a, s.ID, "completed")
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.rounds != 0 || got.Operation.RAGResult.Status != "unknown" {
		t.Fatal("weak evidence reached native tools")
	}
}
