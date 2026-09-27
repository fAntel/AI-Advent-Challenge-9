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

func pipelineBinary(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if out, err := exec.Command("go", "build", "-o", path, "./cmd/"+name).CombinedOutput(); err != nil {
		t.Fatalf("build %s: %v: %s", name, err, out)
	}
	return path
}

func TestPipelineHomeBoxReportAndSchedule(t *testing.T) {
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/entities" || r.URL.Query().Get("q") != "lamp" || r.Header.Get("Authorization") != "Bearer mock-key" {
			t.Errorf("unexpected HomeBox request %s", r.URL)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"items":[{"id":"item-1","name":"Desk lamp","quantity":1}],"total":1}`))
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
	catalog, err := NewMCPCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close()
	if err := catalog.Add("homebox", "inventory", []string{pipelineBinary(t, "homebox-mcp"), "--config", config}); err != nil {
		t.Fatal(err)
	}
	reports := filepath.Join(root, "reports")
	if err := catalog.Add("report", "reports", []string{pipelineBinary(t, "report-mcp"), "--output-dir", reports}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "pipelines.toml")
	pipelines, err := NewPipelineStore(path, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := pipelines.Add("lamps"); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	steps := []PipelineStep{
		{Name: "search", Server: "homebox", Tool: "search_entities", ArgumentsJSON: `{"query":"lamp"}`},
		{Name: "summary", Server: "report", Tool: "summarize_inventory", BindingsJSON: `{"items":"search:/items"}`},
		{Name: "save", Server: "report", Tool: "save_report", ArgumentsJSON: `{"filename":"lamps.md"}`, BindingsJSON: `{"content":"summary:/markdown"}`},
	}
	for _, step := range steps {
		if err := pipelines.AddStep(ctx, "lamps", step); err != nil {
			t.Fatal(err)
		}
	}
	run, err := pipelines.Run(ctx, "lamps")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(run.Steps, ",") != "search,summary,save" || run.Result != filepath.Join(reports, "lamps.md") {
		t.Fatalf("run=%+v", run)
	}
	checkReport := func() {
		t.Helper()
		content, err := os.ReadFile(filepath.Join(reports, "lamps.md"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(content), "Desk lamp") || !strings.Contains(string(content), "item-1") || !strings.Contains(string(content), "Items: 1") {
			t.Fatalf("report=%q", content)
		}
	}
	checkReport()
	reloaded, err := NewPipelineStore(path, catalog)
	if err != nil {
		t.Fatal(err)
	}
	scheduler, err := NewScheduler(filepath.Join(root, "schedules.toml"), catalog, NewEventHub(), 20*time.Second, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	scheduler.SetPipelines(reloaded)
	if err := scheduler.Add(ctx, ScheduleJob{Name: "lamp-report", Pipeline: "lamps", Every: "1h"}); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(reports, "lamps.md")); err != nil {
		t.Fatal(err)
	}
	if err := scheduler.Run("lamp-report"); err != nil {
		t.Fatal(err)
	}
	checkReport()
	jobs := scheduler.List()
	if len(jobs) != 1 || jobs[0].LastError != "" || jobs[0].LastResult != filepath.Join(reports, "lamps.md") {
		t.Fatalf("jobs=%+v", jobs)
	}
	second, err := NewScheduler(filepath.Join(root, "schedules.toml"), catalog, NewEventHub(), 20*time.Second, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	second.SetPipelines(reloaded)
	if err := second.Run("lamp-report"); err != nil {
		t.Fatal(err)
	}
	checkReport()
}

func TestPipelineMissingFieldStopsBeforeWrite(t *testing.T) {
	root := t.TempDir()
	catalog, err := NewMCPCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close()
	marker := filepath.Join(root, "writes")
	t.Setenv("ADVENT_MCP_WRITE_FILE", marker)
	if err := catalog.Add("fixture", "fixture", []string{fixtureBinary(t)}); err != nil {
		t.Fatal(err)
	}
	p, err := NewPipelineStore(filepath.Join(root, "pipelines.toml"), catalog)
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Add("broken"); err != nil {
		t.Fatal(err)
	}
	if err := p.AddStep(context.Background(), "broken", PipelineStep{Name: "read", Server: "fixture", Tool: "read_00", ArgumentsJSON: `{"item":"x"}`}); err != nil {
		t.Fatal(err)
	}
	if err := p.AddStep(context.Background(), "broken", PipelineStep{Name: "write", Server: "fixture", Tool: "write", BindingsJSON: `{"value":"read:/missing"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Run(context.Background(), "broken"); err == nil || !strings.Contains(err.Error(), "field \"missing\" not found") {
		t.Fatalf("run error=%v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("write was called: %v", err)
	}
}

func TestPipelineRejectsInvalidInputAndToolError(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ADVENT_MCP_ERROR_TOOL", "1")
	catalog, err := NewMCPCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close()
	marker := filepath.Join(root, "writes")
	t.Setenv("ADVENT_MCP_WRITE_FILE", marker)
	if err := catalog.Add("fixture", "fixture", []string{fixtureBinary(t)}); err != nil {
		t.Fatal(err)
	}
	p, err := NewPipelineStore(filepath.Join(root, "pipelines.toml"), catalog)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"invalid", "failed"} {
		if err := p.Add(name); err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()
	first := PipelineStep{Name: "first", Server: "fixture", Tool: "read_00", ArgumentsJSON: `{"item":"x"}`}
	if err := p.AddStep(ctx, "invalid", first); err != nil {
		t.Fatal(err)
	}
	if err := p.AddStep(ctx, "invalid", first); err == nil {
		t.Fatal("duplicate step accepted")
	}
	if err := p.AddStep(ctx, "invalid", PipelineStep{Name: "second", Server: "fixture", Tool: "read_00", BindingsJSON: `{"item":"first:/ok"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Run(ctx, "invalid"); err == nil || !strings.Contains(err.Error(), "invalid tool arguments") {
		t.Fatalf("invalid input error=%v", err)
	}
	if err := p.AddStep(ctx, "failed", PipelineStep{Name: "first", Server: "fixture", Tool: "tool_error"}); err != nil {
		t.Fatal(err)
	}
	if err := p.AddStep(ctx, "failed", PipelineStep{Name: "write", Server: "fixture", Tool: "write"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Run(ctx, "failed"); err == nil || !strings.Contains(err.Error(), "fixture failure") {
		t.Fatalf("MCP error=%v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("write was called: %v", err)
	}
	p.mu.Lock()
	p.running["failed"] = true
	p.mu.Unlock()
	if _, err := p.Run(ctx, "failed"); err == nil || !strings.Contains(err.Error(), "already running") {
		t.Fatalf("overlap error=%v", err)
	}
}
