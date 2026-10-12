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
	if len(args) > 0 && args[0] == "score-bench" {
		runScoreBench(cfg, args[1:])
		return
	}
	if len(args) == 0 || args[0] != "compare" {
		fatal(errors.New("usage: advent-agent rag compare --suite FILE --output FILE [--matrix paired|enhanced|grounded|repeated --model MODEL --reasoning none --rag-strategy semantic]"))
	}
	fs := flag.NewFlagSet("rag compare", flag.ExitOnError)
	suite := fs.String("suite", "rag-suite.json", "question suite (expected facts are never sent)")
	output := fs.String("output", "reports/rag-answers.json", "resumable answer artifact; choose a new path to regenerate")
	settings := agent.DefaultSettings(cfg)
	fs.StringVar(&settings.Model, "model", settings.Model, "generation model")
	fs.StringVar(&settings.Reasoning, "reasoning", "none", "reasoning effort")
	fs.StringVar(&settings.RAGStrategy, "rag-strategy", "semantic", "retrieval strategy")
	fs.StringVar(&settings.RAGPlatform, "rag-platform", "", "platform filter")
	agent.RAGFlags(fs, &settings, settings)
	runs := fs.Int("runs", 3, "independent passes for the repeated matrix")
	matrix := fs.String("matrix", "paired", "comparison matrix: paired, enhanced (4 RAG modes), or grounded (RAG only)")
	fatalIf(fs.Parse(args[1:]))
	if *matrix != "paired" && *matrix != "enhanced" && *matrix != "grounded" && *matrix != "repeated" {
		fatal(errors.New("matrix must be paired, enhanced, grounded or repeated"))
	}
	if fs.NArg() != 0 {
		fatal(errors.New("unexpected comparison arguments"))
	}
	key := ""
	var err error
	if cfg.Provider.Name == "deepseek" {
		key, err = agent.DeepSeekAPIKey()
		fatalIf(err)
	}
	client, err := agent.NewConfiguredProvider(cfg, &http.Client{Timeout: time.Duration(cfg.Daemon.RequestTimeout)}, agent.NewDebugLogger(cfg.Daemon, key), key)
	fatalIf(err)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if *matrix == "repeated" {
		fatalIf(agent.RunRepeatedRAG(ctx, client, cfg, settings, *suite, *output, *runs))
	} else if *matrix == "grounded" {
		fatalIf(agent.RunGroundedComparison(ctx, client, cfg.RAG, settings, *suite, *output))
	} else if *matrix == "enhanced" {
		fatalIf(agent.RunEnhancedComparison(ctx, client, cfg.RAG, settings, *suite, *output))
	} else {
		fatalIf(agent.RunComparison(ctx, client, cfg.RAG, settings, *suite, *output))
	}
}

func runScoreBench(cfg agent.Config, args []string) {
	fs := flag.NewFlagSet("rag score-bench", flag.ExitOnError)
	input := fs.String("input", "results/task2/answers.json", "frozen Task2 repeated comparison")
	output := fs.String("output", "results/task3/scoring.json", "resumable scoring report")
	questions := fs.String("questions", "", "comma-separated question IDs; default all")
	runs := fs.Int("runs", 3, "independent measured passes, excluding warmup")
	baseline := fs.String("baseline", "", "single-pass scoring report for per-question deadlines")
	factor := fs.Float64("timeout-factor", 0, "deadline as a fraction of baseline duration (e.g. 0.6)")
	warmup := fs.Duration("warmup-timeout", 0, "separate warmup deadline; zero disables")
	fatalIf(fs.Parse(args))
	if fs.NArg() != 0 {
		fatal(errors.New("unexpected benchmark arguments"))
	}
	if cfg.Provider.Name != "ollama" {
		fatal(errors.New("score-bench requires an Ollama profile"))
	}
	client, err := agent.NewConfiguredProvider(cfg, &http.Client{Timeout: time.Duration(cfg.Daemon.RequestTimeout)}, agent.NewDebugLogger(cfg.Daemon, ""), "")
	fatalIf(err)
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	fatalIf(agent.RunScoreBench(ctx, client, cfg, agent.DefaultSettings(cfg), *input, *output, *questions, *runs, agent.ScoreBenchOptions{Baseline: *baseline, TimeoutFactor: *factor, WarmupTimeout: *warmup}))
}
