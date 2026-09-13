package main

import (
	"bufio"
	agent "deepseek-agent"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type options struct {
	prompt, system, systemFile, formatFile, length, stop, approach, roles, model, reasoning string
	temperature                                                                             float64
	temperatureSet, stats, debug                                                            bool
}

func main() {
	cfg, err := agent.LoadConfig()
	if err != nil {
		fatal(err)
	}
	api := agent.NewAPIClient(cfg.Daemon.SocketPath, 10*time.Second)
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "delete" {
		if len(args) != 2 {
			fatal(errors.New("usage: deepseek-agent delete SESSION_ID"))
		}
		fatalIf(ensureDaemon(api, cfg))
		fatalIf(api.Delete(args[1]))
		return
	}
	if len(args) > 0 && args[0] == "resume" {
		resume(api, cfg, args[1:])
		return
	}
	runNew(api, cfg, args)
}
func parseOptions(args []string, defaults agent.Settings) (options, agent.Settings) {
	var o options
	fs := flag.NewFlagSet("deepseek-agent", flag.ExitOnError)
	fs.StringVar(&o.prompt, "prompt", "", "one-shot prompt")
	fs.StringVar(&o.prompt, "p", "", "one-shot prompt")
	fs.StringVar(&o.system, "system-prompt", "", "session system prompt")
	fs.StringVar(&o.systemFile, "system-prompt-file", "", "file containing session system prompt")
	fs.StringVar(&o.formatFile, "format", "", "answer format file")
	fs.StringVar(&o.formatFile, "f", "", "answer format file")
	fs.StringVar(&o.length, "length", "", "answer length")
	fs.StringVar(&o.length, "l", "", "answer length")
	fs.StringVar(&o.stop, "stop", "", "clarification stop sequence")
	fs.StringVar(&o.stop, "s", "", "clarification stop sequence")
	fs.StringVar(&o.approach, "approach", defaults.Approach, "prompt approach")
	fs.StringVar(&o.approach, "a", defaults.Approach, "prompt approach")
	fs.StringVar(&o.roles, "roles", strings.Join(defaults.Roles, ","), "comma-separated roles")
	fs.StringVar(&o.model, "model", defaults.Model, "model")
	fs.StringVar(&o.model, "m", defaults.Model, "model")
	fs.StringVar(&o.reasoning, "reasoning", defaults.Reasoning, "reasoning effort")
	fs.StringVar(&o.reasoning, "r", defaults.Reasoning, "reasoning effort")
	fs.Func("temperature", "temperature 0..2", func(v string) error {
		n, e := strconv.ParseFloat(v, 64)
		if e == nil {
			o.temperature = n
			o.temperatureSet = true
		}
		return e
	})
	fs.Func("t", "temperature 0..2", func(v string) error {
		n, e := strconv.ParseFloat(v, 64)
		if e == nil {
			o.temperature = n
			o.temperatureSet = true
		}
		return e
	})
	fs.BoolVar(&o.stats, "stats", false, "show operation statistics")
	fs.BoolVar(&o.debug, "debug", false, "return masked HTTP diagnostics")
	fs.BoolVar(&o.debug, "d", false, "return masked HTTP diagnostics")
	_ = fs.Parse(args)
	s := defaults
	s.Model = o.model
	s.Reasoning = o.reasoning
	s.Approach = o.approach
	s.Length = o.length
	s.Stop = o.stop
	s.Stats = o.stats
	s.Debug = o.debug
	if o.temperatureSet {
		s.Temperature = &o.temperature
	}
	if o.roles != "" {
		s.Roles = splitRoles(o.roles)
	}
	if o.formatFile != "" {
		s.Format = readFile(o.formatFile)
	}
	if err := agent.ValidateSettings(s); err != nil {
		fatal(err)
	}
	return o, s
}
func runNew(api *agent.APIClient, cfg agent.Config, args []string) {
	o, settings := parseOptions(args, agent.DefaultSettings(cfg))
	if o.prompt == "" {
		if info, err := os.Stdin.Stat(); err == nil && info.Mode()&os.ModeCharDevice == 0 {
			data, err := io.ReadAll(os.Stdin)
			fatalIf(err)
			o.prompt = string(data)
		}
	}
	if o.system != "" && o.systemFile != "" {
		fatal(errors.New("--system-prompt and --system-prompt-file are mutually exclusive"))
	}
	system := cfg.Agent.SystemPrompt
	if o.system != "" {
		system = o.system
	}
	if o.systemFile != "" {
		system = readFile(o.systemFile)
	}
	fatalIf(ensureDaemon(api, cfg))
	session, err := api.Create(agent.CreateSessionRequest{SystemPrompt: system, Settings: &settings})
	fatalIf(err)
	runSession(api, cfg, session.ID, o.prompt, settings)
}
func resume(api *agent.APIClient, cfg agent.Config, args []string) {
	fatalIf(ensureDaemon(api, cfg))
	id := ""
	optionArgs := []string{}
	if len(args) > 0 {
		id = args[0]
		optionArgs = args[1:]
	}
	if id == "" {
		sessions, err := api.List()
		fatalIf(err)
		if len(sessions) == 0 {
			fatal(errors.New("no saved sessions"))
		}
		for i, s := range sessions {
			state := "idle"
			if s.Operation != nil {
				state = s.Operation.State
			}
			fmt.Printf("%d) %s  %-14s %s  %s\n", i+1, s.ID, state, s.UpdatedAt.Local().Format("2006-01-02 15:04"), s.Preview)
		}
		fmt.Print("Session: ")
		line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || n < 1 || n > len(sessions) {
			fatal(errors.New("invalid selection"))
		}
		id = sessions[n-1].ID
	}
	session, err := api.Get(id)
	fatalIf(err)
	_, settings := parseOptions(optionArgs, session.Settings)
	runSession(api, cfg, id, "", settings)
}

func ensureDaemon(api *agent.APIClient, cfg agent.Config) error {
	if _, err := api.List(); err == nil {
		return nil
	} else if !errors.Is(err, agent.ErrDaemonUnavailable) {
		return err
	}
	if !cfg.Client.Autostart {
		return fmt.Errorf("%w; start it with ./deepseek-agentd", agent.ErrDaemonUnavailable)
	}
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		return errors.New("cannot auto-start deepseek-agentd: DEEPSEEK_API_KEY is not exported in this shell")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.Daemon.SocketPath), 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(cfg.Daemon.SocketPath+".start.lock", os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return fmt.Errorf("open daemon startup lock: %w", err)
	}
	defer lock.Close()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock daemon startup: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if _, err := api.List(); err == nil {
		return nil
	}
	daemonPath, err := findDaemon()
	if err != nil {
		return err
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer devNull.Close()
	cmd := exec.Command(daemonPath)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = devNull, devNull, devNull
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", daemonPath, err)
	}
	_ = cmd.Process.Release()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := api.List(); err == nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return errors.New("deepseek-agentd did not become ready; run ./deepseek-agentd manually to see its startup error")
}

func findDaemon() (string, error) {
	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), "deepseek-agentd")
		if info, statErr := os.Stat(candidate); statErr == nil && info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			return candidate, nil
		}
	}
	if path, err := exec.LookPath("deepseek-agentd"); err == nil {
		return path, nil
	}
	return "", errors.New("cannot auto-start deepseek-agentd: executable not found next to deepseek-agent or in PATH")
}

func runSession(api *agent.APIClient, cfg agent.Config, id, initial string, settings agent.Settings) {
	lease, err := api.Attach(id)
	fatalIf(err)
	defer api.Detach(id, lease)
	fmt.Fprintf(os.Stderr, "Session: %s\n", id)
	current, err := api.Get(id)
	fatalIf(err)
	if current.Operation != nil {
		switch current.Operation.State {
		case "running":
			wait(api, cfg, id, settings)
		case "awaiting_input":
			if len(current.Messages) > 0 {
				fmt.Println("Agent:", current.Messages[len(current.Messages)-1].Content)
			}
		case "failed", "interrupted":
			fmt.Fprintln(os.Stderr, "agent:", current.Operation.State+":", current.Operation.Error)
		case "completed":
			if len(current.Messages) > 0 {
				fmt.Println("Last answer:", current.Messages[len(current.Messages)-1].Content)
			}
		}
	}
	done := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Duration(cfg.Daemon.LeaseTimeout) / 3)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				_ = api.Heartbeat(id, lease)
			}
		}
	}()
	defer close(done)
	if initial != "" {
		fatalIf(api.Submit(id, lease, initial, settings))
		if wait(api, cfg, id, settings) != "awaiting_input" {
			return
		}
	}
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("You: ")
		line, e := reader.ReadString('\n')
		if e != nil && len(line) == 0 {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "/") {
			if handleCommand(api, cfg, id, lease, line, &settings) {
				return
			}
			continue
		}
		fatalIf(api.Submit(id, lease, line, settings))
		wait(api, cfg, id, settings)
	}
}
func wait(api *agent.APIClient, cfg agent.Config, id string, settings agent.Settings) string {
	for {
		time.Sleep(time.Duration(cfg.Client.PollInterval))
		s, err := api.Get(id)
		fatalIf(err)
		if s.Operation == nil {
			continue
		}
		switch s.Operation.State {
		case "running":
			continue
		case "awaiting_input", "completed":
			if len(s.Messages) > 0 {
				fmt.Println("Agent:", s.Messages[len(s.Messages)-1].Content)
			}
			printExtras(s.Operation)
			return s.Operation.State
		case "failed", "interrupted":
			fmt.Fprintln(os.Stderr, "agent:", s.Operation.State+":", s.Operation.Error)
			return s.Operation.State
		}
	}
}
func handleCommand(api *agent.APIClient, cfg agent.Config, id, lease, line string, s *agent.Settings) bool {
	parts := strings.SplitN(strings.TrimPrefix(line, "/"), " ", 2)
	cmd := parts[0]
	value := ""
	if len(parts) > 1 {
		value = strings.TrimSpace(parts[1])
	}
	switch cmd {
	case "exit":
		return true
	case "help":
		fmt.Println("/settings /model /reasoning /temperature /approach /roles /format /length /stop /unset /stats /debug /retry /discard /exit")
	case "settings":
		data, _ := json.MarshalIndent(s, "", "  ")
		fmt.Println(string(data))
	case "model":
		s.Model = value
		validate(s)
	case "reasoning":
		s.Reasoning = value
		validate(s)
	case "temperature":
		n, e := strconv.ParseFloat(value, 64)
		fatalIf(e)
		s.Temperature = &n
		validate(s)
	case "approach":
		s.Approach = value
		validate(s)
	case "roles":
		s.Roles = splitRoles(value)
		validate(s)
	case "format":
		s.Format = readFile(value)
	case "length":
		s.Length = value
	case "stop":
		if value == "" {
			fmt.Fprintln(os.Stderr, "usage: /stop SEQUENCE")
		} else {
			s.Stop = value
		}
	case "unset":
		unset(s, value)
		validate(s)
	case "stats":
		s.Stats = value != "off"
	case "debug":
		s.Debug = value != "off"
	case "retry":
		fatalIf(api.Retry(id, lease))
		wait(api, cfg, id, *s)
	case "discard":
		fatalIf(api.Discard(id, lease))
	default:
		fmt.Fprintln(os.Stderr, "unknown command; use /help")
	}
	return false
}
func unset(s *agent.Settings, v string) {
	switch v {
	case "temperature":
		s.Temperature = nil
	case "format":
		s.Format = ""
	case "length":
		s.Length = ""
	case "stop":
		s.Stop = ""
	case "roles":
		s.Roles = []string{"Business analyst", "Engineer", "Critic"}
	case "stats":
		s.Stats = false
	case "debug":
		s.Debug = false
	default:
		fmt.Fprintln(os.Stderr, "unknown setting")
	}
}
func printExtras(o *agent.Operation) {
	if o.Settings.Stats {
		fmt.Fprintf(os.Stderr, "Stats: requests=%d duration=%s input=%d output=%d total=%d estimated_cost=$%.8f USD\n", o.Metrics.Requests, o.Metrics.Duration.Round(time.Millisecond), o.Metrics.Usage.PromptTokens, o.Metrics.Usage.CompletionTokens, o.Metrics.Usage.TotalTokens, o.Metrics.CostUSD)
	}
	if o.Settings.Debug && o.Diagnostics != "" {
		fmt.Fprint(os.Stderr, o.Diagnostics)
	}
}
func splitRoles(v string) []string {
	p := strings.Split(v, ",")
	out := []string{}
	for _, x := range p {
		if x = strings.TrimSpace(x); x != "" {
			out = append(out, x)
		}
	}
	return out
}
func validate(s *agent.Settings) {
	if err := agent.ValidateSettings(*s); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
	}
}
func readFile(path string) string {
	data, err := os.ReadFile(path)
	fatalIf(err)
	if strings.TrimSpace(string(data)) == "" {
		fatal(errors.New("file is empty"))
	}
	return string(data)
}
func fatalIf(err error) {
	if err != nil {
		fatal(err)
	}
}
func fatal(err error) {
	if errors.Is(err, io.EOF) {
		return
	}
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
