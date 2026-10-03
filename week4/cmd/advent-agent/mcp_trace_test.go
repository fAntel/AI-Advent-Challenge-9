package main

import (
	agent "advent-agent"
	"bytes"
	"strings"
	"testing"
)

func TestPrintMCPEventsExactlyOnceAcrossPolls(t *testing.T) {
	var out bytes.Buffer
	last := 0
	events := []agent.MCPEvent{
		{Sequence: 1, Action: "discover", Server: "homebox", Status: "started"},
		{Sequence: 2, Action: "discover", Server: "homebox", Status: "returned"},
		{Sequence: 3, Action: "call", Server: "homebox", Tool: "search_entities", Status: "returned"},
	}
	printMCPEvents(&out, events[:2], &last)
	printMCPEvents(&out, events, &last)
	printMCPEvents(&out, events, &last)
	if last != 3 || strings.Count(out.String(), "MCP #") != 3 || !strings.Contains(out.String(), "MCP #3 call homebox.search_entities: returned") {
		t.Fatalf("last=%d output=%q", last, out.String())
	}
}
