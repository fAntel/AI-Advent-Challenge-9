package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"
)

type report struct {
	StartedAt          time.Time       `json:"started_at"`
	Endpoint           string          `json:"endpoint"`
	Model              string          `json:"model"`
	ServerVersion      string          `json:"server_version"`
	ModelMetadata      json.RawMessage `json:"model_metadata,omitempty"`
	ContextTokens      int             `json:"context_tokens"`
	MaxGeneratedTokens int             `json:"max_generated_tokens"`
	ThinkingEnabled    bool            `json:"thinking_enabled"`
	Results            []result        `json:"results"`
	RunningModels      json.RawMessage `json:"running_models,omitempty"`
	Error              string          `json:"error,omitempty"`
}

func save(path string, r report) error {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".report-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err := f.Write(append(b, '\n')); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func run(ctx context.Context, args []string, out, stderr io.Writer) int {
	usage := func() {
		fmt.Fprintln(out, "Usage: local-llm demo [--model MODEL] [--output PATH] [--timeout 10m]\nRuns three independent prompts through local Ollama and saves JSON results.")
	}
	if len(args) == 0 || args[0] == "--help" || args[0] == "-h" {
		usage()
		return 0
	}
	if args[0] != "demo" {
		fmt.Fprintln(stderr, "expected demo command")
		return 2
	}
	flags := flag.NewFlagSet("demo", flag.ContinueOnError)
	flags.SetOutput(stderr)
	model := flags.String("model", defaultModel, "installed Ollama model")
	output := flags.String("output", "results/task0.json", "JSON report path (replaced on each run)")
	timeout := flags.Duration("timeout", 10*time.Minute, "timeout per generation request")
	if err := flags.Parse(args[1:]); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 || *timeout <= 0 || *model == "" || *output == "" {
		fmt.Fprintln(stderr, "invalid arguments: supply a model, output path and positive timeout")
		return 2
	}
	c := client{base: endpoint, http: &http.Client{}}
	r := report{StartedAt: time.Now().UTC(), Endpoint: endpoint, Model: *model, ContextTokens: 4096, MaxGeneratedTokens: 2048, Results: []result{}}
	checkCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	version, metadata, err := c.preflight(checkCtx, *model)
	cancel()
	r.ServerVersion, r.ModelMetadata = version, metadata
	if err != nil {
		r.Error = err.Error()
		fmt.Fprintln(stderr, err)
		if e := save(*output, r); e != nil {
			fmt.Fprintln(stderr, e)
		}
		return 1
	}
	fmt.Fprintf(out, "Local Ollama %s | %s\nContext: 4096 | Output limit: 2048 | Thinking: disabled\n", version, *model)
	// Check writability before starting an expensive generation.
	if err := save(*output, r); err != nil {
		fmt.Fprintln(stderr, "save report:", err)
		return 1
	}
	failed := false
	for i, t := range tasks {
		if ctx.Err() != nil {
			r.Error = ctx.Err().Error()
			failed = true
			break
		}
		fmt.Fprintf(out, "\n=== %d/3: %s ===\n%s\n\nAnswer:\n", i+1, t.Difficulty, t.Prompt)
		requestCtx, cancel := context.WithTimeout(ctx, *timeout)
		answer, err := c.chat(requestCtx, *model, t, out)
		cancel()
		r.Results = append(r.Results, answer)
		fmt.Fprintf(out, "\n\nElapsed: %.2fs | Generated: %d tokens | Completion: %s\n", answer.ElapsedSeconds, answer.EvalCount, answer.DoneReason)
		if err != nil {
			failed = true
			fmt.Fprintln(stderr, err)
		}
		if e := save(*output, r); e != nil {
			fmt.Fprintln(stderr, "save report:", e)
			return 1
		}
	}
	checkCtx, cancel = context.WithTimeout(ctx, 15*time.Second)
	err = c.json(checkCtx, "GET", "/api/ps", nil, &r.RunningModels)
	cancel()
	if err != nil {
		r.Error = err.Error()
		failed = true
		fmt.Fprintln(stderr, "capture running model:", err)
	}
	if err := save(*output, r); err != nil {
		fmt.Fprintln(stderr, "save report:", err)
		return 1
	}
	fmt.Fprintln(out, "Report:", *output)
	if failed {
		return 1
	}
	return 0
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(run(ctx, os.Args[1:], os.Stdout, os.Stderr))
}
