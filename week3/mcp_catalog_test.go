package agent

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func fixtureBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "mcp-fixture")
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/mcp-fixture")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fixture: %v: %s", err, out)
	}
	return bin
}

func TestMCPCatalogLazyPaginationPersistence(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "started")
	t.Setenv("ADVENT_MCP_MARKER", marker)
	bin := fixtureBinary(t)
	c, err := NewMCPCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Add("fixture", "fixture description", []string{bin}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("server started at add: %v", err)
	}
	list := c.List()
	if len(list) != 1 || list[0].Connected {
		t.Fatalf("unexpected catalog: %+v", list)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tools, err := c.Tools(ctx, "fixture", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 24 {
		t.Fatalf("got %d tools", len(tools))
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(tools[0].Name, "read_") || tools[23].Name != "write" {
		t.Fatalf("bad order: %s..%s", tools[0].Name, tools[23].Name)
	}
	fresh, err := NewMCPCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh.entries["fixture"].Tools) != 24 {
		t.Fatal("cache did not survive restart")
	}
	fresh.Close()
	result, err := c.Call(ctx, "fixture", "read_00", map[string]any{"item": "a"})
	if err != nil {
		t.Fatal(err)
	}
	if result.StructuredContent == nil {
		t.Fatal("structured result missing")
	}
	if err := c.Remove("fixture"); err != nil {
		t.Fatal(err)
	}
	if len(c.List()) != 0 {
		t.Fatal("remove failed")
	}
}

func TestMCPInternalDiscoveryAndApproval(t *testing.T) {
	dir := t.TempDir()
	bin := fixtureBinary(t)
	c, err := NewMCPCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Add("fixture", "", []string{bin}); err != nil {
		t.Fatal(err)
	}
	a := &Agent{MCP: c}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	call := func(name, args string) NativeToolCall {
		v := NativeToolCall{ID: "call1", Type: "function"}
		v.Function.Name = name
		v.Function.Arguments = args
		return v
	}
	result, pending, err := a.internalMCP(ctx, "", "", nil, 0, 0, call("list_mcp_tools", `{"server":"fixture"}`))
	if err != nil || pending != nil || !strings.Contains(result, "next_cursor") {
		t.Fatalf("discovery: %s %+v %v", result, pending, err)
	}
	_, pending, err = a.internalMCP(ctx, "", "", nil, 0, 0, call("call_mcp_tool", `{"server":"fixture","tool":"write","arguments":{}}`))
	if err != nil || pending == nil {
		t.Fatalf("write approval: %+v %v", pending, err)
	}
	result, pending, err = a.internalMCP(ctx, "", "", nil, 0, 0, call("call_mcp_tool", `{"server":"fixture","tool":"read_00","arguments":{"item":"x"}}`))
	if err != nil || pending != nil || !strings.Contains(result, "structured_content") {
		t.Fatalf("read: %s %+v %v", result, pending, err)
	}
}

func TestMCPTTLRefreshFailureAndListChange(t *testing.T) {
	dir := t.TempDir()
	failure := filepath.Join(dir, "fail-list")
	t.Setenv("ADVENT_MCP_TTL_MS", "60000")
	t.Setenv("ADVENT_MCP_FAIL_FILE", failure)
	t.Setenv("ADVENT_MCP_DYNAMIC", "1")
	bin := fixtureBinary(t)
	c, err := NewMCPCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	if err := c.Add("dynamic", "", []string{bin}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tools, err := c.Tools(ctx, "dynamic", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 25 || !c.List()[0].CacheFresh {
		t.Fatalf("initial cache: %d %+v", len(tools), c.List())
	}
	reloaded, err := NewMCPCatalog(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reloaded.List()[0].CacheFresh || reloaded.List()[0].Connected {
		t.Fatalf("positive TTL did not survive restart: %+v", reloaded.List())
	}
	reloaded.Close()
	if _, err := c.Call(ctx, "dynamic", "trigger_change", map[string]any{}); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for c.List()[0].CacheFresh && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if c.List()[0].CacheFresh {
		t.Fatal("tools/list_changed did not invalidate cache")
	}
	tools, err = c.Tools(ctx, "dynamic", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) != 26 {
		t.Fatalf("refresh after change got %d tools", len(tools))
	}
	if err := os.WriteFile(failure, []byte("fail"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Tools(ctx, "dynamic", true); err == nil || !strings.Contains(err.Error(), "fixture discovery failure") {
		t.Fatalf("refresh error: %v", err)
	}
	if got := len(c.entries["dynamic"].Tools); got != 26 {
		t.Fatalf("snapshot lost after failed refresh: %d", got)
	}
	if c.List()[0].CacheFresh {
		t.Fatal("failed refresh left cache marked fresh")
	}
}

func TestMCPConnectionErrorIncludesBoundedServerStderr(t *testing.T) {
	c, err := NewMCPCatalog(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Add("broken", "", []string{"sh", "-c", "echo bridge-not-ready >&2; exit 7"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err = c.Tools(ctx, "broken", false)
	if err == nil || !strings.Contains(err.Error(), "bridge-not-ready") || !strings.Contains(err.Error(), "connect failed") {
		t.Fatalf("missing server diagnostic: %v", err)
	}
}

func TestMCPDiscoveryRespectsCallerDeadline(t *testing.T) {
	c, err := NewMCPCatalog(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Add("silent", "", []string{"sleep", "10"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err = c.Tools(ctx, "silent", false)
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("deadline error: %v", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("discovery ignored caller deadline: %s", time.Since(start))
	}
}
