package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type SupportCase struct {
	PostID      string `json:"post_id"`
	RootID      string `json:"root_id"`
	ChannelID   string `json:"channel_id"`
	Scope       string `json:"scope"`
	Problem     string `json:"problem"`
	Resolution  string `json:"resolution"`
	AuthorID    string `json:"author_id"`
	ReviewerID  string `json:"reviewer_id"`
	ConfirmedAt int64  `json:"confirmed_at"`
	UpdatedAt   int64  `json:"updated_at"`
	Hash        string `json:"hash"`
}

func confirmedCases(c SupportConfig, ch SupportChannel, root string, t MMThread) []SupportCase {
	reviewers := map[string]bool{}
	for _, id := range c.Reviewers {
		reviewers[id] = true
	}
	var problem MMPost
	for _, p := range t.Posts {
		if p.ID == root {
			problem = p
		}
	}
	if problem.ID == "" {
		return nil
	}
	out := []SupportCase{}
	for _, p := range t.Posts {
		if p.ID == root || p.DeleteAt != 0 || strings.TrimSpace(p.Message) == "" {
			continue
		}
		// Confirmations must follow the current revision. An old reaction cannot
		// silently endorse a subsequently edited answer or problem statement.
		for _, r := range t.Reactions {
			if r.PostID == p.ID && r.Emoji == "white_check_mark" && reviewers[r.UserID] && r.CreateAt >= p.UpdateAt && r.CreateAt >= problem.UpdateAt {
				hash := hashText(problem.Message + "\x00" + p.Message)
				out = append(out, SupportCase{PostID: p.ID, RootID: root, ChannelID: ch.ID, Scope: supportScope(ch), Problem: problem.Message, Resolution: p.Message, AuthorID: p.UserID, ReviewerID: r.UserID, ConfirmedAt: r.CreateAt, UpdatedAt: p.UpdateAt, Hash: hash})
				break
			}
		}
	}
	return out
}

func caseDocuments(c SupportConfig, ch SupportChannel, cases map[string]SupportCase) []map[string]string {
	docs := []map[string]string{}
	for _, v := range cases {
		if v.Scope != supportScope(ch) || v.ChannelID != ch.ID {
			continue
		}
		text := fmt.Sprintf("# Human-confirmed support case\n\nProblem:\n%s\n\nConfirmed resolution (historical report):\n%s\n\nAuthor: %s\nReviewer: %s\nConfirmed at: %d\n", v.Problem, v.Resolution, v.AuthorID, v.ReviewerID, v.ConfirmedAt)
		docs = append(docs, map[string]string{"path": postLink(c.URL, v.PostID), "text": text, "hash": hashText(text)})
	}
	sort.Slice(docs, func(i, j int) bool { return docs[i]["path"] < docs[j]["path"] })
	return docs
}

func caseIndexCommand(ctx context.Context, c SupportConfig, ch SupportChannel, docs []map[string]string, command, query string) ([]byte, error) {
	db := supportDatabase(c, ch)
	if e := os.MkdirAll(filepath.Dir(db), 0700); e != nil {
		return nil, e
	}
	snapshot, e := json.Marshal(docs)
	if e != nil {
		return nil, e
	}
	args := []string{"-I", c.Agent.Indexer, "--corpus-stdin", "--root", c.State, "--database", db, "--ollama", c.Agent.Ollama, "--model", c.Agent.EmbeddingModel, command}
	if command == "search" {
		args = append(args, "--", query)
	}
	cmd := exec.CommandContext(ctx, c.Agent.Python, args...)
	cmd.Stdin = bytes.NewReader(snapshot)
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "DEEPSEEK_API_KEY=") && !strings.HasPrefix(v, "MATTERMOST_TOKEN=") {
			cmd.Env = append(cmd.Env, v)
		}
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, e := cmd.Output()
	if e != nil {
		return nil, fmt.Errorf("case index: %s (%w)", strings.TrimSpace(stderr.String()), e)
	}
	return b, nil
}

func searchSupportCases(ctx context.Context, c SupportConfig, ch SupportChannel, cases map[string]SupportCase, query string) ([]Evidence, error) {
	docs := caseDocuments(c, ch, cases)
	if len(docs) == 0 {
		return []Evidence{}, nil
	}
	status, e := caseIndexCommand(ctx, c, ch, docs, "status", "")
	var meta struct {
		Stale bool `json:"stale"`
	}
	if e == nil {
		e = json.Unmarshal(status, &meta)
	}
	if e != nil || meta.Stale {
		if _, e = caseIndexCommand(ctx, c, ch, docs, "index", ""); e != nil {
			return nil, e
		}
	}
	b, e := caseIndexCommand(ctx, c, ch, docs, "search", query)
	if e != nil {
		return nil, e
	}
	var candidates []Evidence
	if e = json.Unmarshal(b, &candidates); e != nil {
		return nil, e
	}
	// Verify SQLite contents against the freshly reconciled confirmed corpus.
	allowed := map[string]string{}
	for _, d := range docs {
		allowed[d["path"]] = d["text"]
	}
	for _, ev := range candidates {
		body, ok := allowed[ev.Source]
		lines := strings.SplitAfter(body, "\n")
		if !ok || ev.Start < 1 || ev.End < ev.Start || ev.End > len(lines) || strings.Join(lines[ev.Start-1:ev.End], "") != ev.Text {
			return nil, fmt.Errorf("case index source is stale or invalid")
		}
	}
	return candidates, nil
}
