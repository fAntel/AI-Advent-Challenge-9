package agent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func ragFixture(t *testing.T, body string) RAGConfig {
	t.Helper()
	dir := t.TempDir()
	exe := filepath.Join(dir, "indexer")
	db := filepath.Join(dir, "index.sqlite")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"+body), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(db, []byte("index"), 0600); err != nil {
		t.Fatal(err)
	}
	return RAGConfig{exe, db}
}

const fixtureEvidence = `[{"id":"c1","document_id":"doc","strategy":"semantic","text":"Reference fact αβ","title":"Guide","source":"https://example.org/guide","sections":["Section"],"line_start":3,"line_end":5,"availability":"unknown","score":0.9}]`

func TestRAGSubprocessAndValidation(t *testing.T) {
	cfg := ragFixture(t, "printf '%s' '"+fixtureEvidence+"'\n")
	t.Chdir(t.TempDir())
	e, err := Retrieve(context.Background(), cfg, Settings{}, "--question ' ; $(no-shell)")
	if err != nil {
		t.Fatal(err)
	}
	if len(e.Chunks) != 1 || e.Chunks[0].Sections[0] != "Section" || !strings.Contains(e.Prompt(), "lines 3-5") {
		t.Fatalf("%+v", e)
	}
	for _, body := range []string{"exit 2", "echo not-json", "echo null", "echo '[{}]'"} {
		bad := ragFixture(t, body)
		if _, err := Retrieve(context.Background(), bad, Settings{}, "q"); err == nil {
			t.Fatal(body)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Retrieve(ctx, cfg, Settings{}, "q"); err == nil {
		t.Fatal("cancellation ignored")
	}
	if _, _, err := (RAGConfig{Executable: "relative"}).Paths(); err == nil {
		t.Fatal("relative override")
	}
	exe, db, err := (RAGConfig{}).Paths()
	if err != nil || !filepath.IsAbs(exe) || filepath.Dir(filepath.Dir(db)) != filepath.Dir(filepath.Dir(filepath.Dir(exe))) {
		t.Fatalf("%s %s %v", exe, db, err)
	}
}
func TestRAGWholeChunkBudget(t *testing.T) {
	var chunks []EvidenceChunk
	json.Unmarshal([]byte(fixtureEvidence), &chunks)
	size := utf8.RuneCountInString(chunkReference(chunks[0], 1) + chunks[0].Text + "\n\n")
	if len(PackEvidence("q", "semantic", "fp", chunks, size-1).Chunks) != 0 {
		t.Fatal("partial chunk packed")
	}
	e := PackEvidence("q", "semantic", "fp", chunks, size)
	if len(e.Chunks) != 1 || e.Characters != size {
		t.Fatal("unicode or reference budget wrong")
	}
}
func TestRAGPersistsRetryBranchAndPlainSkips(t *testing.T) {
	cfg := testConfig(t)
	cfg.RAG = ragFixture(t, "printf '%s' '"+fixtureEvidence+"'\n")
	fake := &fakeCompleter{failures: []error{errors.New("temporary provider failure")}}
	a, err := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	settings := DefaultSettings(cfg)
	settings.RAG = true
	settings.ContextStrategy = "full"
	settings.Compression = false
	session, err := a.Create(CreateSessionRequest{Chat: true, Settings: &settings})
	if err != nil {
		t.Fatal(err)
	}
	lease, _ := a.Attach(session.ID)
	if err = a.Submit(session.ID, lease, MessageRequest{Content: "question"}); err != nil {
		t.Fatal(err)
	}
	failed := waitState(t, a, session.ID, "failed")
	if failed.Operation.Evidence == nil {
		t.Fatal("missing evidence")
	}
	// A retry must work even if retrieval has become unavailable.
	os.WriteFile(cfg.RAG.Executable, []byte("#!/bin/sh\nexit 9\n"), 0700)
	if err = a.Retry(session.ID, lease); err != nil {
		t.Fatal(err)
	}
	waitState(t, a, session.ID, "completed")
	saved, err := a.store.Load(session.ID)
	if err != nil || !saved.Settings.RAG || saved.Operation.Evidence == nil {
		t.Fatal("persistence", err)
	}
	if _, err = a.CreateCheckpoint(session.ID, lease, "saved"); err != nil {
		t.Fatal(err)
	}
	branch, err := a.CreateBranch(session.ID, lease, BranchRequest{Name: "branch", Checkpoint: "saved"})
	if err != nil || !branch.Settings.RAG {
		t.Fatal("branch", err)
	}
	settings.RAG = false
	plain, _ := a.Create(CreateSessionRequest{Chat: true, Settings: &settings})
	plainLease, _ := a.Attach(plain.ID)
	a.Submit(plain.ID, plainLease, MessageRequest{Content: "plain"})
	result := waitState(t, a, plain.ID, "completed")
	if result.Operation.Evidence != nil {
		t.Fatal("plain retrieved")
	}
	var legacy Settings
	json.Unmarshal([]byte(`{"model":"deepseek-v4-flash","reasoning":"none"}`), &legacy)
	if normalizeSettings(legacy).RAG {
		t.Fatal("legacy changed")
	}
}
func TestRAGQueryAndCapabilities(t *testing.T) {
	s := &Session{Task: &TaskState{Objective: "original"}, Messages: []Message{{Role: "user", Content: "feedback"}}}
	if retrievalQuery(s) != "original\nLatest feedback: feedback" {
		t.Fatal(retrievalQuery(s))
	}
	r := httptest.NewRecorder()
	Server{}.Handler().ServeHTTP(r, httptest.NewRequest(http.MethodGet, "/v1/capabilities", nil))
	if r.Code != 200 || !strings.Contains(r.Body.String(), `"rag":true`) {
		t.Fatal(r.Body.String())
	}
}

type comparisonFake struct {
	requests []CompletionRequest
	failAt   int
}

func (f *comparisonFake) Complete(ctx context.Context, r CompletionRequest) (CompletionResponse, error) {
	f.requests = append(f.requests, r)
	if f.failAt == len(f.requests) {
		return CompletionResponse{}, errors.New("interrupted")
	}
	return CompletionResponse{Answer: "answer", Call: CallMetrics{Duration: time.Millisecond}}, nil
}
func TestComparisonIsolationAndResume(t *testing.T) {
	cfg := ragFixture(t, "printf '%s' '"+fixtureEvidence+"'\n")
	dir := t.TempDir()
	suite := filepath.Join(dir, "suite.json")
	out := filepath.Join(dir, "answers.json")
	os.WriteFile(suite, []byte(`{"questions":[{"id":"a","query":"question a","expected_facts":"SECRET_EXPECTED"},{"id":"b","query":"question b"}]}`), 0600)
	p := &comparisonFake{failAt: 3}
	s := Settings{Model: ProModel, Reasoning: "low"}
	if err := RunComparison(context.Background(), p, cfg, s, suite, out); err == nil {
		t.Fatal("expected interruption")
	}
	next := &comparisonFake{}
	if err := RunComparison(context.Background(), next, cfg, s, suite, out); err != nil {
		t.Fatal(err)
	}
	if len(next.requests) != 2 || !next.requests[0].Settings.RAG || next.requests[1].Settings.RAG {
		t.Fatal("resume/order")
	}
	for _, r := range append(p.requests, next.requests...) {
		data, _ := json.Marshal(r.Messages)
		if strings.Contains(string(data), "SECRET_EXPECTED") || strings.Contains(string(data), "memory") || r.Settings.Temperature != nil {
			t.Fatal("comparison leakage")
		}
		if !r.Settings.RAG && len(r.Messages) != 2 {
			t.Fatal("plain context")
		}
	}
	again := &comparisonFake{}
	if err := RunComparison(context.Background(), again, cfg, s, suite, out); err != nil || len(again.requests) != 0 {
		t.Fatal("regenerated", err)
	}
	os.WriteFile(cfg.Database, []byte("changed"), 0600)
	if err := RunComparison(context.Background(), again, cfg, s, suite, out); err == nil {
		t.Fatal("index mismatch accepted")
	}
}

func TestRAGCompressionAndMCPReuse(t *testing.T) {
	t.Run("compression", func(t *testing.T) {
		cfg := testConfig(t)
		cfg.Agent.RecentMessages = 1
		cfg.Agent.SummaryBatchMessages = 1
		cfg.RAG = ragFixture(t, "echo x >> \"$0.calls\"\nprintf '%s' '"+fixtureEvidence+"'\n")
		fake := &fakeCompleter{}
		a, _ := NewAgent(cfg, fake, NewDebugLogger(cfg.Daemon, ""))
		defer a.Close()
		settings := DefaultSettings(cfg)
		settings.RAG = true
		s, _ := a.Create(CreateSessionRequest{Chat: true, Settings: &settings})
		lease, _ := a.Attach(s.ID)
		a.Submit(s.ID, lease, MessageRequest{Content: "question"})
		waitState(t, a, s.ID, "completed")
		calls, _ := os.ReadFile(cfg.RAG.Executable + ".calls")
		if string(calls) != "x\n" {
			t.Fatal("retrieved during compression", string(calls))
		}
		fake.mu.Lock()
		defer fake.mu.Unlock()
		if len(fake.requests) != 2 {
			t.Fatal("compression not tested")
		}
		data, _ := json.Marshal(fake.requests[1])
		if strings.Contains(string(data), "Reference fact") {
			t.Fatal("evidence leaked to compression")
		}
	})
	t.Run("MCP", func(t *testing.T) {
		cfg := testConfig(t)
		cfg.Daemon.RequestTimeout = Duration(10 * time.Second)
		cfg.RAG = ragFixture(t, "printf '%s' '"+fixtureEvidence+"'\n")
		provider := &scriptedNative{}
		a, _ := NewAgent(cfg, provider, NewDebugLogger(cfg.Daemon, ""))
		defer a.Close()
		if err := a.MCP.Add("fixture", "fixture", []string{fixtureBinary(t)}); err != nil {
			t.Fatal(err)
		}
		settings := DefaultSettings(cfg)
		settings.RAG = true
		s, _ := a.Create(CreateSessionRequest{Chat: true, Settings: &settings})
		lease, _ := a.Attach(s.ID)
		a.Submit(s.ID, lease, MessageRequest{Content: "question"})
		awaitOperation(t, a, s.ID, "awaiting_tool_approval")
		os.WriteFile(cfg.RAG.Executable, []byte("#!/bin/sh\nexit 9\n"), 0700)
		if err := a.ApproveTool(s.ID, lease, false); err != nil {
			t.Fatal(err)
		}
		awaitOperation(t, a, s.ID, "completed")
		provider.mu.Lock()
		defer provider.mu.Unlock()
		for _, m := range provider.seen {
			data, _ := json.Marshal(m)
			if strings.Count(string(data), "Reference fact") != 1 {
				t.Fatal("missing or duplicated evidence")
			}
		}
	})
}
