package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var mcpName = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,63}$`)

const mcpDiscoveryTimeout = 30 * time.Second

type stderrTail struct {
	mu   sync.Mutex
	data []byte
}

func (b *stderrTail) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, p...)
	if len(b.data) > 4096 {
		b.data = append([]byte(nil), b.data[len(b.data)-4096:]...)
	}
	return len(p), nil
}

func (b *stderrTail) String() string {
	if b == nil {
		return ""
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(strings.Map(func(r rune) rune {
		if r < 32 && r != '\n' && r != '\t' {
			return ' '
		}
		return r
	}, string(b.data)))
}

func withoutDeepSeekKey(env []string) []string {
	out := make([]string, 0, len(env))
	for _, value := range env {
		if !strings.HasPrefix(value, "DEEPSEEK_API_KEY=") {
			out = append(out, value)
		}
	}
	return out
}

type MCPServer struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Command     []string `json:"command"`
	Connected   bool     `json:"connected"`
	CacheFresh  bool     `json:"cache_fresh"`
	ToolCount   int      `json:"tool_count"`
}
type mcpEntry struct {
	MCPServer
	Tools       []*mcp.Tool `json:"tools,omitempty"`
	ExpiresAt   time.Time   `json:"expires_at,omitempty"`
	Scope       string      `json:"scope,omitempty"`
	session     *mcp.ClientSession
	invalidated bool
	stderr      *stderrTail
}
type MCPCatalog struct {
	mu      sync.Mutex
	path    string
	entries map[string]*mcpEntry
}

func NewMCPCatalog(stateDir string) (*MCPCatalog, error) {
	c := &MCPCatalog{path: filepath.Join(stateDir, "mcp-catalog.json"), entries: map[string]*mcpEntry{}}
	data, err := os.ReadFile(c.path)
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return nil, err
	}
	var saved []*mcpEntry
	if err = json.Unmarshal(data, &saved); err != nil {
		return nil, fmt.Errorf("decode MCP catalog: %w", err)
	}
	for _, e := range saved {
		if !mcpName.MatchString(e.Name) || len(e.Command) == 0 {
			return nil, errors.New("invalid MCP catalog entry")
		}
		e.Connected = false
		e.CacheFresh = false
		c.entries[e.Name] = e
	}
	return c, nil
}
func (c *MCPCatalog) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(c.path), 0700); err != nil {
		return err
	}
	names := make([]string, 0, len(c.entries))
	for n := range c.entries {
		names = append(names, n)
	}
	sort.Strings(names)
	list := make([]*mcpEntry, 0, len(names))
	for _, n := range names {
		list = append(list, c.entries[n])
	}
	data, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(c.path), ".mcp-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), c.path)
}
func (c *MCPCatalog) Add(name, description string, command []string) error {
	if !mcpName.MatchString(name) {
		return errors.New("MCP server name must start with a letter and contain only letters, digits, _ or -")
	}
	if len(command) == 0 || command[0] == "" {
		return errors.New("MCP command is required")
	}
	for _, part := range command {
		if part == "" {
			return errors.New("MCP command arguments cannot be empty")
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.entries[name]; ok {
		return fmt.Errorf("MCP server %q already exists", name)
	}
	c.entries[name] = &mcpEntry{MCPServer: MCPServer{Name: name, Description: description, Command: append([]string(nil), command...)}}
	if err := c.saveLocked(); err != nil {
		delete(c.entries, name)
		return err
	}
	return nil
}
func (c *MCPCatalog) List() []MCPServer {
	c.mu.Lock()
	defer c.mu.Unlock()
	names := make([]string, 0, len(c.entries))
	for n := range c.entries {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]MCPServer, 0, len(names))
	for _, n := range names {
		e := c.entries[n]
		v := e.MCPServer
		v.Connected = e.session != nil
		v.CacheFresh = len(e.Tools) > 0 && time.Now().Before(e.ExpiresAt)
		v.ToolCount = len(e.Tools)
		out = append(out, v)
	}
	return out
}
func (c *MCPCatalog) Remove(name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[name]
	if e == nil {
		return fmt.Errorf("MCP server %q not found", name)
	}
	delete(c.entries, name)
	if err := c.saveLocked(); err != nil {
		c.entries[name] = e
		return err
	}
	if e.session != nil {
		_ = e.session.Close()
	}
	return nil
}
func (c *MCPCatalog) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		if e.session != nil {
			_ = e.session.Close()
			e.session = nil
		}
	}
}
func (c *MCPCatalog) connectLocked(ctx context.Context, e *mcpEntry) error {
	if e.session != nil {
		return nil
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "advent-agent", Version: "0.3.0"}, &mcp.ClientOptions{ToolListChangedHandler: func(_ context.Context, _ *mcp.ToolListChangedRequest) {
		c.mu.Lock()
		if c.entries[e.Name] == e {
			e.ExpiresAt = time.Time{}
			e.invalidated = true
			_ = c.saveLocked()
		}
		c.mu.Unlock()
	}})
	cmd := exec.Command(e.Command[0], e.Command[1:]...)
	e.stderr = &stderrTail{}
	cmd.Stderr = e.stderr
	cmd.Env = withoutDeepSeekKey(os.Environ())
	connectCtx, cancel := context.WithTimeout(ctx, mcpDiscoveryTimeout)
	defer cancel()
	session, err := client.Connect(connectCtx, &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}, nil)
	if err != nil {
		return mcpDiagnostic(e, "connect", err)
	}
	e.session = session
	go func() {
		_ = session.Wait()
		c.mu.Lock()
		if e.session == session {
			e.session = nil
		}
		c.mu.Unlock()
	}()
	return nil
}
func mcpDiagnostic(e *mcpEntry, phase string, err error) error {
	if stderr := e.stderr.String(); stderr != "" {
		return fmt.Errorf("MCP server %q %s failed: %w; server stderr (untrusted): %s", e.Name, phase, err, stderr)
	}
	return fmt.Errorf("MCP server %q %s failed: %w", e.Name, phase, err)
}
func (c *MCPCatalog) Tools(ctx context.Context, name string, refresh bool) ([]*mcp.Tool, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e := c.entries[name]
	if e == nil {
		return nil, fmt.Errorf("MCP server %q not found", name)
	}
	if !refresh && len(e.Tools) > 0 && time.Now().Before(e.ExpiresAt) {
		return append([]*mcp.Tool(nil), e.Tools...), nil
	}
	// The SDK also caches individual pages. Reconnect before a forced or expired
	// catalog refresh so every page comes from one fresh server inventory.
	if e.session != nil && !e.invalidated {
		_ = e.session.Close()
		e.session = nil
	}
	if err := c.connectLocked(ctx, e); err != nil {
		e.ExpiresAt = time.Time{}
		_ = c.saveLocked()
		return nil, err
	}
	var all []*mcp.Tool
	cursor := ""
	seen := map[string]bool{}
	expiry := time.Time{}
	scope := "public"
	cacheable := true
	firstPage := true
	for {
		pageCtx, cancel := context.WithTimeout(ctx, mcpDiscoveryTimeout)
		res, err := e.session.ListTools(pageCtx, &mcp.ListToolsParams{Cursor: cursor})
		cancel()
		if err != nil {
			e.ExpiresAt = time.Time{}
			_ = c.saveLocked()
			return nil, mcpDiagnostic(e, "tools/list", err)
		}
		all = append(all, res.Tools...)
		if res.TTLMs <= 0 {
			cacheable = false
		} else if firstPage {
			expiry = time.Now().Add(time.Duration(res.TTLMs) * time.Millisecond)
		} else {
			next := time.Now().Add(time.Duration(res.TTLMs) * time.Millisecond)
			if next.Before(expiry) {
				expiry = next
			}
		}
		if res.CacheScope == "private" {
			scope = "private"
		} else if res.CacheScope != "" && res.CacheScope != "public" {
			cacheable = false
		}
		firstPage = false
		if res.NextCursor == "" {
			break
		}
		if seen[res.NextCursor] {
			e.ExpiresAt = time.Time{}
			_ = c.saveLocked()
			return nil, errors.New("MCP pagination cursor repeated")
		}
		seen[res.NextCursor] = true
		cursor = res.NextCursor
	}
	if !cacheable {
		expiry = time.Time{}
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Name < all[j].Name })
	e.Tools = all
	e.ExpiresAt = expiry
	e.Scope = scope
	e.ToolCount = len(all)
	e.invalidated = false
	if err := c.saveLocked(); err != nil {
		return nil, err
	}
	return append([]*mcp.Tool(nil), all...), nil
}
func (c *MCPCatalog) Call(ctx context.Context, server, tool string, args map[string]any) (*mcp.CallToolResult, error) {
	c.mu.Lock()
	e := c.entries[server]
	if e == nil {
		c.mu.Unlock()
		return nil, fmt.Errorf("MCP server %q not found", server)
	}
	if err := c.connectLocked(ctx, e); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	session := e.session
	c.mu.Unlock()
	// A dispatched call is never retried, even when the transport fails.
	return session.CallTool(ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
}
