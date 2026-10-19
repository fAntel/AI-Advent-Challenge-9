package agent

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type SupportChannel struct {
	ID       string            `toml:"id" json:"id"`
	Projects map[string]string `toml:"projects" json:"projects"`
}

type SupportConfig struct {
	URL         string           `toml:"url"`
	State       string           `toml:"state"`
	MCP         string           `toml:"mattermost_mcp"`
	PollSeconds int              `toml:"poll_seconds"`
	Reviewers   []string         `toml:"reviewers"`
	Channels    []SupportChannel `toml:"channels"`
	Agent       Config           `toml:"agent"`
	Path        string           `toml:"-"`
}

func LoadSupportConfig(path string) (SupportConfig, error) {
	base, err := LoadConfig("")
	if err != nil {
		return SupportConfig{}, err
	}
	c := SupportConfig{Agent: base, State: filepath.Join(filepath.Dir(base.State), "support"), MCP: filepath.Join(filepath.Dir(base.GitMCP), "mattermost-mcp"), PollSeconds: 15}
	if path == "" {
		return c, fmt.Errorf("--config is required")
	}
	c.Path, err = filepath.Abs(path)
	if err != nil {
		return c, err
	}
	if _, err = toml.DecodeFile(c.Path, &c); err != nil {
		return c, err
	}
	c.URL = strings.TrimRight(c.URL, "/")
	u, err := url.Parse(c.URL)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1"))) {
		return c, fmt.Errorf("Mattermost URL must use HTTPS (HTTP allowed on loopback)")
	}
	if c.PollSeconds < 1 || len(c.Channels) == 0 || c.Agent.TimeoutSeconds < 1 || c.Agent.Model == "" {
		return c, fmt.Errorf("invalid support configuration")
	}
	for _, p := range []string{c.State, c.MCP, c.Agent.State, c.Agent.Indexer, c.Agent.GitMCP} {
		if !filepath.IsAbs(p) {
			return c, fmt.Errorf("support paths must be absolute")
		}
	}
	// Canonical state boundaries also protect aliases through internal symlinks.
	for _, p := range []*string{&c.State, &c.Agent.State} {
		if real, e := filepath.EvalSymlinks(*p); e == nil {
			*p = real
		}
	}
	seen := map[string]bool{}
	for i := range c.Channels {
		ch := &c.Channels[i]
		if !validMMID(ch.ID) || seen[ch.ID] || len(ch.Projects) == 0 {
			return c, fmt.Errorf("invalid/duplicate channel or empty project mapping")
		}
		seen[ch.ID] = true
		for name, path := range ch.Projects {
			if name == "" || strings.ContainsAny(name, "/\\:") || !filepath.IsAbs(path) {
				return c, fmt.Errorf("invalid project name/path")
			}
			root, e := ProjectRoot(path)
			if e != nil {
				return c, e
			}
			ch.Projects[name] = root
			// A state directory must not contain a source root. State inside a root is
			// permitted and explicitly protected by the support filesystem wrapper.
			if within(c.State, root) {
				return c, fmt.Errorf("project cannot be inside support state")
			}
		}
	}
	for _, id := range c.Reviewers {
		if !validMMID(id) {
			return c, fmt.Errorf("invalid reviewer ID")
		}
	}
	return c, nil
}

func (c SupportConfig) Channel(id string) (SupportChannel, error) {
	for _, ch := range c.Channels {
		if ch.ID == id {
			return ch, nil
		}
	}
	return SupportChannel{}, fmt.Errorf("channel not configured")
}
