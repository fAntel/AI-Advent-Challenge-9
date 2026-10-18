package agent

import (
	"crypto/sha256"
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	State          string `toml:"state"`
	Endpoint       string `toml:"endpoint"`
	Model          string `toml:"model"`
	Python         string `toml:"python"`
	Indexer        string `toml:"indexer"`
	GitMCP         string `toml:"git_mcp"`
	Ollama         string `toml:"ollama"`
	EmbeddingModel string `toml:"embedding_model"`
	TimeoutSeconds int    `toml:"timeout_seconds"`
}

func LoadConfig(path string) (Config, error) {
	exe, err := os.Executable()
	if err != nil {
		return Config{}, err
	}
	base := filepath.Dir(exe)
	c := Config{State: filepath.Join(base, "data", "state"), Endpoint: "https://api.deepseek.com/chat/completions", Model: "deepseek-v4-flash", Python: "python3", Indexer: filepath.Join(base, "scripts", "repo_index.py"), GitMCP: filepath.Join(base, "git-mcp"), Ollama: "http://127.0.0.1:11434", EmbeddingModel: "embeddinggemma:300m", TimeoutSeconds: 300}
	if path != "" {
		if _, err = toml.DecodeFile(path, &c); err != nil {
			return c, err
		}
	}
	if c.TimeoutSeconds < 1 || c.Model == "" {
		return c, fmt.Errorf("invalid timeout/model")
	}
	for _, p := range []*string{&c.State, &c.Indexer, &c.GitMCP} {
		if !filepath.IsAbs(*p) {
			return c, fmt.Errorf("configuration paths must be absolute: %s", *p)
		}
	}
	return c, nil
}
func (c Config) Timeout() time.Duration { return time.Duration(c.TimeoutSeconds) * time.Second }
func (c Config) Socket() string         { return filepath.Join(c.State, "agent.sock") }
func (c Config) Database(root string) string {
	h := sha256.Sum256([]byte(root))
	return filepath.Join(c.State, "indexes", fmt.Sprintf("%x.sqlite", h[:8]))
}
func ProjectRoot(path string) (string, error) {
	if path == "" {
		path = "."
	}
	out, err := exec.Command("git", "-C", path, "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("project must be inside a Git repository: %w", err)
	}
	return filepath.EvalSymlinks(strings.TrimSpace(string(out)))
}
