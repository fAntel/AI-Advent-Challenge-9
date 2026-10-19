package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

type SupportJob struct {
	ID          string    `json:"id"`
	ChannelID   string    `json:"channel_id"`
	RootID      string    `json:"root_id"`
	Question    string    `json:"question"`
	Scope       string    `json:"scope"`
	Status      string    `json:"status"`
	Attempts    int       `json:"attempts"`
	NextAttempt int64     `json:"next_attempt"`
	Operation   Operation `json:"operation"`
	Text        string    `json:"text,omitempty"`
	Publication string    `json:"publication"`
	PostID      string    `json:"post_id,omitempty"`
	Error       string    `json:"error,omitempty"`
}
type SupportState struct {
	Version int                    `json:"version"`
	URL     string                 `json:"url"`
	BotID   string                 `json:"bot_id"`
	Cursors map[string]int64       `json:"cursors"`
	Seen    map[string]bool        `json:"seen"`
	Threads map[string]string      `json:"threads"`
	Jobs    []*SupportJob          `json:"jobs"`
	Cases   map[string]SupportCase `json:"cases"`
}
type SupportService struct {
	Config       SupportConfig
	MM           *Mattermost
	Investigator *SupportInvestigator
	State        SupportState
	Bot          MMUser
	Now          func() time.Time
}

func NewSupportService(c SupportConfig) *SupportService {
	s := &SupportService{Config: c, MM: NewMattermost(c.URL, os.Getenv("MATTERMOST_TOKEN")), Investigator: NewSupportInvestigator(c), Now: time.Now}
	s.Investigator.Cases = func(ctx context.Context, channel, query string) ([]Evidence, error) {
		ch, e := c.Channel(channel)
		if e != nil {
			return nil, e
		}
		if e = s.reconcile(ctx, channel); e != nil {
			return nil, fmt.Errorf("cannot verify case confirmations: %w", e)
		}
		return searchSupportCases(ctx, c, ch, s.State.Cases, query)
	}
	return s
}
func (s *SupportService) save() error {
	return atomicJSON(filepath.Join(s.Config.State, "support.json"), s.State)
}
func (s *SupportService) Initialize(ctx context.Context) error {
	bot, e := s.MM.User(ctx, "me")
	if e != nil {
		return e
	}
	if !bot.IsBot {
		return fmt.Errorf("MATTERMOST_TOKEN must belong to a bot account")
	}
	s.Bot = bot
	b, e := os.ReadFile(filepath.Join(s.Config.State, "support.json"))
	if e == nil {
		if e = json.Unmarshal(b, &s.State); e != nil {
			return e
		}
		if s.State.Version != 1 || s.State.URL != s.Config.URL || s.State.BotID != bot.ID {
			return fmt.Errorf("support state belongs to another server/bot or unsupported version")
		}
	} else if os.IsNotExist(e) {
		s.State = SupportState{Version: 1, URL: s.Config.URL, BotID: bot.ID, Cursors: map[string]int64{}, Seen: map[string]bool{}, Threads: map[string]string{}, Cases: map[string]SupportCase{}}
	} else {
		return e
	}
	if s.State.Cursors == nil || s.State.Seen == nil || s.State.Threads == nil || s.State.Cases == nil {
		return fmt.Errorf("invalid support state")
	}
	for _, ch := range s.Config.Channels {
		if _, ok := s.State.Cursors[ch.ID]; !ok {
			s.State.Cursors[ch.ID] = s.Now().UnixMilli()
		}
	}
	return s.save()
}

func (s *SupportService) Poll(ctx context.Context) error {
	for _, ch := range s.Config.Channels {
		posts, e := s.MM.PostsSince(ctx, ch.ID, s.State.Cursors[ch.ID])
		if e != nil {
			return e
		}
		sort.Slice(posts, func(i, j int) bool {
			if posts[i].CreateAt == posts[j].CreateAt {
				return posts[i].ID < posts[j].ID
			}
			return posts[i].CreateAt < posts[j].CreateAt
		})
		high := s.State.Cursors[ch.ID]
		for _, p := range posts {
			if p.CreateAt > high {
				high = p.CreateAt
			}
			if s.State.Seen[p.ID] {
				continue
			}
			if p.ChannelID != ch.ID {
				return fmt.Errorf("post outside channel")
			}
			if p.UserID == s.Bot.ID || p.Type != "" || p.DeleteAt != 0 {
				s.State.Seen[p.ID] = true
				continue
			}
			u, e := s.MM.User(ctx, p.UserID)
			if e != nil {
				return e
			}
			s.State.Seen[p.ID] = true
			if u.IsBot || p.Props["from_bot"] == "true" {
				continue
			}
			if p.RootID != "" && !mentionsBot(p.Message, s.Bot.Username) {
				continue
			}
			if strings.TrimSpace(p.Message) == "" {
				continue
			}
			root := p.RootID
			if root == "" {
				root = p.ID
			}
			s.State.Threads[root] = ch.ID
			s.State.Jobs = append(s.State.Jobs, &SupportJob{ID: p.ID, ChannelID: ch.ID, RootID: root, Question: p.Message, Scope: supportScope(ch), Status: "queued", Publication: hashText(s.Config.URL + "/" + s.Bot.ID + "/" + p.ID)})
		}
		s.State.Cursors[ch.ID] = high
		if e = s.save(); e != nil {
			return e
		}
	}
	// Reactions don't necessarily change channel post timestamps. Reconcile
	// tracked threads independently, including when no new posts arrive.
	for _, ch := range s.Config.Channels {
		if e := s.reconcile(ctx, ch.ID); e != nil {
			log.Printf("support confirmation refresh unavailable for channel %s", ch.ID)
		}
	}
	return nil
}
func mentionsBot(text, name string) bool {
	marker := "@" + name
	for start := 0; start < len(text); {
		n := strings.Index(text[start:], marker)
		if n < 0 {
			return false
		}
		n += start
		end := n + len(marker)
		boundary := func(b byte) bool {
			return !(b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9' || b == '_' || b == '-' || b == '.')
		}
		if (n == 0 || boundary(text[n-1])) && (end == len(text) || boundary(text[end])) {
			return true
		}
		start = end
	}
	return false
}
func (s *SupportService) reconcile(ctx context.Context, channel string) error {
	ch, e := s.Config.Channel(channel)
	if e != nil {
		return e
	}
	var first error
	for root, id := range s.State.Threads {
		if id != channel {
			continue
		}
		// Fail closed: an unreachable/deleted thread cannot supply confirmed cases.
		for key, v := range s.State.Cases {
			if v.RootID == root {
				delete(s.State.Cases, key)
			}
		}
		t, e := s.Investigator.Thread(ctx, channel, root)
		if e != nil {
			if first == nil {
				first = e
			}
			continue
		}
		for _, v := range confirmedCases(s.Config, ch, root, t) {
			s.State.Cases[v.PostID] = v
		}
	}
	if e = s.save(); e != nil {
		return e
	}
	return first
}

func (s *SupportService) Process(ctx context.Context) error {
	for _, job := range s.State.Jobs {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if job.Status == "sent" || job.Status == "cancelled" || job.NextAttempt > s.Now().UnixMilli() {
			continue
		}
		ch, e := s.Config.Channel(job.ChannelID)
		if e != nil || supportScope(ch) != job.Scope {
			job.Status = "cancelled"
			job.Error = "channel/project mapping changed"
			if e = s.save(); e != nil {
				return e
			}
			continue
		}
		if job.Status == "queued" {
			job.Attempts++
			if needsFinalization(job.Operation) {
				finalCtx, cancel := context.WithTimeout(ctx, s.Config.Agent.Timeout())
				var answer Answer
				answer, e = s.Investigator.Engine.finalizeAnswer(finalCtx, &job.Operation, job.Operation.Draft, s.save)
				cancel()
				if e == nil {
					job.Operation.Answer = &answer
					job.Operation.Error = ""
				}
			} else {
				job.Operation, e = s.Investigator.Investigate(ctx, ch, job.RootID, job.Question, func(op Operation) error { job.Operation = op; return s.save() })
			}
			if e != nil {
				job.Error = e.Error()
				log.Printf("support investigation %s failed (attempt %d)", job.ID, job.Attempts)
				if ctx.Err() != nil {
					_ = s.save()
					return ctx.Err()
				}
				if job.Attempts < 3 {
					job.NextAttempt = s.Now().Add(time.Duration(job.Attempts) * 30 * time.Second).UnixMilli()
					if e = s.save(); e != nil {
						return e
					}
					continue
				}
				job.Text = "Support analysis could not be completed because an investigation dependency or answer validation failed. This is an operational failure, not a diagnosis. Please ask a human reviewer, or mention the bot to retry."
			} else {
				job.Text = Render(*job.Operation.Answer, job.Operation.Evidence)
				if len([]rune(job.Text)) > 14000 {
					job.Text = "Support analysis exceeded the message limit and was not published. Please narrow the question or ask a human reviewer."
					job.Error = "rendered answer too large"
				}
			}
			job.Status = "ready"
			job.NextAttempt = 0
			if e = s.save(); e != nil {
				return e
			}
		}
		// Always reconcile first, including after a timeout/crash between POST and
		// saving its response. Never trust marker text from a different author.
		t, e := s.Investigator.Thread(ctx, job.ChannelID, job.RootID)
		if e != nil {
			job.NextAttempt = s.Now().Add(30 * time.Second).UnixMilli()
			job.Error = e.Error()
			if e = s.save(); e != nil {
				return e
			}
			continue
		}
		for _, p := range t.Posts {
			if p.UserID == s.Bot.ID && p.Props["support_publication"] == job.Publication {
				job.Status = "sent"
				job.PostID = p.ID
				break
			}
		}
		if job.Status == "sent" {
			if e = s.save(); e != nil {
				return e
			}
			continue
		}
		job.Status = "publishing"
		if e = s.save(); e != nil {
			return e
		}
		p, e := s.MM.Post(ctx, job.ChannelID, job.RootID, job.Text, job.Publication)
		if e != nil {
			job.Error = e.Error()
			job.NextAttempt = s.Now().Add(30 * time.Second).UnixMilli()
			if e = s.save(); e != nil {
				return e
			}
			continue
		}
		job.Status = "sent"
		job.PostID = p.ID
		job.NextAttempt = 0
		if e = s.save(); e != nil {
			return e
		}
		log.Printf("support replied to thread %s (job %s)", job.RootID, job.ID)
	}
	return nil
}

func RunSupport(ctx context.Context, c SupportConfig, once bool) error {
	if e := os.MkdirAll(c.State, 0700); e != nil {
		return e
	}
	lock, e := os.OpenFile(filepath.Join(c.State, "support.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); e != nil {
		return fmt.Errorf("support service already running")
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	s := NewSupportService(c)
	if e = s.Initialize(ctx); e != nil {
		return e
	}
	delay := time.Duration(c.PollSeconds) * time.Second
	baseDelay := delay
	for {
		e = s.Poll(ctx)
		if e == nil {
			e = s.Process(ctx)
		}
		if once {
			return e
		}
		if e != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("support cycle unavailable; retrying in %s", delay)
		} else {
			delay = baseDelay
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
		if e != nil && delay < 5*time.Minute {
			delay *= 2
			if delay > 5*time.Minute {
				delay = 5 * time.Minute
			}
		}
	}
}
