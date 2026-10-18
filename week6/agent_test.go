package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func gitRun(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("git: %v %s", e, out)
	}
	return strings.TrimSpace(string(out))
}
func repo(t *testing.T) string {
	t.Helper()
	r := t.TempDir()
	gitRun(t, r, "init", "-q", "-b", "main")
	r, e := ProjectRoot(r)
	if e != nil {
		t.Fatal(e)
	}
	return r
}
func TestGitHelper(t *testing.T) {
	if os.Getenv("WEEK6_MCP_HELPER") != "1" {
		return
	}
	var root string
	for i, a := range os.Args {
		if a == "--project" {
			root = os.Args[i+1]
		}
	}
	err := GitServer(root).Run(context.Background(), &mcp.StdioTransport{})
	if err != nil {
		os.Exit(2)
	}
	os.Exit(0)
}
func TestGitMCP(t *testing.T) {
	root := repo(t)
	exe, _ := os.Executable()
	wrapper := filepath.Join(t.TempDir(), "mcp")
	script := "#!/bin/sh\nWEEK6_MCP_HELPER=1 exec '" + strings.ReplaceAll(exe, "'", "'\\''") + "' -test.run=TestGitHelper -- \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	c := Config{GitMCP: wrapper}
	get := func() GitSnapshot {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s, e := FetchBranch(ctx, c, root)
		if e != nil {
			t.Fatal(e)
		}
		return s
	}
	s := get()
	if s.Branch != "main" || !s.Unborn || s.Detached {
		t.Fatalf("unborn: %+v", s)
	}
	gitRun(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "commit", "--allow-empty", "-qm", "initial")
	gitRun(t, root, "checkout", "-qb", "next")
	s = get()
	if s.Branch != "next" || s.Unborn || s.Commit == "" {
		t.Fatalf("branch: %+v", s)
	}
	gitRun(t, root, "checkout", "--detach", "-q")
	s = get()
	if !s.Detached || s.Branch != "" {
		t.Fatalf("detached: %+v", s)
	}
	if _, e := ReadBranch(context.Background(), t.TempDir()); e == nil {
		t.Fatal("expected nonrepo error")
	}
}
func TestGrounding(t *testing.T) {
	ev := []Evidence{{ID: "a", Text: "WeekN TaskM: description"}}
	valid := `{"status":"answered","answer":"Use this format [1].","quotes":[{"chunk_id":"a","text":"WeekN TaskM: description"}],"clarification":""}`
	if _, e := ValidateAnswer(valid, ev); e != nil {
		t.Fatal(e)
	}
	for _, bad := range []string{strings.ReplaceAll(valid, "[1]", "[2]"), strings.ReplaceAll(valid, "description\"}],", "invented\"}],"), strings.ReplaceAll(valid, "[1]", ""), valid + " {}", `{"status":"unknown","answer":"guess","quotes":[],"clarification":"missing"}`} {
		if _, e := ValidateAnswer(bad, ev); e == nil {
			t.Fatalf("accepted %s", bad)
		}
	}
}
func TestRelevance(t *testing.T) {
	ev := []Evidence{{ID: "a", Text: "one"}, {ID: "b", Text: "two"}, {ID: "c", Text: "three"}}
	var calls []Call
	complete := func(context.Context, string, []Message) (string, Call, error) {
		return `{"scores":[{"id":"a","score":1},{"id":"b","score":2},{"id":"c","score":3}]}`, Call{}, nil
	}
	got, e := SelectEvidence(context.Background(), ev, "q", complete, &calls)
	if e != nil || len(got) != 2 || got[0].ID != "c" {
		t.Fatalf("%+v %v", got, e)
	}
	malformed := func(context.Context, string, []Message) (string, Call, error) {
		return `{"scores":[{"id":"a","score":2},{"id":"a","score":3}]}`, Call{}, nil
	}
	if _, e = SelectEvidence(context.Background(), ev, "q", malformed, &calls); e == nil {
		t.Fatal("accepted duplicate")
	}
}
func fakeEngine(t *testing.T) *Engine {
	t.Helper()
	e := NewEngine(Config{State: t.TempDir(), Model: "fake"})
	e.Fetch = func(_ context.Context, _ Config, r string) (GitSnapshot, error) {
		return GitSnapshot{Root: r, Branch: "main", At: time.Now().UTC()}, nil
	}
	e.Index = func(context.Context, Config, string, string, string) ([]byte, error) {
		return []byte(`[{"id":"doc","source":"README.md","line_start":1,"line_end":1,"text":"A project."}]`), nil
	}
	e.Complete = func(_ context.Context, purpose string, m []Message) (string, Call, error) {
		call := Call{Purpose: purpose}
		switch purpose {
		case "relevance":
			return `{"scores":[{"id":"doc","score":3}]}`, call, nil
		case "summary":
			return "User asked about the project.", call, nil
		default:
			return `{"status":"answered","answer":"A project [1].","quotes":[{"chunk_id":"doc","text":"A project."}],"clarification":""}`, call, nil
		}
	}
	return e
}
func TestSessionRecovery(t *testing.T) {
	e := fakeEngine(t)
	s, err := e.Create(repo(t))
	if err != nil {
		t.Fatal(err)
	}
	s, err = e.Ask(context.Background(), s.ID, "What project?", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Messages) != 2 || s.Operations[0].Git == nil {
		t.Fatalf("%+v", s)
	}
	original := e.Complete
	e.Complete = func(context.Context, string, []Message) (string, Call, error) {
		return "", Call{}, fmt.Errorf("provider down")
	}
	s, err = e.Ask(context.Background(), s.ID, "Next?", false)
	if err == nil || len(s.Messages) != 2 {
		t.Fatal("failure accepted")
	}
	if _, err = e.Ask(context.Background(), s.ID, "skip", false); err == nil {
		t.Fatal("pending not guarded")
	}
	// A fresh engine loads disk state and obtains new live context on retry.
	fresh := fakeEngine(t)
	fresh.Config = e.Config
	fresh.Complete = original
	fresh.Fetch = func(_ context.Context, _ Config, r string) (GitSnapshot, error) {
		return GitSnapshot{Root: r, Branch: "changed", At: time.Now().UTC()}, nil
	}
	s, err = fresh.Ask(context.Background(), s.ID, "", true)
	if err != nil {
		t.Fatal(err)
	}
	if s.Operations[2].Git.Branch != "changed" || len(s.Messages) != 4 {
		t.Fatal("retry did not refresh")
	}
	for i := 0; i < 5; i++ {
		s, err = fresh.Ask(context.Background(), s.ID, "More?", false)
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.Summary == "" || s.Summarized == 0 {
		t.Fatal("no summary")
	}
	if _, err = fresh.Get("../escape"); err == nil {
		t.Fatal("accepted unsafe id")
	}
}
func TestGitFailureAndUnknown(t *testing.T) {
	e := fakeEngine(t)
	e.Fetch = func(context.Context, Config, string) (GitSnapshot, error) {
		return GitSnapshot{}, fmt.Errorf("offline")
	}
	s, _ := e.Create(repo(t))
	s, err := e.Ask(context.Background(), s.ID, "project", false)
	if err != nil || s.Operations[0].Warning == "" || s.Operations[0].Answer == nil {
		t.Fatalf("%+v %v", s, err)
	}
	e.Index = func(context.Context, Config, string, string, string) ([]byte, error) { return []byte("[]"), nil }
	e.Complete = func(context.Context, string, []Message) (string, Call, error) {
		t.Fatal("unexpected generation")
		return "", Call{}, nil
	}
	s, err = e.Ask(context.Background(), s.ID, "unsupported", false)
	if err != nil || s.Operations[1].Answer.Status != "unknown" {
		t.Fatal(err)
	}
}
func TestBranchOnly(t *testing.T) {
	e := fakeEngine(t)
	e.Index = func(context.Context, Config, string, string, string) ([]byte, error) { return []byte("[]"), nil }
	e.Complete = func(_ context.Context, p string, m []Message) (string, Call, error) {
		var data struct {
			Evidence []Evidence `json:"evidence"`
		}
		_ = json.Unmarshal([]byte(m[1].Content), &data)
		ev := data.Evidence[0]
		b, _ := json.Marshal(Answer{Status: "answered", Answer: "Branch main [1].", Quotes: []Quote{{ev.ID, `"branch":"main"`}}})
		return string(b), Call{}, nil
	}
	s, _ := e.Create(repo(t))
	s, err := e.Ask(context.Background(), s.ID, "branch?", false)
	if err != nil || s.Operations[0].Answer == nil {
		t.Fatal(err)
	}
}
func TestProviderAndCancellation(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "test-secret")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if _, ok := body["tools"]; ok {
			t.Error("tools exposed")
		}
		if r.Header.Get("Authorization") != "Bearer test-secret" {
			t.Error("missing key")
		}
		fmt.Fprint(w, `{"choices":[{"message":{"role":"assistant","content":"{}"},"finish_reason":"length"}],"usage":{"total_tokens":5}}`)
	}))
	defer server.Close()
	p := Provider(Config{Endpoint: server.URL, Model: "fake", TimeoutSeconds: 3})
	_, call, err := p(context.Background(), "answer", nil)
	if err == nil || call.Usage.Total != 5 {
		t.Fatal("truncation accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err = p(ctx, "answer", nil); err == nil {
		t.Fatal("ignored cancellation")
	}
}

func TestInterruptedDiscardAndSummaryFailure(t *testing.T) {
	e := fakeEngine(t)
	s, _ := e.Create(repo(t))
	// Persisted start without a terminal result simulates daemon/process loss.
	s.Operations = append(s.Operations, Operation{Question: "interrupted", At: time.Now()})
	if err := e.save(s); err != nil {
		t.Fatal(err)
	}
	if _, err := e.Ask(context.Background(), s.ID, "next", false); err == nil {
		t.Fatal("interruption ignored")
	}
	if _, err := e.Discard(s.ID); err != nil {
		t.Fatal(err)
	}
	original := e.Complete
	e.Complete = func(ctx context.Context, p string, m []Message) (string, Call, error) {
		if p == "summary" {
			return "", Call{}, fmt.Errorf("summary failed")
		}
		return original(ctx, p, m)
	}
	for i := 0; i < 6; i++ {
		var err error
		s, err = e.Ask(context.Background(), s.ID, "question", false)
		if err != nil {
			t.Fatal(err)
		}
	}
	if s.Summarized != 0 || len(s.Messages) != 12 || !strings.Contains(s.Operations[len(s.Operations)-1].Warning, "Summary failed") {
		t.Fatalf("summary failure lost history: %+v", s)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Ask(ctx, s.ID, "cancelled", false); err == nil {
		t.Fatal("cancelled request accepted")
	}
	other := fakeEngine(t)
	if _, err := other.Get(s.ID); err == nil {
		t.Fatal("state isolation failed")
	}
}
func TestStaleIndexBlocksGeneration(t *testing.T) {
	e := fakeEngine(t)
	e.Index = func(context.Context, Config, string, string, string) ([]byte, error) {
		return nil, fmt.Errorf("stale; reindex")
	}
	e.Complete = func(context.Context, string, []Message) (string, Call, error) {
		t.Fatal("generation despite stale index")
		return "", Call{}, nil
	}
	s, _ := e.Create(repo(t))
	s, err := e.Ask(context.Background(), s.ID, "question", false)
	if err == nil || len(s.Messages) != 0 || s.Operations[0].Error == "" {
		t.Fatal("stale index not saved as failure")
	}
}
func TestDaemonRestart(t *testing.T) {
	dir, err := os.MkdirTemp("/tmp", "week6-daemon-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	c := Config{State: dir, Model: "fake", TimeoutSeconds: 3}
	start := func() (context.CancelFunc, chan error) {
		t.Helper()
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- Serve(ctx, c) }()
		for i := 0; i < 100; i++ {
			if Health(context.Background(), c) == nil {
				return cancel, done
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
		t.Fatal("daemon failed to start")
		return nil, nil
	}
	cancel, done := start()
	engine := NewEngine(c)
	s, err := engine.Create(repo(t))
	if err != nil {
		t.Fatal(err)
	}
	if err = Serve(context.Background(), c); err == nil {
		t.Fatal("second daemon took lock")
	}
	cancel()
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	cancel, done = start()
	defer func() { cancel(); <-done }()
	reqBody := fmt.Sprintf(`{"id":%q}`, s.ID)
	resp, err := HTTPClient(c).Post("http://unix/get", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out Response
	if err = json.NewDecoder(resp.Body).Decode(&out); err != nil || out.Session == nil || out.Session.Root != s.Root {
		t.Fatalf("restart: %+v %v", out, err)
	}
}
