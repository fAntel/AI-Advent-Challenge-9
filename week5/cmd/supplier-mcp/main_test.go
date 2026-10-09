package main

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSupplierSearchAndReadOnlyAnnotation(t *testing.T) {
	server := newServer()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := server.Connect(context.Background(), st, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	listed, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 1 || listed.Tools[0].Name != "search_offers" || listed.Tools[0].Annotations == nil || !listed.Tools[0].Annotations.ReadOnlyHint {
		t.Fatalf("tools=%+v", listed.Tools)
	}
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "search_offers", Arguments: map[string]any{"query": "desk lamp"}})
	if err != nil || result.IsError {
		t.Fatalf("call=%+v err=%v", result, err)
	}
	structured, ok := result.StructuredContent.(map[string]any)
	if !ok {
		t.Fatalf("structured=%T", result.StructuredContent)
	}
	items, ok := structured["offers"].([]any)
	if !ok || len(items) != 3 {
		t.Fatalf("offers=%+v", structured["offers"])
	}
	if offers[0].AvailableQuantity != 0 || offers[1].AvailableQuantity < 2 {
		t.Fatal("fixture no longer distinguishes unavailable and available offers")
	}
	if got := search("shelf"); len(got) != 0 {
		t.Fatalf("unexpected matches: %+v", got)
	}
}
