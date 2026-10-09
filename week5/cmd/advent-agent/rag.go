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
		fatal(errors.New("usage: advent-agent rag compare --suite FILE --output FILE [--matrix paired|enhanced|grounded --model MODEL --reasoning none --rag-strategy semantic]"))
	}
	fs := flag.NewFlagSet("rag compare", flag.ExitOnError)
	suite := fs.String("suite", "rag-suite.json", "question suite (expected facts are never sent)")
	output := fs.String("output", "reports/rag-answers.json", "resumable answer artifact; choose a new path to regenerate")
	settings := agent.Settings{Model: agent.FlashModel, Reasoning: "none"}
	fs.StringVar(&settings.Model, "model", agent.FlashModel, "generation model")
	fs.StringVar(&settings.Reasoning, "reasoning", "none", "reasoning effort")
	fs.StringVar(&settings.RAGStrategy, "rag-strategy", "semantic", "retrieval strategy")
	fs.StringVar(&settings.RAGPlatform, "rag-platform", "", "platform filter")
	agent.RAGFlags(fs, &settings, settings)
	matrix := fs.String("matrix", "paired", "comparison matrix: paired, enhanced (4 RAG modes), or grounded (RAG only)")
	fatalIf(fs.Parse(args[1:]))
	if *matrix != "paired" && *matrix != "enhanced" && *matrix != "grounded" {
		fatal(errors.New("matrix must be paired, enhanced or grounded"))
	}
	if fs.NArg() != 0 {
		fatal(errors.New("unexpected comparison arguments"))
	}
	key, err := agent.DeepSeekAPIKey()
	fatalIf(err)
	client := &agent.DeepSeekClient{HTTP: &http.Client{Timeout: time.Duration(cfg.Daemon.RequestTimeout)}, Endpoint: cfg.DeepSeek.Endpoint, APIKey: key, Logger: agent.NewDebugLogger(cfg.Daemon, key)}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if *matrix == "grounded" {
		fatalIf(agent.RunGroundedComparison(ctx, client, cfg.RAG, settings, *suite, *output))
	} else if *matrix == "enhanced" {
		fatalIf(agent.RunEnhancedComparison(ctx, client, cfg.RAG, settings, *suite, *output))
	} else {
		fatalIf(agent.RunComparison(ctx, client, cfg.RAG, settings, *suite, *output))
	}
}
