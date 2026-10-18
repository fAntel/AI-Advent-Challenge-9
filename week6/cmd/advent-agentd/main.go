package main

import (
	agent "advent-agent"
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	path := flag.String("config", "", "TOML configuration path")
	flag.Parse()
	c, e := agent.LoadConfig(*path)
	if e == nil {
		ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer done()
		if flag.NArg() == 1 && flag.Arg(0) == "stop" {
			var resp *http.Response
			req, _ := http.NewRequestWithContext(ctx, "POST", "http://unix/stop", nil)
			resp, e = agent.HTTPClient(c).Do(req)
			if resp != nil {
				resp.Body.Close()
			}
		} else if flag.NArg() != 0 {
			e = fmt.Errorf("usage: advent-agentd [--config PATH] [stop]")
		} else {
			e = agent.Serve(ctx, c)
		}
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
