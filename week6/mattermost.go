package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

var mmID = regexp.MustCompile(`^[a-z0-9]{26}$`)

func validMMID(s string) bool { return mmID.MatchString(s) }

type MMPost struct {
	ID        string         `json:"id"`
	ChannelID string         `json:"channel_id"`
	RootID    string         `json:"root_id"`
	UserID    string         `json:"user_id"`
	Message   string         `json:"message"`
	Type      string         `json:"type"`
	CreateAt  int64          `json:"create_at"`
	UpdateAt  int64          `json:"update_at"`
	DeleteAt  int64          `json:"delete_at"`
	Props     map[string]any `json:"props,omitempty"`
}
type MMUser struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	IsBot    bool   `json:"is_bot"`
}
type MMReaction struct {
	UserID   string `json:"user_id"`
	PostID   string `json:"post_id"`
	Emoji    string `json:"emoji_name"`
	CreateAt int64  `json:"create_at"`
}
type MMThread struct {
	Posts     []MMPost          `json:"posts"`
	Authors   map[string]MMUser `json:"authors"`
	Reactions []MMReaction      `json:"reactions"`
}
type MMPostList struct {
	Order []string          `json:"order"`
	Posts map[string]MMPost `json:"posts"`
}

type Mattermost struct {
	URL   string
	Token string
	HTTP  *http.Client
}

func NewMattermost(base, token string) *Mattermost {
	return &Mattermost{URL: strings.TrimRight(base, "/"), Token: token, HTTP: &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("Mattermost redirects refused") }}}
}
func (m *Mattermost) Request(ctx context.Context, method, path string, in, out any) error {
	if m.Token == "" {
		return fmt.Errorf("MATTERMOST_TOKEN is required")
	}
	var body io.Reader
	if in != nil {
		b, e := json.Marshal(in)
		if e != nil {
			return e
		}
		body = bytes.NewReader(b)
	}
	req, e := http.NewRequestWithContext(ctx, method, m.URL+"/api/v4"+path, body)
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+m.Token)
	req.Header.Set("Content-Type", "application/json")
	resp, e := m.HTTP.Do(req)
	if e != nil {
		return fmt.Errorf("Mattermost request failed: %w", e)
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
	if e != nil {
		return e
	}
	if len(b) > 8<<20 {
		return fmt.Errorf("Mattermost response exceeds 8 MiB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("Mattermost HTTP %d", resp.StatusCode)
	}
	if out != nil {
		return json.Unmarshal(b, out)
	}
	return nil
}
func (m *Mattermost) User(ctx context.Context, id string) (MMUser, error) {
	var u MMUser
	if id != "me" && !validMMID(id) {
		return u, fmt.Errorf("invalid user ID")
	}
	e := m.Request(ctx, "GET", "/users/"+id, nil, &u)
	return u, e
}
func (m *Mattermost) Thread(ctx context.Context, c SupportConfig, channel, root string) (MMThread, error) {
	t := MMThread{Authors: map[string]MMUser{}, Posts: []MMPost{}, Reactions: []MMReaction{}}
	if _, e := c.Channel(channel); e != nil {
		return t, e
	}
	if !validMMID(root) {
		return t, fmt.Errorf("invalid thread ID")
	}
	// Check the root's channel before fetching the rest of the thread.
	var p MMPost
	if e := m.Request(ctx, "GET", "/posts/"+root, nil, &p); e != nil {
		return t, e
	}
	if p.ChannelID != channel || p.RootID != "" || p.DeleteAt != 0 {
		return t, fmt.Errorf("thread outside configured channel or deleted")
	}
	var list MMPostList
	if e := m.Request(ctx, "GET", "/posts/"+root+"/thread", nil, &list); e != nil {
		return t, e
	}
	if len(list.Posts) > 100 {
		return t, fmt.Errorf("thread exceeds 100 posts; narrow context before analysis")
	}
	size := 0
	for _, p := range list.Posts {
		if p.ChannelID != channel || (p.ID != root && p.RootID != root) {
			return t, fmt.Errorf("invalid thread membership")
		}
		if p.DeleteAt != 0 {
			continue
		}
		size += len(p.Message)
		if size > 96000 {
			return t, fmt.Errorf("thread text exceeds 96000 bytes")
		}
		t.Posts = append(t.Posts, p)
		if _, ok := t.Authors[p.UserID]; !ok {
			u, e := m.User(ctx, p.UserID)
			if e != nil {
				return t, e
			}
			t.Authors[p.UserID] = u
		}
		var reactions []MMReaction
		if e := m.Request(ctx, "GET", "/posts/"+p.ID+"/reactions", nil, &reactions); e != nil {
			return t, e
		}
		t.Reactions = append(t.Reactions, reactions...)
	}
	sort.Slice(t.Posts, func(i, j int) bool {
		if t.Posts[i].CreateAt == t.Posts[j].CreateAt {
			return t.Posts[i].ID < t.Posts[j].ID
		}
		return t.Posts[i].CreateAt < t.Posts[j].CreateAt
	})
	return t, nil
}
func (m *Mattermost) PostsSince(ctx context.Context, channel string, since int64) ([]MMPost, error) {
	if !validMMID(channel) {
		return nil, fmt.Errorf("invalid channel ID")
	}
	out := []MMPost{}
	for page := 0; page < 1000; page++ {
		var list MMPostList
		if e := m.Request(ctx, "GET", fmt.Sprintf("/channels/%s/posts?per_page=100&page=%d", channel, page), nil, &list); e != nil {
			return nil, e
		}
		done := len(list.Order) < 100
		for _, id := range list.Order {
			p, ok := list.Posts[id]
			if !ok {
				return nil, fmt.Errorf("invalid post listing")
			}
			if p.CreateAt < since {
				done = true
				continue
			}
			out = append(out, p)
		}
		if done {
			return out, nil
		}
	}
	return nil, fmt.Errorf("channel catch-up exceeded 100000 posts")
}
func (m *Mattermost) Post(ctx context.Context, channel, root, text, key string) (MMPost, error) {
	var p MMPost
	e := m.Request(ctx, "POST", "/posts", map[string]any{"channel_id": channel, "root_id": root, "message": text, "props": map[string]string{"support_publication": key}}, &p)
	return p, e
}
func postLink(base, id string) string {
	return strings.TrimRight(base, "/") + "/_redirect/pl/" + url.PathEscape(id)
}

type ThreadArgs struct {
	ChannelID string `json:"channel_id"`
	RootID    string `json:"root_id"`
}

func MattermostServer(c SupportConfig, m *Mattermost) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "mattermost-mcp", Version: "1.0.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "get_thread", Description: "Read an allowed support thread, authors and reactions; no mutations.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(ctx context.Context, _ *mcp.CallToolRequest, a ThreadArgs) (*mcp.CallToolResult, MMThread, error) {
		v, e := m.Thread(ctx, c, a.ChannelID, a.RootID)
		return nil, v, e
	})
	return s
}
func FetchSupportThread(ctx context.Context, c SupportConfig, channel, root string) (MMThread, error) {
	var t MMThread
	if _, e := c.Channel(channel); e != nil {
		return t, e
	}
	cmd := exec.Command(c.MCP, "--config", c.Path)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "DEEPSEEK_API_KEY=") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "support-bot", Version: "1.0.0"}, nil)
	session, e := client.Connect(ctx, &mcp.CommandTransport{Command: cmd, TerminateDuration: time.Second}, nil)
	if e != nil {
		return t, e
	}
	defer session.Close()
	r, e := session.CallTool(ctx, &mcp.CallToolParams{Name: "get_thread", Arguments: ThreadArgs{channel, root}})
	if e != nil {
		return t, e
	}
	if r.IsError {
		return t, fmt.Errorf("Mattermost MCP get_thread failed")
	}
	b, e := json.Marshal(r.StructuredContent)
	if e != nil {
		return t, e
	}
	if len(b) > 192000 {
		return t, fmt.Errorf("MCP context too large")
	}
	e = json.Unmarshal(b, &t)
	return t, e
}
