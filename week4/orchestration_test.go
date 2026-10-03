package agent

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type supplierFlowProvider struct {
	mu    sync.Mutex
	round int
	err   string
}

func (*supplierFlowProvider) Complete(context.Context, CompletionRequest) (CompletionResponse, error) {
	return CompletionResponse{}, nil
}

func (p *supplierFlowProvider) CompleteNative(_ context.Context, messages []NativeMessage, _ Settings, _ []NativeTool) (NativeRound, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.round++
	want := ""
	switch p.round {
	case 2:
		want = "search_entities"
	case 3:
		want = "Floor lamp"
	case 4:
		want = "LED bulb"
	case 5:
		want = "LAMP-LOW"
	case 6:
		want = "# HomeBox inventory report"
	case 7:
		want = "replenishment.md"
	}
	if want != "" {
		found := false
		for _, m := range messages {
			if m.Role == "tool" && strings.Contains(m.Content.(string), want) {
				found = true
			}
		}
		if !found {
			p.err = "round " + string(rune('0'+p.round)) + " did not see " + want
		}
	}
	u := Usage{PromptTokens: 5, CompletionTokens: 2, TotalTokens: 7}
	r := NativeRound{Completion: CompletionResponse{Metrics: Metrics{Requests: 1, Usage: u}, Usage: u, Call: CallMetrics{Usage: u, FinishReason: "tool_calls"}}}
	call := func(id, function, args string) NativeToolCall {
		c := NativeToolCall{ID: id, Type: "function"}
		c.Function.Name, c.Function.Arguments = function, args
		return c
	}
	var calls []NativeToolCall
	switch p.round {
	case 1:
		calls = []NativeToolCall{
			call("discover-homebox", "list_mcp_tools", `{"server":"homebox"}`),
			call("discover-supplier", "list_mcp_tools", `{"server":"supplier"}`),
			call("discover-report", "list_mcp_tools", `{"server":"report"}`),
		}
	case 2:
		calls = []NativeToolCall{call("inventory", "call_mcp_tool", `{"server":"homebox","tool":"search_entities","arguments":{}}`)}
	case 3:
		calls = []NativeToolCall{
			call("lamp", "call_mcp_tool", `{"server":"homebox","tool":"get_entity","arguments":{"id":"item-1"}}`),
			call("bulb", "call_mcp_tool", `{"server":"homebox","tool":"get_entity","arguments":{"id":"item-2"}}`),
		}
	case 4:
		calls = []NativeToolCall{
			call("lamp-offers", "call_mcp_tool", `{"server":"supplier","tool":"search_offers","arguments":{"query":"Desk lamp"}}`),
			call("bulb-offers", "call_mcp_tool", `{"server":"supplier","tool":"search_offers","arguments":{"query":"LED bulb"}}`),
		}
	case 5:
		calls = []NativeToolCall{call("summary", "call_mcp_tool", `{"server":"report","tool":"summarize_inventory","arguments":{"items":[{"id":"item-1","name":"Desk lamp","quantity":1},{"id":"item-2","name":"LED bulb","quantity":0},{"id":"item-3","name":"Floor lamp","quantity":4}]}}`)}
	case 6:
		content := "# HomeBox inventory report\n\nItems: 3\n\n- Desk lamp (`item-1`), quantity: 1\n- LED bulb (`item-2`), quantity: 0\n- Floor lamp (`item-3`), quantity: 4\n\n# Replenishment, target quantity 3\n\n- Desk lamp: buy 2 of LAMP-READY; $15 each plus $4 shipping = $34. LAMP-LOW is unavailable.\n- LED bulb: buy 3 of BULB-BASIC; $4 each plus $3 shipping = $15.\n- Floor lamp: no purchase needed.\n\nTotal: $49 USD.\n"
		args, _ := json.Marshal(map[string]any{"server": "report", "tool": "save_report", "arguments": map[string]any{"filename": "replenishment.md", "content": content}})
		calls = []NativeToolCall{call("save", "call_mcp_tool", string(args))}
	default:
		r.Message = NativeMessage{Role: "assistant", Content: "Saved replenishment.md using three MCP servers."}
		r.Completion.Answer = "Saved replenishment.md using three MCP servers."
		r.Completion.Call.FinishReason = "stop"
		return r, nil
	}
	r.Message = NativeMessage{Role: "assistant", ToolCalls: calls}
	return r, nil
}

func TestAgentOrchestratesHomeBoxSupplierAndReport(t *testing.T) {
	items := []map[string]any{
		{"id": "item-1", "name": "Desk lamp", "quantity": 1},
		{"id": "item-2", "name": "LED bulb", "quantity": 0},
		{"id": "item-3", "name": "Floor lamp", "quantity": 4},
	}
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer mock-key" {
			t.Errorf("missing auth: %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/entities":
			_ = json.NewEncoder(w).Encode(map[string]any{"items": items, "total": 3})
		case "/api/v1/entities/item-1":
			_ = json.NewEncoder(w).Encode(items[0])
		case "/api/v1/entities/item-2":
			_ = json.NewEncoder(w).Encode(items[1])
		default:
			t.Errorf("unexpected HomeBox request: %s", r.URL)
			http.NotFound(w, r)
		}
	}))
	defer api.Close()
	root := t.TempDir()
	ca := filepath.Join(root, "ca.pem")
	if err := os.WriteFile(ca, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: api.Certificate().Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "homebox.json")
	data, _ := json.Marshal(map[string]string{"url": api.URL, "apiKey": "mock-key", "caFile": ca})
	if err := os.WriteFile(config, data, 0600); err != nil {
		t.Fatal(err)
	}
	cfg := testConfig(t)
	cfg.Daemon.RequestTimeout = Duration(30 * time.Second)
	cfg.Daemon.LeaseTimeout = Duration(30 * time.Second)
	provider := &supplierFlowProvider{}
	a, err := NewAgent(cfg, provider, NewDebugLogger(cfg.Daemon, ""))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if err := a.MCP.Add("homebox", "HomeBox inventory", []string{pipelineBinary(t, "homebox-mcp"), "--config", config}); err != nil {
		t.Fatal(err)
	}
	if err := a.MCP.Add("supplier", "Supplier offer catalog", []string{pipelineBinary(t, "supplier-mcp")}); err != nil {
		t.Fatal(err)
	}
	reports := filepath.Join(root, "reports")
	if err := a.MCP.Add("report", "Inventory reports", []string{pipelineBinary(t, "report-mcp"), "--output-dir", reports}); err != nil {
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
	if err := a.Submit(s.ID, lease, MessageRequest{Content: "For inventory below 3, choose the lowest total cost available offer, then save a report."}); err != nil {
		t.Fatal(err)
	}
	pending := awaitOperation(t, a, s.ID, "awaiting_tool_approval")
	if pending.Operation.PendingTool.Server != "report" || pending.Operation.PendingTool.Tool != "save_report" {
		t.Fatalf("approval=%+v", pending.Operation.PendingTool)
	}
	if err := a.ApproveTool(s.ID, lease, true); err != nil {
		t.Fatal(err)
	}
	completed := awaitOperation(t, a, s.ID, "completed")
	provider.mu.Lock()
	providerErr, rounds := provider.err, provider.round
	provider.mu.Unlock()
	if providerErr != "" || rounds != 7 {
		t.Fatalf("provider rounds=%d error=%s", rounds, providerErr)
	}
	report, err := os.ReadFile(filepath.Join(reports, "replenishment.md"))
	if err != nil || !strings.Contains(string(report), "LAMP-READY") || !strings.Contains(string(report), "BULB-BASIC") || !strings.Contains(string(report), "Total: $49 USD") {
		t.Fatalf("report=%q err=%v", report, err)
	}
	var started []string
	var previous int
	for _, event := range completed.Operation.MCPEvents {
		if event.Sequence != previous+1 {
			t.Fatalf("event order: %+v", completed.Operation.MCPEvents)
		}
		previous = event.Sequence
		if event.Status == "started" {
			started = append(started, event.Server+"."+event.Tool)
		}
	}
	want := []string{"homebox.", "supplier.", "report.", "homebox.search_entities", "homebox.get_entity", "homebox.get_entity", "supplier.search_offers", "supplier.search_offers", "report.summarize_inventory", "report.save_report"}
	if strings.Join(started, ",") != strings.Join(want, ",") {
		t.Fatalf("routes=%v want=%v", started, want)
	}
	if completed.Operation.MCPEvents[len(completed.Operation.MCPEvents)-1].Status != "returned" {
		t.Fatalf("last event=%+v", completed.Operation.MCPEvents[len(completed.Operation.MCPEvents)-1])
	}
}
