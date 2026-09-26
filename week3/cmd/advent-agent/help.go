package main

import (
	"fmt"
	"io"
)

func writeCLIUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  advent-agent [OPTIONS]
  advent-agent resume [SESSION_ID] [OPTIONS]
  advent-agent delete SESSION_ID
  advent-agent profiles
  advent-agent mcp COMMAND

MCP commands:
  mcp add NAME [--description TEXT] -- COMMAND [ARG...]
  mcp list
  mcp tools [--refresh] NAME
  mcp remove NAME

Chat options:`)
}

func writeMCPUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage:
  advent-agent mcp add NAME [--description TEXT] -- COMMAND [ARG...]
  advent-agent mcp list
  advent-agent mcp tools [--refresh] NAME
  advent-agent mcp remove NAME`)
}
