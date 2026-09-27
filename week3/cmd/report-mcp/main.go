package main

import (
	agent "advent-agent"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type summarizeArgs struct {
	Items []map[string]any `json:"items" jsonschema:"required,HomeBox items to summarize"`
}

type saveArgs struct {
	Filename string `json:"filename" jsonschema:"required,Report filename within the configured output directory"`
	Content  string `json:"content" jsonschema:"required,Markdown report content"`
}

func summarize(items []map[string]any) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# HomeBox inventory report\n\nItems: %d\n\n", len(items))
	for _, item := range items {
		name, _ := item["name"].(string)
		id, _ := item["id"].(string)
		quantity, _ := item["quantity"].(float64)
		name = strings.ReplaceAll(name, "\n", " ")
		fmt.Fprintf(&b, "- %s (`%s`), quantity: %g\n", name, id, quantity)
	}
	return b.String()
}

func save(dir, filename, content string) (string, error) {
	if filename == "" || filename == "." || filename == ".." || filepath.Base(filename) != filename || strings.ContainsAny(filename, "/\\") {
		return "", errors.New("filename must be a single file name")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	file, err := os.CreateTemp(dir, ".report-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.WriteString(content)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return "", err
	}
	if closeErr != nil {
		return "", closeErr
	}
	path := filepath.Join(dir, filename)
	if err := os.Rename(file.Name(), path); err != nil {
		return "", err
	}
	return path, nil
}

func newServer(outputDir string) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "report-mcp", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "summarize_inventory", Description: "Create a deterministic Markdown summary of HomeBox inventory items.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(_ context.Context, _ *mcp.CallToolRequest, input summarizeArgs) (*mcp.CallToolResult, any, error) {
		markdown := summarize(input.Items)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: markdown}}}, map[string]any{"markdown": markdown, "itemCount": len(input.Items)}, nil
	})
	mcp.AddTool(s, &mcp.Tool{Name: "save_report", Description: "Atomically save a Markdown report in the configured output directory.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false}}, func(_ context.Context, _ *mcp.CallToolRequest, input saveArgs) (*mcp.CallToolResult, any, error) {
		path, err := save(outputDir, input.Filename, input.Content)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: path}}}, map[string]string{"path": path}, nil
	})
	return s
}

func main() {
	outputDir := flag.String("output-dir", "", "directory for saved reports")
	flag.Parse()
	if *outputDir == "" {
		cfg, err := agent.LoadConfig()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		*outputDir = filepath.Join(cfg.StateDir, "reports")
	}
	if !filepath.IsAbs(*outputDir) {
		fmt.Fprintln(os.Stderr, "output-dir must be absolute")
		os.Exit(2)
	}
	if err := newServer(*outputDir).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
