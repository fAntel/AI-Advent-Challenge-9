package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type scriptedNative struct {
	mu     sync.Mutex
	rounds int
	seen   [][]NativeMessage
}

func TestMCPApprovedWriteExecutesOnce(t *testing.T) {
	cfg := testConfig(t)
	cfg.Daemon.RequestTimeout = Duration(5 * time.Second)
	provider := &scriptedNative{}
	logger := NewDebugLogger(cfg.Daemon, "")
	writeFile := filepath.Join(t.TempDir(), "writes")
	t.Setenv("ADVENT_MCP_WRITE_FILE", writeFile)
	a, err := NewAgent(cfg, provider, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.MCP.Add("fixture", "fixture", []string{fixtureBinary(t)}); err != nil {
		t.Fatal(err)
	}
	s, err := a.Create(CreateSessionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := a.Attach(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "Do the task"}); err != nil {
		t.Fatal(err)
	}
	awaitOperation(t, a, s.ID, "awaiting_tool_approval")
	if err := a.ApproveTool(s.ID, lease, true); err != nil {
		t.Fatal(err)
	}
	completed := awaitOperation(t, a, s.ID, "completed")
	data, err := os.ReadFile(writeFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "1\n" {
		t.Fatalf("write executed %q", data)
	}
	if completed.Task.Phase != TaskPhasePlanning {
		t.Fatalf("phase changed: %+v", completed.Task)
	}
}

func (p *scriptedNative) Complete(context.Context, CompletionRequest) (CompletionResponse, error) {
	return CompletionResponse{Answer: "legacy"}, nil
}
func (p *scriptedNative) CompleteNative(_ context.Context, messages []NativeMessage, _ Settings, _ []NativeTool) (NativeRound, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.rounds++
	p.seen = append(p.seen, append([]NativeMessage(nil), messages...))
	u := Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7}
	r := NativeRound{Completion: CompletionResponse{Metrics: Metrics{Requests: 1, Usage: u}, Usage: u, Call: CallMetrics{Usage: u, FinishReason: "stop"}}}
	if p.rounds == 1 {
		call := NativeToolCall{ID: "tool-call-1", Type: "function"}
		call.Function.Name = "call_mcp_tool"
		call.Function.Arguments = `{"server":"fixture","tool":"write","arguments":{}}`
		r.Message = NativeMessage{Role: "assistant", ToolCalls: []NativeToolCall{call}, ReasoningContent: "need write"}
		r.Completion.Call.FinishReason = "tool_calls"
	} else {
		r.Message = NativeMessage{Role: "assistant", Content: "Final answer"}
		r.Completion.Answer = "Final answer"
	}
	return r, nil
}

func awaitOperation(t *testing.T, a *Agent, id, state string) Session {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s, err := a.Get(id)
		if err != nil {
			t.Fatal(err)
		}
		if s.Operation != nil && s.Operation.State == state {
			return *s
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("operation did not reach %s", state)
	return Session{}
}

func TestMCPApprovalResumesWithoutRepeatingModelCall(t *testing.T) {
	cfg := testConfig(t)
	cfg.Daemon.RequestTimeout = Duration(5 * time.Second)
	provider := &scriptedNative{}
	logger := NewDebugLogger(cfg.Daemon, "")
	a, err := NewAgent(cfg, provider, logger)
	if err != nil {
		t.Fatal(err)
	}
	bin := fixtureBinary(t)
	if err := a.MCP.Add("fixture", "fixture", []string{bin}); err != nil {
		t.Fatal(err)
	}
	s, err := a.Create(CreateSessionRequest{})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := a.Attach(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "Do the task"}); err != nil {
		t.Fatal(err)
	}
	pending := awaitOperation(t, a, s.ID, "awaiting_tool_approval")
	if pending.Task.Phase != TaskPhasePlanning || pending.Operation.PendingTool.Tool != "write" {
		t.Fatalf("unexpected pending operation: %+v", pending.Operation)
	}
	a.Close()
	resumed, err := NewAgent(cfg, provider, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer resumed.Close()
	lease, err = resumed.Attach(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := resumed.ApproveTool(s.ID, lease, false); err != nil {
		t.Fatal(err)
	}
	completed := awaitOperation(t, resumed, s.ID, "completed")
	if completed.Task.Phase != TaskPhasePlanning || completed.Task.Status != TaskStatusPaused {
		t.Fatalf("phase advanced: %+v", completed.Task)
	}
	if len(completed.Messages) != 2 || completed.Messages[1].Content != "Final answer" {
		t.Fatalf("unexpected dialog: %+v", completed.Messages)
	}
	if completed.Operation.Metrics.Requests != 2 || len(completed.Operation.Calls) != 2 {
		t.Fatalf("metrics: %+v", completed.Operation)
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.rounds != 2 || len(provider.seen) < 2 || !strings.Contains(provider.seen[1][len(provider.seen[1])-1].Content.(string), "denied") {
		t.Fatalf("denial not replayed: %+v", provider.seen)
	}
}
