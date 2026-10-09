package main

import (
	agent "advent-agent"
	"advent-agent/minichat"
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
)

const help = `Usage: mini-chat [OPTIONS]
       mini-chat resume SESSION_ID [OPTIONS]
       mini-chat sessions [OPTIONS]
       mini-chat eval --suite FILE --output FILE [OPTIONS]

A standalone RAG chat. No agent daemon is started.
Options: --db FILE --index FILE --indexer FILE --model NAME --reasoning LEVEL
Commands:
  /help /memory /history /exit
  /goal TEXT
  /constraint KEY=VALUE
  /definition KEY=VALUE
  /forget constraint|definition KEY
  /retry /discard
Every ordinary message retrieves documentation. Task memory is extracted
from user statements; ambiguous conflicting facts ask for clarification.
`

type lazyProvider struct {
	cfg    agent.Config
	client *agent.DeepSeekClient
}

func (p *lazyProvider) Complete(ctx context.Context, r agent.CompletionRequest) (agent.CompletionResponse, error) {
	if p.client == nil {
		key, err := agent.DeepSeekAPIKey()
		if err != nil {
			return agent.Completion{}, err
		}
		p.client = &agent.DeepSeekClient{HTTP: &http.Client{Timeout: time.Duration(p.cfg.Daemon.RequestTimeout)}, Endpoint: p.cfg.DeepSeek.Endpoint, APIKey: key, Logger: agent.NewDebugLogger(p.cfg.Daemon, key)}
	}
	return p.client.Complete(ctx, r)
}
func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	go func() { <-ctx.Done(); _ = os.Stdin.Close() }()
	if err := run(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
func run(ctx context.Context, args []string, in io.Reader, out, errOut io.Writer) error {
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Fprint(out, help)
			return nil
		}
	}
	command, id := "chat", ""
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		command = args[0]
		args = args[1:]
	}
	if command == "resume" {
		if len(args) == 0 {
			return errors.New("resume requires a session ID")
		}
		id = args[0]
		args = args[1:]
	}
	if command != "chat" && command != "resume" && command != "sessions" && command != "eval" {
		return errors.New(help)
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cfg, err := agent.LoadConfig()
	if err != nil {
		return err
	}
	settings := minichat.Defaults(cfg.RAG)
	fs := flag.NewFlagSet("mini-chat", flag.ContinueOnError)
	fs.SetOutput(errOut)
	db := fs.String("db", filepath.Join(filepath.Dir(exe), "data", "chat.sqlite"), "conversation database")
	fs.StringVar(&settings.Retrieval.Database, "index", settings.Retrieval.Database, "documentation index")
	fs.StringVar(&settings.Retrieval.Executable, "indexer", settings.Retrieval.Executable, "kotlin-index executable")
	fs.StringVar(&settings.Generation.Model, "model", settings.Generation.Model, "answer and memory model")
	fs.StringVar(&settings.Generation.Reasoning, "reasoning", settings.Generation.Reasoning, "reasoning effort")
	suite := fs.String("suite", "", "evaluation suite")
	output := fs.String("output", "", "evaluation output JSON")
	if err = fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if err = agent.ValidateDeepSeekSettings(settings.Generation); err != nil {
		return err
	}
	for _, path := range []*string{&settings.Retrieval.Database, &settings.Retrieval.Executable} {
		if *path != "" {
			*path, err = filepath.Abs(*path)
			if err != nil {
				return err
			}
		}
	}
	settings.Retrieval.Executable, settings.Retrieval.Database, err = settings.Retrieval.Paths()
	if err != nil {
		return err
	}
	store, err := minichat.Open(*db)
	if err != nil {
		return err
	}
	defer store.Close()
	engine := &minichat.Engine{Store: store, Provider: &lazyProvider{cfg: cfg}}
	if command == "sessions" {
		list, err := store.List()
		if err != nil {
			return err
		}
		for _, s := range list {
			fmt.Fprintf(out, "%s  %s  %d turns  %s\n", s.ID, s.Updated.Format(time.RFC3339), len(s.Turns), s.Memory.Goal.Value)
		}
		return nil
	}
	if command == "eval" {
		if *suite == "" || *output == "" {
			return errors.New("eval requires --suite and --output")
		}
		return minichat.Evaluate(ctx, engine, settings, *suite, *output, out)
	}
	var session *minichat.Session
	if command == "resume" {
		changed := false
		fs.Visit(func(f *flag.Flag) {
			if f.Name != "db" {
				changed = true
			}
		})
		if changed {
			return errors.New("resume uses saved settings; only --db can be overridden")
		}
		session, err = store.Load(id)
	} else {
		session, err = store.Create(settings)
	}
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "Session: %s\nType /help for commands.\n", session.ID)
	if session.Pending() != nil {
		fmt.Fprintln(out, "Pending turn recovered. Use /retry or /discard.")
	}
	return chatLoop(ctx, engine, session.ID, in, out, errOut)
}
func chatLoop(ctx context.Context, e *minichat.Engine, id string, in io.Reader, out, errOut io.Writer) error {
	scanner := bufio.NewScanner(in)
	scanner.Buffer(make([]byte, 4096), 128*1024)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		fmt.Fprint(out, "> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "/exit" {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		before, err := e.Store.Load(id)
		if err != nil {
			return err
		}
		var current *minichat.Session
		if strings.HasPrefix(line, "/") {
			cmd, value, _ := strings.Cut(line, " ")
			value = strings.TrimSpace(value)
			switch cmd {
			case "/help":
				fmt.Fprint(out, help)
			case "/memory":
				printJSON(out, map[string]any{"memory": before.Memory, "questions": before.Questions()})
			case "/history":
				for _, t := range before.Turns {
					fmt.Fprintf(out, "user: %s\n[%s] %s\n", t.Question, t.State, t.Answer)
					if t.Error != "" {
						fmt.Fprintln(out, "error:", t.Error)
					}
				}
			case "/goal":
				err = e.Correct(id, "goal", "", value, false)
			case "/constraint", "/definition":
				key, v, ok := strings.Cut(value, "=")
				if !ok {
					err = errors.New("usage: " + cmd + " KEY=VALUE")
				} else {
					err = e.Correct(id, strings.TrimPrefix(cmd, "/"), strings.TrimSpace(key), v, false)
				}
			case "/forget":
				parts := strings.Fields(value)
				if len(parts) != 2 || (parts[0] != "constraint" && parts[0] != "definition") {
					err = errors.New("usage: /forget constraint|definition KEY")
				} else {
					err = e.Correct(id, parts[0], parts[1], "", true)
				}
			case "/retry":
				current, err = e.Retry(ctx, id)
			case "/discard":
				err = e.Discard(id)
			default:
				err = errors.New("unknown command; use /help")
			}
		} else {
			current, err = e.Ask(ctx, id, line)
		}
		if err != nil {
			fmt.Fprintln(errOut, "error:", err)
		}
		if err == nil && current != nil && len(current.Turns) > 0 {
			t := current.Turns[len(current.Turns)-1]
			if t.State == "completed" {
				fmt.Fprintln(out, t.Answer)
			}
		}
		after, loadErr := e.Store.Load(id)
		if loadErr != nil {
			return loadErr
		}
		a, _ := json.Marshal(before.Memory)
		b, _ := json.Marshal(after.Memory)
		if string(a) != string(b) {
			fmt.Fprintln(out, "Task memory updated:")
			printJSON(out, after.Memory)
		}
	}
}
func printJSON(w io.Writer, v any) {
	b, _ := json.MarshalIndent(v, "", "  ")
	fmt.Fprintln(w, string(b))
}
