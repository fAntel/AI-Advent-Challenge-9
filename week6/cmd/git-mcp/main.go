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
	root := flag.String("project", ".", "repository directory")
	flag.Parse()
	r, e := agent.ProjectRoot(*root)
	if e == nil {
		e = agent.GitServer(r).Run(context.Background(), &mcp.StdioTransport{})
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
