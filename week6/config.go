package agent

import (
	"crypto/sha256"
	"fmt"
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	readBlocked        []string
	State              string `toml:"state"`
	Endpoint           string `toml:"endpoint"`
	Model              string `toml:"model"`
	Python             string `toml:"python"`
	Indexer            string `toml:"indexer"`
	GitMCP             string `toml:"git_mcp"`
	DataMCP            string `toml:"data_mcp"`
	DataMCPCredentials string `toml:"data_mcp_credentials"`
	Ollama             string `toml:"ollama"`
	EmbeddingModel     string `toml:"embedding_model"`
	TimeoutSeconds     int    `toml:"timeout_seconds"`
}

func LoadConfig(path string) (Config, error) {
	exe, err := os.Executable()
	if err != nil {
		return Config{}, err
	}
	base := filepath.Dir(exe)
	c := Config{State: filepath.Join(base, "data", "state"), Endpoint: "https://api.deepseek.com/chat/completions", Model: "deepseek-v4-flash", Python: "python3", Indexer: filepath.Join(base, "scripts", "repo_index.py"), GitMCP: filepath.Join(base, "git-mcp"), Ollama: "http://127.0.0.1:11434", EmbeddingModel: "embeddinggemma:300m", TimeoutSeconds: 300}
	c.DataMCP = bundledDataMCP(base)
	if path != "" {
		if _, err = toml.DecodeFile(path, &c); err != nil {
			return c, err
		}
	}
	if c.DataMCPCredentials == "" {
		c.DataMCPCredentials = filepath.Join(c.State, "healbe-credentials.json")
	}
	c.readBlocked = append(c.readBlocked, c.DataMCPCredentials)
	if !filepath.IsAbs(c.DataMCPCredentials) {
		return c, fmt.Errorf("data_mcp_credentials must be absolute")
	}
	if c.TimeoutSeconds < 1 || c.Model == "" {
		return c, fmt.Errorf("invalid timeout/model")
	}
	if c.DataMCP != "" && !filepath.IsAbs(c.DataMCP) {
		return c, fmt.Errorf("data_mcp must be an absolute executable path")
	}
	for _, p := range []*string{&c.State, &c.Indexer, &c.GitMCP} {
		if !filepath.IsAbs(*p) {
			return c, fmt.Errorf("configuration paths must be absolute: %s", *p)
		}
	}
	return c, nil
}

func bundledDataMCP(base string) string {
	for _, relative := range []string{
		"data-mcp/build/package/healbe-data-mcp.app/Contents/MacOS/healbe-data-mcp",
		"data-mcp/build/package/healbe-data-mcp/bin/healbe-data-mcp",
	} {
		path := filepath.Join(base, relative)
		if info, err := os.Stat(path); err == nil && !info.IsDir() && info.Mode()&0111 != 0 {
			return path
		}
	}
	return ""
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
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	root, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project must be a directory")
	}
	for _, part := range strings.Split(root, string(filepath.Separator)) {
		if strings.EqualFold(part, ".git") {
			return "", fmt.Errorf("project cannot be Git metadata")
		}
	}
	return root, nil
}
