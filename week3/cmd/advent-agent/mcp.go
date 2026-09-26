package main

import (
	agent "advent-agent"
	"errors"
	"fmt"
	"os"
	"strings"
)

func runMCP(api *agent.APIClient, cfg agent.Config, args []string) {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h" || args[0] == "help") {
		writeMCPUsage(os.Stdout)
		return
	}
	if len(args) == 0 {
		fatal(errors.New("usage: advent-agent mcp add|list|tools|remove"))
	}
	switch args[0] {
	case "add":
		if len(args) < 4 {
			fatal(errors.New("usage: advent-agent mcp add NAME [--description TEXT] -- COMMAND [ARG...]"))
		}
		name := args[1]
		description := ""
		i := 2
		for i < len(args) && args[i] != "--" {
			if args[i] != "--description" || i+1 >= len(args) {
				fatal(errors.New("expected --description TEXT and -- COMMAND"))
			}
			description = args[i+1]
			i += 2
		}
		if i >= len(args)-1 {
			fatal(errors.New("MCP command required after --"))
		}
		fatalIf(ensureDaemon(api, cfg))
		fatalIf(api.MCPAdd(name, description, args[i+1:]))
		fmt.Println("Added", name)
	case "list":
		if len(args) != 1 {
			fatal(errors.New("usage: advent-agent mcp list"))
		}
		fatalIf(ensureDaemon(api, cfg))
		servers, err := api.MCPList()
		fatalIf(err)
		for _, s := range servers {
			state := "disconnected"
			if s.Connected {
				state = "connected"
			}
			cache := "stale"
			if s.CacheFresh {
				cache = "fresh"
			}
			fmt.Printf("%s\t%s\t%s (%d tools)\t%s\n", s.Name, state, cache, s.ToolCount, strings.Join(s.Command, " "))
		}
	case "tools":
		refresh := false
		name := ""
		for _, arg := range args[1:] {
			if arg == "--refresh" {
				refresh = true
			} else if name == "" {
				name = arg
			} else {
				fatal(errors.New("usage: advent-agent mcp tools [--refresh] NAME"))
			}
		}
		if name == "" {
			fatal(errors.New("usage: advent-agent mcp tools [--refresh] NAME"))
		}
		fatalIf(ensureDaemon(api, cfg))
		fmt.Fprintf(os.Stderr, "Discovering MCP tools from %s...\n", name)
		tools, err := api.MCPTools(name, refresh)
		fatalIf(err)
		for _, t := range tools {
			fmt.Printf("%s\t%s\n", t.Name, strings.ReplaceAll(t.Description, "\n", " "))
		}
		fmt.Printf("Total tools: %d\n", len(tools))
	case "remove":
		if len(args) != 2 {
			fatal(errors.New("usage: advent-agent mcp remove NAME"))
		}
		fatalIf(ensureDaemon(api, cfg))
		fatalIf(api.MCPRemove(args[1]))
		fmt.Println("Removed", args[1])
	default:
		fatal(fmt.Errorf("unknown MCP command %q", args[0]))
	}
}
