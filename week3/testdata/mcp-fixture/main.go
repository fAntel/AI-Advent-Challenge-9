package main

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	if marker := os.Getenv("ADVENT_MCP_MARKER"); marker != "" {
		_ = os.WriteFile(marker, []byte("started"), 0600)
	}
	server := mcp.NewServer(&mcp.Implementation{Name: "fixture", Version: "1"}, &mcp.ServerOptions{PageSize: 3})
	server.AddReceivingMiddleware(func(next mcp.MethodHandler) mcp.MethodHandler {
		return func(ctx context.Context, method string, req mcp.Request) (mcp.Result, error) {
			if method == "tools/list" {
				if path := os.Getenv("ADVENT_MCP_FAIL_FILE"); path != "" {
					if _, err := os.Stat(path); err == nil {
						return nil, fmt.Errorf("fixture discovery failure")
					}
				}
			}
			result, err := next(ctx, method, req)
			if method == "tools/list" && err == nil {
				if list, ok := result.(*mcp.ListToolsResult); ok {
					list.TTLMs, _ = strconv.Atoi(os.Getenv("ADVENT_MCP_TTL_MS"))
					list.CacheScope = os.Getenv("ADVENT_MCP_SCOPE")
					if list.CacheScope == "" {
						list.CacheScope = "public"
					}
				}
			}
			return result, err
		}
	})
	for i := 0; i < 23; i++ {
		name := fmt.Sprintf("read_%02d", i)
		server.AddTool(&mcp.Tool{Name: name, Description: "read fixture item", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"item": map[string]any{"type": "string"}}, "required": []string{"item"}}, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}, StructuredContent: map[string]any{"ok": true}}, nil
		})
	}
	server.AddTool(&mcp.Tool{Name: "write", Description: "write fixture item", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if path := os.Getenv("ADVENT_MCP_WRITE_FILE"); path != "" {
			f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err == nil {
				_, _ = f.WriteString("1\n")
				_ = f.Close()
			}
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "written"}}}, nil
	})
	if os.Getenv("ADVENT_MCP_DYNAMIC") == "1" {
		server.AddTool(&mcp.Tool{Name: "trigger_change", Description: "invalidate tools", InputSchema: map[string]any{"type": "object"}, Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			server.AddTool(&mcp.Tool{Name: "added_after_change", Description: "new tool", InputSchema: map[string]any{"type": "object"}}, func(context.Context, *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
				return &mcp.CallToolResult{}, nil
			})
			return &mcp.CallToolResult{}, nil
		})
	}
	_ = server.Run(context.Background(), &mcp.StdioTransport{})
}
