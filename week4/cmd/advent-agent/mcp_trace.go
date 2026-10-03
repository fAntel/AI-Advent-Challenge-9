package main

import (
	agent "advent-agent"
	"fmt"
	"io"
)

func printMCPEvents(w io.Writer, events []agent.MCPEvent, last *int) {
	for _, event := range events {
		if event.Sequence <= *last {
			continue
		}
		name := event.Server
		if event.Tool != "" {
			name += "." + event.Tool
		}
		if name == "" {
			name = "unknown"
		}
		fmt.Fprintf(w, "MCP #%d %s %s: %s\n", event.Sequence, event.Action, name, event.Status)
		*last = event.Sequence
	}
}
