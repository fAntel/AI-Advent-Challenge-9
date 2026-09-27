package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// ScheduleJob is a durable instruction to invoke a registered MCP tool.
type ScheduleJob struct {
	Name           string `toml:"name" json:"name"`
	Pipeline       string `toml:"pipeline" json:"pipeline,omitempty"`
	Server         string `toml:"server" json:"server"`
	Tool           string `toml:"tool" json:"tool"`
	ArgumentsJSON  string `toml:"arguments_json" json:"arguments_json"`
	Every          string `toml:"every" json:"every,omitempty"`
	ScheduledEvery string `toml:"scheduled_every" json:"scheduled_every,omitempty"`
	At             string `toml:"at" json:"at,omitempty"`
	NextRun        string `toml:"next_run" json:"next_run,omitempty"`
	LastRun        string `toml:"last_run" json:"last_run,omitempty"`
	LastError      string `toml:"last_error" json:"last_error,omitempty"`
	LastResult     string `toml:"last_result" json:"last_result,omitempty"`
	Notify         bool   `toml:"notify" json:"notify"`
	Done           bool   `toml:"done" json:"done"`
}

type scheduleFile struct {
	Jobs []ScheduleJob `toml:"jobs"`
}

type Notice struct {
	Job  string    `json:"job"`
	At   time.Time `json:"at"`
	Text string    `json:"text"`
}

type EventHub struct {
	mu      sync.Mutex
	clients map[chan Notice]struct{}
	latest  *Notice
}

func NewEventHub() *EventHub { return &EventHub{clients: make(map[chan Notice]struct{})} }
func (h *EventHub) Subscribe() (<-chan Notice, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	c := make(chan Notice, 8)
	if h.latest != nil {
		c <- *h.latest
	}
	h.clients[c] = struct{}{}
	return c, func() { h.mu.Lock(); delete(h.clients, c); close(c); h.mu.Unlock() }
}
func (h *EventHub) Publish(n Notice) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.latest = &n
	for c := range h.clients {
		select {
		case c <- n:
		default:
		}
	}
}

type Scheduler struct {
	mu              sync.Mutex
	path            string
	jobs            map[string]ScheduleJob
	running         map[string]bool
	mcp             *MCPCatalog
	pipelines       *PipelineStore
	hub             *EventHub
	logger          func(string, string, error)
	stop            chan struct{}
	wake            chan struct{}
	now             func() time.Time
	timeout         time.Duration
	defaultInterval time.Duration
	ctx             context.Context
	cancel          context.CancelFunc
	done            chan struct{}
	wg              sync.WaitGroup
}

func NewScheduler(path string, catalog *MCPCatalog, hub *EventHub, timeout, defaultInterval time.Duration, logger func(string, string, error)) (*Scheduler, error) {
	ctx, cancel := context.WithCancel(context.Background())
	s := &Scheduler{path: path, jobs: map[string]ScheduleJob{}, running: map[string]bool{}, mcp: catalog, hub: hub, logger: logger, stop: make(chan struct{}), wake: make(chan struct{}, 1), now: time.Now, timeout: timeout, defaultInterval: defaultInterval, ctx: ctx, cancel: cancel, done: make(chan struct{})}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	if err == nil {
		dirty := false
		var file scheduleFile
		meta, e := toml.Decode(string(data), &file)
		if e != nil {
			return nil, fmt.Errorf("decode schedules: %w", e)
		}
		if u := meta.Undecoded(); len(u) > 0 {
			return nil, fmt.Errorf("unknown schedule key %s", u[0])
		}
		for _, j := range file.Jobs {
			validTarget := (j.Pipeline == "" && j.Server != "" && j.Tool != "") || (j.Pipeline != "" && j.Server == "" && j.Tool == "")
			if j.Name == "" || !validTarget || s.jobs[j.Name].Name != "" {
				return nil, errors.New("invalid or duplicate schedule job")
			}
			if j.Every != "" {
				d, e := s.interval(j.Every)
				if e != nil || d < time.Second {
					return nil, fmt.Errorf("schedule %q has invalid interval", j.Name)
				}
				if j.ScheduledEvery != d.String() {
					j.NextRun = s.now().Add(d).UTC().Format(time.RFC3339Nano)
					j.ScheduledEvery = d.String()
					dirty = true
				}
			}
			if j.NextRun != "" {
				if _, e := time.Parse(time.RFC3339Nano, j.NextRun); e != nil {
					return nil, e
				}
			}
			s.jobs[j.Name] = j
			if j.Notify && j.LastRun != "" && j.LastResult != "" {
				at, _ := time.Parse(time.RFC3339Nano, j.LastRun)
				if s.hub.latest == nil || at.After(s.hub.latest.At) {
					s.hub.latest = &Notice{Job: j.Name, At: at, Text: j.LastResult}
				}
			}
		}
		if dirty {
			if e := s.saveLocked(); e != nil {
				return nil, e
			}
		}
	}
	return s, nil
}

func (s *Scheduler) SetPipelines(p *PipelineStore) { s.pipelines = p }

func (s *Scheduler) UsesPipeline(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, j := range s.jobs {
		if j.Pipeline == name {
			return true
		}
	}
	return false
}

// Reload applies manual schedules.toml edits. Changed intervals start a fresh
// period from reload time, and the persisted next_run is updated immediately.
func (s *Scheduler) Reload() ([]ScheduleJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.running) > 0 {
		return nil, errors.New("wait for running scheduled calls before reloading")
	}
	if _, err := os.Stat(s.path); err != nil {
		return nil, err
	}
	fresh, err := NewScheduler(s.path, s.mcp, NewEventHub(), s.timeout, s.defaultInterval, s.logger)
	if err != nil {
		return nil, err
	}
	fresh.cancel()
	s.jobs = fresh.jobs
	select {
	case s.wake <- struct{}{}:
	default:
	}
	out := make([]ScheduleJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
func (s *Scheduler) interval(value string) (time.Duration, error) {
	if value == "default" {
		return s.defaultInterval, nil
	}
	return time.ParseDuration(value)
}

func (s *Scheduler) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}
	names := make([]string, 0, len(s.jobs))
	for name := range s.jobs {
		names = append(names, name)
	}
	sort.Strings(names)
	var b strings.Builder
	quote := func(value string) string { data, _ := json.Marshal(value); return string(data) }
	for _, name := range names {
		j := s.jobs[name]
		b.WriteString("[[jobs]]\n")
		for _, p := range [][2]string{{"name", j.Name}, {"pipeline", j.Pipeline}, {"server", j.Server}, {"tool", j.Tool}, {"arguments_json", j.ArgumentsJSON}, {"every", j.Every}, {"scheduled_every", j.ScheduledEvery}, {"at", j.At}, {"next_run", j.NextRun}, {"last_run", j.LastRun}, {"last_error", j.LastError}, {"last_result", j.LastResult}} {
			fmt.Fprintf(&b, "%s = %s\n", p[0], quote(p[1]))
		}
		fmt.Fprintf(&b, "notify = %t\ndone = %t\n\n", j.Notify, j.Done)
	}
	f, err := os.CreateTemp(filepath.Dir(s.path), ".schedules-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.WriteString(b.String())
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), s.path)
}

func (s *Scheduler) List() []ScheduleJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ScheduleJob, 0, len(s.jobs))
	for _, j := range s.jobs {
		out = append(out, j)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (s *Scheduler) Add(ctx context.Context, j ScheduleJob) error {
	if !mcpName.MatchString(j.Name) || (j.Pipeline != "" && !mcpName.MatchString(j.Pipeline)) || (j.Pipeline == "" && (!mcpName.MatchString(j.Server) || !mcpName.MatchString(j.Tool))) {
		return errors.New("invalid job, server, or tool name")
	}
	if !((j.Pipeline == "" && j.Server != "" && j.Tool != "") || (j.Pipeline != "" && j.Server == "" && j.Tool == "")) {
		return errors.New("choose a pipeline or a server and tool")
	}
	if (j.Every == "") == (j.At == "") {
		return errors.New("choose exactly one of every or at")
	}
	var next time.Time
	if j.Every != "" {
		d, e := s.interval(j.Every)
		if e != nil || d < time.Second {
			return errors.New("every must be at least one second")
		}
		next = s.now().Add(d)
	} else {
		var e error
		next, e = time.Parse(time.RFC3339, j.At)
		if e != nil {
			return e
		}
		if !next.After(s.now()) {
			return errors.New("at must be in the future")
		}
	}
	if j.ArgumentsJSON == "" {
		j.ArgumentsJSON = "{}"
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(j.ArgumentsJSON), &args); err != nil || args == nil {
		return errors.New("arguments must be a JSON object")
	}
	if j.Pipeline != "" {
		if s.pipelines == nil {
			return errors.New("pipelines unavailable")
		}
		if len(args) != 0 {
			return errors.New("pipeline schedule does not accept tool arguments")
		}
		if err := s.pipelines.Validate(ctx, j.Pipeline); err != nil {
			return err
		}
	} else {
		tools, err := s.mcp.Tools(ctx, j.Server, false)
		if err != nil {
			return err
		}
		found := false
		for _, tool := range tools {
			if tool.Name == j.Tool {
				found = true
				if err := validateMCPArguments(tool.InputSchema, args); err != nil {
					return err
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("MCP tool %s/%s not found", j.Server, j.Tool)
		}
	}
	j.LastRun, j.LastResult, j.LastError = "", "", ""
	j.Done = false
	j.NextRun = next.UTC().Format(time.RFC3339Nano)
	if j.Every != "" {
		d, _ := s.interval(j.Every)
		j.ScheduledEvery = d.String()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.jobs[j.Name]; ok {
		return fmt.Errorf("schedule %q already exists", j.Name)
	}
	s.jobs[j.Name] = j
	if err := s.saveLocked(); err != nil {
		delete(s.jobs, j.Name)
		return err
	}
	select {
	case s.wake <- struct{}{}:
	default:
	}
	return nil
}
func (s *Scheduler) Remove(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	j, ok := s.jobs[name]
	if !ok {
		return os.ErrNotExist
	}
	delete(s.jobs, name)
	if err := s.saveLocked(); err != nil {
		s.jobs[name] = j
		return err
	}
	return nil
}
func (s *Scheduler) Start() { go s.loop() }
func (s *Scheduler) Close() { s.cancel(); close(s.stop); <-s.done; s.wg.Wait() }
func (s *Scheduler) loop() {
	defer close(s.done)
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stop:
			return
		case <-s.wake:
		case <-ticker.C:
		}
		s.runDue()
	}
}
func (s *Scheduler) runDue() {
	now := s.now()
	for _, j := range s.List() {
		if j.Done || j.NextRun == "" {
			continue
		}
		next, e := time.Parse(time.RFC3339Nano, j.NextRun)
		if e != nil || next.After(now) {
			continue
		}
		s.Run(j.Name)
	}
}
func (s *Scheduler) Run(name string) error {
	s.mu.Lock()
	if s.ctx.Err() != nil {
		s.mu.Unlock()
		return context.Canceled
	}
	j, ok := s.jobs[name]
	if !ok {
		s.mu.Unlock()
		return os.ErrNotExist
	}
	if s.running[name] {
		s.mu.Unlock()
		return fmt.Errorf("schedule %q is already running", name)
	}
	s.running[name] = true
	s.wg.Add(1)
	defer func() { s.mu.Lock(); delete(s.running, name); s.mu.Unlock() }()
	defer s.wg.Done()
	previous := j
	if j.Every != "" {
		d, _ := s.interval(j.Every)
		j.NextRun = s.now().Add(d).UTC().Format(time.RFC3339Nano)
		j.ScheduledEvery = d.String()
	} else {
		j.Done = true
		j.NextRun = ""
	}
	s.jobs[name] = j
	err := s.saveLocked()
	if err != nil {
		s.jobs[name] = previous
	}
	s.mu.Unlock()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()
	var args map[string]any
	_ = json.Unmarshal([]byte(j.ArgumentsJSON), &args)
	var result *mcp.CallToolResult
	var callErr error
	if j.Pipeline != "" {
		if s.pipelines == nil {
			callErr = errors.New("pipelines unavailable")
		} else {
			run, err := s.pipelines.Run(ctx, j.Pipeline)
			callErr = err
			result = &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: run.Result}}}
		}
	} else {
		result, callErr = s.mcp.Call(ctx, j.Server, j.Tool, args)
	}
	message := ""
	if callErr == nil && result == nil {
		callErr = errors.New("MCP tool returned no result")
	}
	if callErr == nil {
		for _, c := range result.Content {
			if t, ok := c.(*mcp.TextContent); ok {
				message = t.Text
				break
			}
		}
		if message == "" {
			data, _ := json.Marshal(result.StructuredContent)
			message = string(data)
		}
	}
	if len(message) > 2048 {
		message = message[:2048]
	}
	if callErr == nil && result.IsError {
		callErr = fmt.Errorf("MCP tool: %s", message)
	}
	finished := s.now().UTC()
	s.mu.Lock()
	current, still := s.jobs[name]
	var saveErr error
	if still {
		current.LastRun = finished.Format(time.RFC3339Nano)
		current.LastResult = message
		current.LastError = ""
		if callErr != nil {
			current.LastError = callErr.Error()
		}
		s.jobs[name] = current
		saveErr = s.saveLocked()
	}
	s.mu.Unlock()
	if saveErr != nil && callErr == nil {
		callErr = fmt.Errorf("persist schedule result: %w", saveErr)
	}
	if s.logger != nil {
		s.logger(j.Name, message, callErr)
	}
	if still && j.Notify && callErr == nil {
		s.hub.Publish(Notice{Job: j.Name, At: finished, Text: message})
	}
	return callErr
}
