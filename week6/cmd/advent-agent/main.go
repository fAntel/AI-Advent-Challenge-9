package main

import (
	agent "advent-agent"
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const help = `Repository assistant
  /help QUESTION   Give a project goal (same as plain text)
  /help            Show these commands
  /retry           Retry failed work; answer failures retry reporting only
  /discard         Abandon an operation; saved edits remain
  /summary         Show conversation summary
  /stats           Show provider usage for this session
  /exit            Leave chat; the daemon remains running
Examples:
  /help How are weeks and tasks organized?
  /help What format should a commit message use?
  Which branch am I on?
  Find every use of the Widget API.
  Update the README to match the current implementation.
Edits save directly inside the session project; .git and daemon state are protected.`

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
func run() error {
	path := flag.String("config", "", "TOML configuration path")
	project := flag.String("project", ".", "session project directory (defaults to working directory)")
	prompt := flag.String("p", "", "one-shot project goal")
	_ = flag.Bool("chat", false, "interactive chat (default)")
	flag.Usage = func() {
		fmt.Fprintln(flag.CommandLine.Output(), "Usage: advent-agent [flags] [resume ID | rag index | rag status]\nFlags precede subcommands.")
		flag.PrintDefaults()
	}
	flag.Parse()
	c, err := agent.LoadConfig(*path)
	if err != nil {
		return err
	}
	ctx, done := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer done()
	args := flag.Args()
	if len(args) > 0 && args[0] == "rag" {
		if len(args) != 2 || (args[1] != "index" && args[1] != "status") {
			return fmt.Errorf("usage: advent-agent rag index|status")
		}
		root, e := agent.ProjectRoot(*project)
		if e != nil {
			return e
		}
		out, e := agent.IndexCommand(ctx, c, root, args[1], "")
		if e != nil {
			return e
		}
		fmt.Println(string(out))
		return nil
	}
	if len(args) > 0 && (len(args) != 2 || args[0] != "resume") {
		return fmt.Errorf("usage: advent-agent [flags] [resume ID | rag index | rag status]")
	}
	if err = ensure(ctx, c, *path); err != nil {
		return err
	}
	var s *agent.Session
	if len(args) == 2 {
		s, err = call(ctx, c, "get", agent.Request{ID: args[1]})
	} else {
		root, e := agent.ProjectRoot(*project)
		if e != nil {
			return e
		}
		s, err = call(ctx, c, "create", agent.Request{Project: root})
	}
	if err != nil {
		return err
	}
	fmt.Println("Session:", s.ID)
	fmt.Println("Project:", s.Root)
	ask := func(line string) error {
		action := "ask"
		question := strings.TrimSpace(line)
		if question == "/retry" {
			action = "retry"
			question = ""
		} else if question == "/discard" {
			action = "discard"
			question = ""
		} else if strings.HasPrefix(question, "/help ") {
			question = strings.TrimSpace(strings.TrimPrefix(question, "/help "))
		} else if strings.HasPrefix(question, "/") {
			return fmt.Errorf("unknown command; use /help")
		}
		next, e := call(ctx, c, action, agent.Request{ID: s.ID, Question: question})
		if next != nil {
			s = next
		}
		if next != nil && len(s.Operations) > 0 && action != "discard" {
			for _, change := range s.Operations[len(s.Operations)-1].Changes {
				fmt.Printf("\nChange %s: %s\n%s", change.Status, change.Path, change.Diff)
			}
		}
		if e != nil {
			return e
		}
		if action == "discard" {
			fmt.Println("Discarded.")
			return nil
		}
		op := s.Operations[len(s.Operations)-1]
		if op.Warning != "" {
			fmt.Fprintln(os.Stderr, op.Warning)
		}
		fmt.Println(agent.Render(*op.Answer, op.Evidence))
		return nil
	}
	if *prompt != "" {
		return ask(*prompt)
	}
	fmt.Println("Type /help for commands. Ctrl-C cancels the current request and exits.")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 4096), 1<<20)
	for {
		fmt.Print("you> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		line := strings.TrimSpace(scanner.Text())
		switch line {
		case "":
			continue
		case "/exit":
			return nil
		case "/help":
			fmt.Println(help)
			continue
		case "/summary":
			fmt.Println(s.Summary)
			continue
		case "/stats":
			var tokens, requests int
			for _, op := range s.Operations {
				for _, c := range op.Calls {
					tokens += c.Usage.Total
					requests++
				}
			}
			fmt.Printf("Model: %s; requests: %d; provider tokens: %d\n", s.Model, requests, tokens)
			continue
		}
		if e := ask(line); e != nil {
			fmt.Fprintln(os.Stderr, "error:", e)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}
func call(ctx context.Context, c agent.Config, action string, input agent.Request) (*agent.Session, error) {
	b, _ := json.Marshal(input)
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://unix/"+action, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	res, err := agent.HTTPClient(c).Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return nil, fmt.Errorf("daemon HTTP %d", res.StatusCode)
	}
	var out agent.Response
	if err = json.NewDecoder(res.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Error != "" {
		return out.Session, fmt.Errorf("%s", out.Error)
	}
	if out.Session == nil {
		return nil, fmt.Errorf("daemon response missing session")
	}
	return out.Session, nil
}
func ensure(ctx context.Context, c agent.Config, path string) error {
	probe, cancel := context.WithTimeout(ctx, time.Second)
	err := agent.Health(probe, c)
	cancel()
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "incompatible daemon") {
		return fmt.Errorf("incompatible daemon; stop it with advent-agentd stop before restarting")
	}
	if err = os.MkdirAll(c.State, 0700); err != nil {
		return err
	}
	log, err := os.OpenFile(filepath.Join(c.State, "daemon.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer log.Close()
	exe, _ := os.Executable()
	args := []string{}
	if path != "" {
		absolute, e := filepath.Abs(path)
		if e != nil {
			return e
		}
		args = append(args, "--config", absolute)
	}
	cmd := exec.Command(filepath.Join(filepath.Dir(exe), "advent-agentd"), args...)
	cmd.Stdout = log
	cmd.Stderr = log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err = cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	for i := 0; i < 50; i++ {
		probe, cancel = context.WithTimeout(ctx, 100*time.Millisecond)
		err = agent.Health(probe, c)
		cancel()
		if err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
	}
	return fmt.Errorf("daemon startup failed; inspect %s", filepath.Join(c.State, "daemon.log"))
}
