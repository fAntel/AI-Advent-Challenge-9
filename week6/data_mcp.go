package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type dataMCPKey struct{}
type dataConnection struct {
	session     *mcp.ClientSession
	definitions []map[string]any
	names       map[string]string
}

// One subprocess per operation; credentials and SDK tokens never cross operations.
func connectDataMCP(ctx context.Context, path string, credentialsFile ...string) (context.Context, func(), error) {
	if path == "" {
		return ctx, func() {}, nil
	}
	cmd := exec.Command(path)
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "DEEPSEEK_API_KEY=") && !strings.HasPrefix(value, "MATTERMOST_TOKEN=") {
			cmd.Env = append(cmd.Env, value)
		}
	}
	if len(credentialsFile) > 0 && credentialsFile[0] != "" && os.Getenv("HEALBE_LOGIN") == "" && os.Getenv("HEALBE_PASSWORD") == "" {
		credentials, err := readDataCredentials(credentialsFile[0])
		if err != nil {
			return ctx, func() {}, err
		}
		if credentials != nil {
			clean := cmd.Env[:0]
			for _, value := range cmd.Env {
				if !strings.HasPrefix(value, "HEALBE_LOGIN=") && !strings.HasPrefix(value, "HEALBE_PASSWORD=") {
					clean = append(clean, value)
				}
			}
			cmd.Env = clean
			cmd.Env = append(cmd.Env, "HEALBE_LOGIN="+credentials.Login, "HEALBE_PASSWORD="+credentials.Password)
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "advent-agent", Version: "1.0.0"}, nil)
	session, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}, nil)
	if err != nil {
		return ctx, func() {}, fmt.Errorf("data MCP connection failed: %w", err)
	}
	close := func() { _ = session.Close() }
	d := &dataConnection{session: session, names: map[string]string{}}
	cursor := ""
	for {
		list, err := session.ListTools(ctx, &mcp.ListToolsParams{Cursor: cursor})
		if err != nil {
			close()
			return ctx, func() {}, fmt.Errorf("data MCP discovery failed: %w", err)
		}
		for _, tool := range list.Tools {
			name := "healbe_" + tool.Name
			if _, ok := d.names[name]; ok {
				close()
				return ctx, func() {}, fmt.Errorf("duplicate data MCP tool")
			}
			d.names[name] = tool.Name
			d.definitions = append(d.definitions, map[string]any{"type": "function", "function": map[string]any{
				"name": name, "description": tool.Description, "parameters": tool.InputSchema,
			}})
		}
		if list.NextCursor == "" {
			break
		}
		if list.NextCursor == cursor || len(d.names) > 100 {
			close()
			return ctx, func() {}, fmt.Errorf("invalid data MCP pagination")
		}
		cursor = list.NextCursor
	}
	return context.WithValue(ctx, dataMCPKey{}, d), close, nil
}

func callDataMCP(ctx context.Context, name string, args map[string]json.RawMessage) (string, error) {
	d, ok := ctx.Value(dataMCPKey{}).(*dataConnection)
	if !ok {
		return "", fmt.Errorf("data MCP is not configured")
	}
	target, ok := d.names[name]
	if !ok {
		return "", fmt.Errorf("unknown data MCP tool")
	}
	result, err := d.session.CallTool(ctx, &mcp.CallToolParams{Name: target, Arguments: args})
	if err != nil {
		return "", fmt.Errorf("data MCP transport failed; write outcome may be unknown; read back before retrying")
	}
	parts := []string{}
	for _, content := range result.Content {
		if t, ok := content.(*mcp.TextContent); ok {
			parts = append(parts, t.Text)
		}
	}
	body := strings.Join(parts, "\n")
	if body == "" && result.StructuredContent != nil {
		raw, _ := json.Marshal(result.StructuredContent)
		body = string(raw)
	}
	if len(body) > maxToolBytes {
		return "", fmt.Errorf("data MCP result too large; use a smaller download limit; do not repeat uploads")
	}
	if result.IsError {
		return "", fmt.Errorf("data MCP: %s", body)
	}
	return body, nil
}

// Keep explicit tool credentials out of operation records. User-entered chat text
// remains part of conversation history, as documented.
func redactToolArguments(raw string) string {
	var args map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &args) != nil {
		return raw
	}
	if _, ok := args["password"]; !ok {
		return raw
	}
	args["password"] = json.RawMessage(`"[redacted]"`)
	b, _ := json.Marshal(args)
	return string(b)
}

// Runtime inventory describes only this operation, never tools found in source.
func runtimeToolInventory(ctx context.Context, c Config) string {
	var b strings.Builder
	b.WriteString("Built-in harness tools (not MCP):\n")
	for _, definition := range toolDefinitions() {
		name := definition["function"].(map[string]any)["name"].(string)
		if name != "git_branch" {
			fmt.Fprintln(&b, name)
		}
	}
	if c.GitMCP != "" {
		b.WriteString("\nGit MCP: configured on demand; not probed by this inventory.\ngit_branch\n")
	}
	if d, ok := ctx.Value(dataMCPKey{}).(*dataConnection); ok {
		fmt.Fprintf(&b, "\nHEALBE data MCP: connected; discovered %d tools in this operation.\n", len(d.names))
		names := make([]string, 0, len(d.names))
		for name := range d.names {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			fmt.Fprintln(&b, name)
		}
	} else {
		b.WriteString("\nHEALBE data MCP: not connected in this operation.\n")
	}
	b.WriteString("\nMattermost MCP belongs to support-bot and is not connected to this developer session.\n")
	return b.String()
}

// InspectTools connects directly using CLI configuration; it does not consult a
// potentially older running daemon or call a model or account API.
func InspectTools(ctx context.Context, c Config) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	ctx, close, err := connectDataMCP(ctx, c.DataMCP, c.DataMCPCredentials)
	if err != nil {
		return "", err
	}
	defer close()
	return "Direct CLI inspection (not the running daemon):\n" + runtimeToolInventory(ctx, c), nil
}

type dataCredentials struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func readDataCredentials(path string) (*dataCredentials, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return nil, fmt.Errorf("data MCP credentials must be a private regular file (mode 0600)")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("cannot read data MCP credentials file")
	}
	var c dataCredentials
	if json.Unmarshal(raw, &c) != nil || c.Login == "" || c.Password == "" {
		return nil, fmt.Errorf("invalid data MCP credentials file; expected login and password")
	}
	return &c, nil
}
