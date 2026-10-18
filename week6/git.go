package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
	"os/exec"
	"strings"
	"time"
)

type GitSnapshot struct {
	Root     string    `json:"root"`
	Branch   string    `json:"branch"`
	Commit   string    `json:"commit"`
	Detached bool      `json:"detached"`
	Unborn   bool      `json:"unborn"`
	At       time.Time `json:"at"`
}

func ReadBranch(ctx context.Context, root string) (GitSnapshot, error) {
	actual, err := ProjectRoot(root)
	if err != nil {
		return GitSnapshot{}, err
	}
	run := func(args ...string) (string, error) {
		out, e := exec.CommandContext(ctx, "git", append([]string{"-C", actual}, args...)...).Output()
		return strings.TrimSpace(string(out)), e
	}
	b, be := run("symbolic-ref", "--quiet", "--short", "HEAD")
	h, he := run("rev-parse", "--verify", "HEAD")
	if ctx.Err() != nil {
		return GitSnapshot{}, ctx.Err()
	}
	if be != nil && he != nil {
		return GitSnapshot{}, fmt.Errorf("cannot read Git HEAD")
	}
	return GitSnapshot{Root: actual, Branch: b, Commit: h, Detached: be != nil, Unborn: he != nil, At: time.Now().UTC()}, nil
}
func GitServer(root string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "git-mcp", Version: "1.0.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "git_branch", Description: "Read the configured repository's current branch and HEAD; no arguments or mutations.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, GitSnapshot, error) {
		v, e := ReadBranch(ctx, root)
		return nil, v, e
	})
	return s
}
func FetchBranch(ctx context.Context, c Config, root string) (GitSnapshot, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.Command(c.GitMCP, "--project", root)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "DEEPSEEK_API_KEY=") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "advent-agent", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}, nil)
	if err != nil {
		return GitSnapshot{}, err
	}
	defer session.Close()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "git_branch", Arguments: map[string]any{}})
	if err != nil {
		return GitSnapshot{}, err
	}
	if result.IsError {
		return GitSnapshot{}, fmt.Errorf("git_branch failed")
	}
	data, err := json.Marshal(result.StructuredContent)
	if err != nil {
		return GitSnapshot{}, err
	}
	var snapshot GitSnapshot
	if err = json.Unmarshal(data, &snapshot); err != nil {
		return snapshot, err
	}
	if snapshot.Root != root || snapshot.At.IsZero() {
		return snapshot, fmt.Errorf("invalid Git MCP snapshot")
	}
	return snapshot, nil
}
