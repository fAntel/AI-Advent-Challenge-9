package agent

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("session not found")
var ErrConflict = errors.New("session is busy or attached")

type runtimeSession struct {
	session    *Session
	lease      string
	leaseUntil time.Time
	cancel     context.CancelFunc
	timer      *time.Timer
}
type Agent struct {
	mu                                       sync.Mutex
	store                                    Store
	completer                                Completer
	logger                                   *DebugLogger
	defaults                                 Settings
	contextWindowTokens                      int
	leaseTimeout, evictAfter, requestTimeout time.Duration
	sessions                                 map[string]*runtimeSession
	closed                                   chan struct{}
}

func NewAgent(cfg Config, completer Completer, logger *DebugLogger) (*Agent, error) {
	a := &Agent{store: Store{Dir: cfg.Daemon.SessionDir}, completer: completer, logger: logger, defaults: DefaultSettings(cfg), contextWindowTokens: cfg.Agent.ContextWindowTokens, leaseTimeout: time.Duration(cfg.Daemon.LeaseTimeout), evictAfter: time.Duration(cfg.Daemon.EvictAfter), requestTimeout: time.Duration(cfg.Daemon.RequestTimeout), sessions: map[string]*runtimeSession{}, closed: make(chan struct{})}
	if err := a.store.Recover(); err != nil {
		return nil, err
	}
	go a.expireLeases()
	return a, nil
}
func (a *Agent) Close() {
	close(a.closed)
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.sessions {
		if r.cancel != nil {
			r.cancel()
		}
		_ = a.store.Save(r.session)
	}
}

func randomID() string { b := make([]byte, 16); _, _ = rand.Read(b); return hex.EncodeToString(b) }
func (a *Agent) Create(req CreateSessionRequest) (*Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now().UTC()
	settings := a.defaults
	if req.Settings != nil {
		settings = *req.Settings
	}
	if err := ValidateSettings(settings); err != nil {
		return nil, err
	}
	s := &Session{ID: randomID(), CreatedAt: now, UpdatedAt: now, SystemPrompt: req.SystemPrompt, Settings: settings, Messages: []Message{}, ContextWindowTokens: a.contextWindowTokens, TokenAccountingComplete: true}
	if err := a.store.Save(s); err != nil {
		return nil, err
	}
	a.sessions[s.ID] = &runtimeSession{session: s}
	a.logger.Log("session.create", s.ID, "", "session created")
	return cloneSession(s), nil
}
func (a *Agent) loadLocked(id string) (*runtimeSession, error) {
	if r := a.sessions[id]; r != nil {
		return r, nil
	}
	s, err := a.store.Load(id)
	if err != nil {
		return nil, ErrNotFound
	}
	if s.ContextWindowTokens == 0 {
		s.ContextWindowTokens = a.contextWindowTokens
	}
	r := &runtimeSession{session: s}
	a.sessions[id] = r
	a.logger.Log("session.load", id, "", "session loaded from disk")
	return r, nil
}
func (a *Agent) List() ([]Session, error) { return a.store.List() }
func (a *Agent) Get(id string) (*Session, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, err := a.loadLocked(id)
	if err != nil {
		return nil, err
	}
	a.scheduleEvictLocked(id, r)
	return cloneSession(r.session), nil
}
func (a *Agent) Attach(id string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, err := a.loadLocked(id)
	if err != nil {
		return "", err
	}
	if r.lease != "" && time.Now().Before(r.leaseUntil) {
		return "", ErrConflict
	}
	token := randomID()
	r.lease = token
	r.leaseUntil = time.Now().Add(a.leaseTimeout)
	if r.timer != nil {
		r.timer.Stop()
	}
	r.session.Attached = true
	a.logger.Log("session.attach", id, "", "CLI attached")
	return token, nil
}
func (a *Agent) Heartbeat(id, token string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, err := a.loadLocked(id)
	if err != nil {
		return err
	}
	if r.lease != token || token == "" {
		return ErrConflict
	}
	r.leaseUntil = time.Now().Add(a.leaseTimeout)
	return nil
}
func (a *Agent) Detach(id, token string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, err := a.loadLocked(id)
	if err != nil {
		return err
	}
	if r.lease != token {
		return ErrConflict
	}
	r.lease = ""
	r.session.Attached = false
	_ = a.store.Save(r.session)
	a.logger.Log("session.detach", id, "", "CLI disconnected; session saved")
	a.scheduleEvictLocked(id, r)
	return nil
}
func (a *Agent) Delete(id string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, err := a.loadLocked(id)
	if err != nil {
		return err
	}
	if r.lease != "" || (r.session.Operation != nil && r.session.Operation.State == "running") {
		return ErrConflict
	}
	delete(a.sessions, id)
	a.logger.Log("session.delete", id, "", "session deleted")
	return a.store.Delete(id)
}

func (a *Agent) Submit(id, token string, req MessageRequest) error {
	if strings.TrimSpace(req.Content) == "" {
		return errors.New("message is empty")
	}
	a.mu.Lock()
	r, err := a.loadLocked(id)
	if err != nil {
		a.mu.Unlock()
		return err
	}
	if r.lease != token {
		a.mu.Unlock()
		return ErrConflict
	}
	if r.session.Operation != nil && r.session.Operation.State == "running" {
		a.mu.Unlock()
		return ErrConflict
	}
	settings := r.session.Settings
	if req.Settings != nil {
		settings = *req.Settings
		r.session.Settings = settings
	}
	if err := ValidateSettings(settings); err != nil {
		a.mu.Unlock()
		return err
	}
	continuing := r.session.Operation != nil && r.session.Operation.State == "awaiting_input"
	if !continuing {
		r.session.Operation = &Operation{ID: randomID(), State: "running", Settings: settings, StartedAt: time.Now().UTC(), BaseCount: len(r.session.Messages)}
	} else {
		r.session.Operation.State = "running"
	}
	r.session.Messages = append(r.session.Messages, Message{Role: "user", Content: req.Content})
	r.session.UpdatedAt = time.Now().UTC()
	_ = a.store.Save(r.session)
	opID := r.session.Operation.ID
	ctx, cancel := context.WithTimeout(context.Background(), a.requestTimeout)
	r.cancel = cancel
	a.logger.Log("workflow.start", id, opID, "message accepted; background request started")
	a.mu.Unlock()
	go a.run(ctx, id, opID)
	return nil
}

func (a *Agent) Retry(id, token string) error {
	a.mu.Lock()
	r, err := a.loadLocked(id)
	if err != nil {
		a.mu.Unlock()
		return err
	}
	if r.lease != token || r.session.Operation == nil || (r.session.Operation.State != "failed" && r.session.Operation.State != "interrupted") {
		a.mu.Unlock()
		return ErrConflict
	}
	r.session.Operation.State = "running"
	r.session.Operation.Error = ""
	ctx, cancel := context.WithTimeout(context.Background(), a.requestTimeout)
	r.cancel = cancel
	op := r.session.Operation.ID
	_ = a.store.Save(r.session)
	a.mu.Unlock()
	go a.run(ctx, id, op)
	return nil
}
func (a *Agent) Discard(id, token string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	r, err := a.loadLocked(id)
	if err != nil {
		return err
	}
	if r.lease != token || r.session.Operation == nil || (r.session.Operation.State != "failed" && r.session.Operation.State != "interrupted") {
		return ErrConflict
	}
	r.session.Messages = append([]Message(nil), r.session.Messages[:r.session.Operation.BaseCount]...)
	r.session.Operation = nil
	r.session.UpdatedAt = time.Now().UTC()
	return a.store.Save(r.session)
}

func (a *Agent) run(ctx context.Context, id, opID string) {
	ctx = context.WithValue(ctx, sessionContextKey, id)
	ctx = context.WithValue(ctx, operationContextKey, opID)
	a.mu.Lock()
	r := a.sessions[id]
	if r == nil || r.session.Operation == nil || r.session.Operation.ID != opID {
		a.mu.Unlock()
		return
	}
	s := cloneSession(r.session)
	a.mu.Unlock()
	op := s.Operation
	messages := requestMessages(s.SystemPrompt, s.Messages, op.Settings, op.Settings.Stop != "" && !containsFold(s.Messages[len(s.Messages)-1].Content, op.Settings.Stop))
	completion, err := a.completer.Complete(ctx, messages, op.Settings)
	a.mu.Lock()
	defer a.mu.Unlock()
	r = a.sessions[id]
	if r == nil || r.session.Operation == nil || r.session.Operation.ID != opID {
		return
	}
	r.cancel = nil
	if err != nil {
		r.session.Operation.State = "failed"
		r.session.Operation.Error = a.logger.Redact(err.Error())
		if errors.Is(err, ErrContextWindowExceeded) {
			r.session.Operation.Error = "model context window exceeded; start a new session or shorten the dialog"
		}
		if op.Settings.Debug {
			r.session.Operation.Diagnostics = completion.Diagnostics
		}
		a.logger.Log("workflow.failed", id, opID, err.Error())
	} else {
		r.session.Messages = append(r.session.Messages, Message{Role: "assistant", Content: completion.Answer})
		recordCompletion(r.session, completion)
		a.logUsage(id, opID, completion)
		if op.Settings.Debug {
			r.session.Operation.Diagnostics += completion.Diagnostics
		}
		if completion.Call.FinishReason == "length" {
			r.session.Operation.State = "truncated"
			r.session.Operation.Warning = "model answer was truncated because it reached a token limit"
			a.logger.Log("answer.truncated", id, opID, r.session.Operation.Warning)
		} else if op.Settings.Stop != "" && !containsFold(s.Messages[len(s.Messages)-1].Content, op.Settings.Stop) {
			r.session.Operation.State = "awaiting_input"
			a.logger.Log("workflow.awaiting_input", id, opID, "clarification answer received")
		} else if op.Settings.Approach == "self-prompt" {
			finalSettings := op.Settings
			finalSettings.Approach = "none"
			finalSettings.Stop = ""
			generated := completion.Answer
			a.mu.Unlock()
			second, e := a.completer.Complete(ctx, requestMessages(s.SystemPrompt, []Message{{Role: "user", Content: generated}}, finalSettings, false), finalSettings)
			a.mu.Lock()
			r = a.sessions[id]
			if e != nil {
				r.session.Operation.State = "failed"
				r.session.Operation.Error = a.logger.Redact(e.Error())
			} else {
				r.session.Messages[len(r.session.Messages)-1].Content = second.Answer
				recordCompletion(r.session, second)
				a.logUsage(id, opID, second)
				if op.Settings.Debug {
					r.session.Operation.Diagnostics += second.Diagnostics
				}
				if second.Call.FinishReason == "length" {
					r.session.Operation.State = "truncated"
					r.session.Operation.Warning = "model answer was truncated because it reached a token limit"
				} else {
					r.session.Operation.State = "completed"
				}
			}
		} else {
			r.session.Operation.State = "completed"
			a.logger.Log("answer.received", id, opID, "answer received")
		}
	}
	now := time.Now().UTC()
	r.session.Operation.FinishedAt = &now
	r.session.UpdatedAt = now
	_ = a.store.Save(r.session)
	a.logger.Log("session.save", id, opID, "session saved")
	a.scheduleEvictLocked(id, r)
}

func requestMessages(system string, history []Message, s Settings, clarify bool) []Message {
	result := []Message{}
	parts := []string{}
	if system != "" {
		parts = append(parts, system)
	}
	if s.Format != "" {
		parts = append(parts, "Follow this answer format exactly:\n<answer-format>\n"+s.Format+"\n</answer-format>")
	}
	if s.Length != "" {
		parts = append(parts, "Target final-answer length: "+s.Length)
	}
	switch s.Approach {
	case "step-by-step":
		parts = append(parts, "Solve step by step and present the steps clearly.")
	case "self-prompt":
		parts = append(parts, "Return only a clear, self-contained solving prompt. Do not solve it.")
	case "multi-role":
		parts = append(parts, "Answer separately as each role: "+strings.Join(s.Roles, ", "))
	}
	if s.Stop != "" {
		parts = append(parts, "Ask exactly one concise clarifying question until the latest user message contains the stop sequence, case-insensitively: "+s.Stop)
		if clarify {
			parts = append(parts, "The sequence is absent. Ask one question only.")
		} else {
			parts = append(parts, "The sequence is present. Give the final answer now.")
		}
	}
	if len(parts) > 0 {
		result = append(result, Message{Role: "system", Content: strings.Join(parts, "\n\n")})
	}
	return append(result, history...)
}
func containsFold(v, sub string) bool {
	return strings.Contains(strings.ToLower(v), strings.ToLower(sub))
}
func addMetrics(dst *Metrics, m Metrics) {
	dst.Requests += m.Requests
	dst.Duration += m.Duration
	dst.CostUSD += m.CostUSD
	dst.Usage.PromptTokens += m.Usage.PromptTokens
	dst.Usage.PromptCacheHitTokens += m.Usage.PromptCacheHitTokens
	dst.Usage.PromptCacheMissTokens += m.Usage.PromptCacheMissTokens
	dst.Usage.CompletionTokens += m.Usage.CompletionTokens
	dst.Usage.ReasoningTokens += m.Usage.ReasoningTokens
	dst.Usage.TotalTokens += m.Usage.TotalTokens
}
func recordCompletion(s *Session, completion Completion) {
	call := completion.Call
	if call.StartedAt.IsZero() {
		call.Duration = completion.Metrics.Duration
		call.Usage = completion.Metrics.Usage
		call.CostUSD = completion.Metrics.CostUSD
	}
	s.Operation.Calls = append(s.Operation.Calls, call)
	s.Calls = append(s.Calls, call)
	addMetrics(&s.Operation.Metrics, completion.Metrics)
	addMetrics(&s.Metrics, completion.Metrics)
}
func (a *Agent) logUsage(sessionID, operationID string, completion Completion) {
	u := completion.Metrics.Usage
	a.logger.Log("usage.recorded", sessionID, operationID,
		fmt.Sprintf("input=%d cache_hit=%d cache_miss=%d output=%d total=%d cost_usd=%.8f finish_reason=%s",
			u.PromptTokens, u.PromptCacheHitTokens, u.PromptCacheMissTokens,
			u.CompletionTokens, u.TotalTokens, completion.Metrics.CostUSD, completion.Call.FinishReason))
}
func cloneSession(s *Session) *Session {
	c := *s
	c.Messages = append([]Message(nil), s.Messages...)
	c.Calls = append([]CallMetrics(nil), s.Calls...)
	if s.Operation != nil {
		o := *s.Operation
		o.Calls = append([]CallMetrics(nil), s.Operation.Calls...)
		c.Operation = &o
	}
	c.Attached = false
	if len(c.Messages) > 0 {
		c.Preview = c.Messages[0].Content
		if len(c.Preview) > 60 {
			c.Preview = c.Preview[:60]
		}
	}
	return &c
}
func (a *Agent) scheduleEvictLocked(id string, r *runtimeSession) {
	if r.lease != "" || (r.session.Operation != nil && r.session.Operation.State == "running") {
		return
	}
	if r.timer != nil {
		r.timer.Stop()
	}
	r.timer = time.AfterFunc(a.evictAfter, func() {
		a.mu.Lock()
		defer a.mu.Unlock()
		current := a.sessions[id]
		if current == r && r.lease == "" && (r.session.Operation == nil || r.session.Operation.State != "running") {
			_ = a.store.Save(r.session)
			delete(a.sessions, id)
			a.logger.Log("session.evict", id, "", "no connected clients; session saved and closed")
		}
	})
}
func (a *Agent) expireLeases() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-a.closed:
			return
		case now := <-ticker.C:
			a.mu.Lock()
			for id, r := range a.sessions {
				if r.lease != "" && now.After(r.leaseUntil) {
					r.lease = ""
					r.session.Attached = false
					a.logger.Log("lease.expire", id, "", "CLI lease expired")
					a.scheduleEvictLocked(id, r)
				}
			}
			a.mu.Unlock()
		}
	}
}
func (a *Agent) String() string { return fmt.Sprintf("Agent(%s)", a.store.Dir) }
