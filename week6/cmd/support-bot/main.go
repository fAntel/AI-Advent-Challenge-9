package main

import (
	agent "advent-agent"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	path := flag.String("config", "", "support TOML configuration")
	once := flag.Bool("once", false, "poll/process once, then exit (first launch establishes the new-message cursor)")
	flag.Parse()
	c, e := agent.LoadSupportConfig(*path)
	if e == nil {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		e = agent.RunSupport(ctx, c, *once)
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
