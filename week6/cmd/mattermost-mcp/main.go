package main

import (
	agent "advent-agent"
	"context"
	"flag"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"os"
)

func main() {
	path := flag.String("config", "", "support TOML configuration")
	flag.Parse()
	c, e := agent.LoadSupportConfig(*path)
	if e == nil {
		e = agent.MattermostServer(c, agent.NewMattermost(c.URL, os.Getenv("MATTERMOST_TOKEN"))).Run(context.Background(), &mcp.StdioTransport{})
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
