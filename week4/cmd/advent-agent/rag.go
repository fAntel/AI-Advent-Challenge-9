package main

import (
	agent "advent-agent"
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func runRAG(cfg agent.Config, args []string) {
	if len(args) == 0 || args[0] != "compare" {
		fatal(errors.New("usage: advent-agent rag compare --suite FILE --output FILE [--model MODEL --reasoning low --rag-strategy semantic]"))
	}
	fs := flag.NewFlagSet("rag compare", flag.ExitOnError)
	suite := fs.String("suite", "rag-suite.json", "question suite (expected facts are never sent)")
	output := fs.String("output", "reports/rag-answers.json", "resumable answer artifact; choose a new path to regenerate")
	settings := agent.Settings{Model: agent.ProModel, Reasoning: "low"}
	fs.StringVar(&settings.Model, "model", agent.ProModel, "generation model")
	fs.StringVar(&settings.Reasoning, "reasoning", "low", "reasoning effort")
	fs.StringVar(&settings.RAGStrategy, "rag-strategy", "semantic", "retrieval strategy")
	fs.StringVar(&settings.RAGPlatform, "rag-platform", "", "platform filter")
	fs.IntVar(&settings.RAGLimit, "rag-limit", 5, "retrieval results")
	fs.IntVar(&settings.RAGBudget, "rag-budget", 6000, "evidence characters")
	fatalIf(fs.Parse(args[1:]))
	if fs.NArg() != 0 {
		fatal(errors.New("unexpected comparison arguments"))
	}
	key, err := agent.DeepSeekAPIKey()
	fatalIf(err)
	client := &agent.DeepSeekClient{HTTP: &http.Client{Timeout: time.Duration(cfg.Daemon.RequestTimeout)}, Endpoint: cfg.DeepSeek.Endpoint, APIKey: key, Logger: agent.NewDebugLogger(cfg.Daemon, key)}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	fatalIf(agent.RunComparison(ctx, client, cfg.RAG, settings, *suite, *output))
}
