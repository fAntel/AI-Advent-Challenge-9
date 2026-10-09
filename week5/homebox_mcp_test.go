package agent

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type homeboxScript struct {
	round        int
	sawResult    bool
	sawDiscovery bool
	sawPhase     bool
}

func (*homeboxScript) Complete(context.Context, CompletionRequest) (CompletionResponse, error) {
	return CompletionResponse{}, nil
}
func (p *homeboxScript) CompleteNative(_ context.Context, messages []NativeMessage, _ Settings, _ []NativeTool) (NativeRound, error) {
	p.round++
	u := Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7}
	r := NativeRound{Completion: CompletionResponse{Metrics: Metrics{Requests: 1, Usage: u}, Usage: u, Call: CallMetrics{Usage: u, FinishReason: "tool_calls"}}}
	if p.round == 1 {
		for _, m := range messages {
			if m.Role == "system" && strings.Contains(m.Content.(string), "homebox") {
				p.sawDiscovery = true
			}
			if content, ok := m.Content.(string); ok && strings.Contains(content, "daemon-set phase") {
				p.sawPhase = true
			}
		}
		call := NativeToolCall{ID: "discover", Type: "function"}
		call.Function.Name = "list_mcp_tools"
		call.Function.Arguments = `{"server":"homebox"}`
		r.Message = NativeMessage{Role: "assistant", ToolCalls: []NativeToolCall{call}}
	} else if p.round == 2 {
		found := false
		for _, m := range messages {
			if m.Role == "tool" && strings.Contains(m.Content.(string), "search_entities") {
				found = true
			}
		}
		if !found {
			r.Message = NativeMessage{Role: "assistant", Content: "Tool was not discovered"}
			r.Completion.Answer = "Tool was not discovered"
			return r, nil
		}
		call := NativeToolCall{ID: "search", Type: "function"}
		call.Function.Name = "call_mcp_tool"
		call.Function.Arguments = `{"server":"homebox","tool":"search_entities","arguments":{"query":"lamp"}}`
		r.Message = NativeMessage{Role: "assistant", ToolCalls: []NativeToolCall{call}}
	} else {
		for _, m := range messages {
			if m.Role == "tool" && strings.Contains(m.Content.(string), "Desk lamp") {
				p.sawResult = true
			}
		}
		answer := "The item is Desk lamp."
		if !p.sawResult {
			answer = "HomeBox result missing"
		}
		r.Message = NativeMessage{Role: "assistant", Content: answer}
		r.Completion.Answer = answer
		r.Completion.Call.FinishReason = "stop"
	}
	return r, nil
}

func TestHomeBoxMCPAgentEndToEnd(t *testing.T) {
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/entities" || r.URL.Query().Get("q") != "lamp" || r.Header.Get("Authorization") != "Bearer mock-key" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
		}
		_, _ = w.Write([]byte(`{"items":[{"id":"item-1","name":"Desk lamp"}],"total":1}`))
	}))
	defer api.Close()
	dir := t.TempDir()
	ca := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: api.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	cfgFile := filepath.Join(dir, "homebox.json")
	data, _ := json.Marshal(map[string]string{"url": api.URL, "apiKey": "mock-key", "caFile": ca})
	if err := os.WriteFile(cfgFile, data, 0600); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "homebox-mcp")
	if out, err := exec.Command("go", "build", "-o", bin, "./cmd/homebox-mcp").CombinedOutput(); err != nil {
		t.Fatalf("build: %v: %s", err, out)
	}
	cfg := testConfig(t)
	cfg.Daemon.RequestTimeout = Duration(10 * time.Second)
	provider := &homeboxScript{}
	a, err := NewAgent(cfg, provider, NewDebugLogger(cfg.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.MCP.Add("homebox", "HomeBox inventory", []string{bin, "--config", cfgFile}); err != nil {
		t.Fatal(err)
	}
	s, err := a.Create(CreateSessionRequest{Chat: true})
	if err != nil {
		t.Fatal(err)
	}
	lease, err := a.Attach(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "Find the lamp in HomeBox"}); err != nil {
		t.Fatal(err)
	}
	completed := awaitOperation(t, a, s.ID, "completed")
	if !provider.sawDiscovery || !provider.sawResult || provider.sawPhase || provider.round != 3 {
		t.Fatalf("tool chain: %+v", provider)
	}
	if got := completed.Messages[len(completed.Messages)-1].Content; !strings.Contains(got, "Desk lamp") {
		t.Fatalf("answer: %q", got)
	}
	if !completed.Chat || completed.Task != nil {
		t.Fatalf("chat created a task: %+v", completed.Task)
	}
	stored, err := a.store.Load(s.ID)
	if err != nil || !stored.Chat || stored.Task != nil {
		t.Fatalf("chat mode was not persisted: %+v %v", stored, err)
	}
	if err := a.ContinueTask(s.ID, lease); err == nil || !strings.Contains(err.Error(), "chat sessions have no task lifecycle") {
		t.Fatalf("chat continuation: %v", err)
	}
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "Thanks"}); err != nil {
		t.Fatal(err)
	}
	followup := awaitOperation(t, a, s.ID, "completed")
	if followup.Task != nil || len(followup.Messages) != 4 {
		t.Fatalf("chat follow-up: %+v", followup)
	}
}
