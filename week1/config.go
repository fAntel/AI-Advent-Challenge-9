package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Daemon   DaemonConfig   `toml:"daemon"`
	DeepSeek DeepSeekConfig `toml:"deepseek"`
	Agent    AgentConfig    `toml:"agent"`
	Client   ClientConfig   `toml:"client"`
}

type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
	parsed, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

type DaemonConfig struct {
	SocketPath     string   `toml:"socket_path"`
	SessionDir     string   `toml:"session_dir"`
	RequestTimeout Duration `toml:"request_timeout"`
	LeaseTimeout   Duration `toml:"lease_timeout"`
	EvictAfter     Duration `toml:"evict_after"`
	Debug          bool     `toml:"debug"`
	LogPath        string   `toml:"log_path"`
	LogMaxBytes    int64    `toml:"log_max_bytes"`
	LogBackups     int      `toml:"log_backups"`
}

type DeepSeekConfig struct {
	Endpoint    string   `toml:"endpoint"`
	Model       string   `toml:"model"`
	Reasoning   string   `toml:"reasoning"`
	Temperature *float64 `toml:"temperature"`
}

type AgentConfig struct {
	SystemPrompt         string `toml:"system_prompt"`
	ContextWindowTokens  int    `toml:"context_window_tokens"`
	CompressionEnabled   bool   `toml:"compression_enabled"`
	RecentMessages       int    `toml:"recent_messages"`
	SummaryBatchMessages int    `toml:"summary_batch_messages"`
}
type ClientConfig struct {
	PollInterval Duration `toml:"poll_interval"`
	Autostart    bool     `toml:"autostart"`
}

func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	state := filepath.Join(home, ".local", "state", "deepseek-agent")
	logPath := filepath.Join(state, "agentd.log")
	if userLogs, err := os.UserCacheDir(); err == nil && strings.Contains(userLogs, "Library/Caches") {
		logPath = filepath.Join(home, "Library", "Logs", "deepseek-agent", "agentd.log")
	}
	return Config{
		Daemon: DaemonConfig{
			SocketPath: filepath.Join(state, "agent.sock"), SessionDir: filepath.Join(state, "sessions"),
			RequestTimeout: Duration(5 * time.Minute), LeaseTimeout: Duration(30 * time.Second), EvictAfter: Duration(2 * time.Second),
			LogPath: logPath, LogMaxBytes: 10 << 20, LogBackups: 3,
		},
		DeepSeek: DeepSeekConfig{Endpoint: "https://api.deepseek.com/chat/completions", Model: FlashModel, Reasoning: "none"},
		Agent: AgentConfig{
			ContextWindowTokens: 1_000_000, CompressionEnabled: true,
			RecentMessages: 5, SummaryBatchMessages: 5,
		},
		Client: ClientConfig{PollInterval: Duration(250 * time.Millisecond), Autostart: true},
	}
}

func LoadConfig() (Config, error) {
	cfg := DefaultConfig()
	home, err := os.UserHomeDir()
	if err != nil {
		return cfg, err
	}
	paths := []string{"/etc/deepseek-agent/config.toml", filepath.Join(home, ".config", "deepseek-agent", "config.toml")}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return cfg, fmt.Errorf("read config %q: %w", path, err)
		}
		meta, err := toml.Decode(string(data), &cfg)
		if err != nil {
			return cfg, fmt.Errorf("parse config %q: %w", path, err)
		}
		if undecoded := meta.Undecoded(); len(undecoded) != 0 {
			return cfg, fmt.Errorf("config %q: unknown key %s", path, undecoded[0])
		}
	}
	cfg.Daemon.SocketPath = expandHome(cfg.Daemon.SocketPath, home)
	cfg.Daemon.SessionDir = expandHome(cfg.Daemon.SessionDir, home)
	cfg.Daemon.LogPath = expandHome(cfg.Daemon.LogPath, home)
	if err := cfg.Validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func expandHome(path, home string) string {
	if path == "~" {
		return home
	}
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, path[2:])
	}
	return path
}

func (c Config) Validate() error {
	if c.Daemon.SocketPath == "" || c.Daemon.SessionDir == "" {
		return errors.New("socket_path and session_dir are required")
	}
	if c.Daemon.RequestTimeout <= 0 || c.Daemon.LeaseTimeout <= 0 || c.Daemon.EvictAfter < 0 || c.Client.PollInterval <= 0 {
		return errors.New("timeouts and poll_interval must be positive")
	}
	if c.Daemon.LogMaxBytes <= 0 || c.Daemon.LogBackups < 0 {
		return errors.New("invalid log rotation settings")
	}
	if c.Agent.ContextWindowTokens <= 0 {
		return errors.New("agent.context_window_tokens must be positive")
	}
	if c.Agent.RecentMessages <= 0 || c.Agent.SummaryBatchMessages <= 0 {
		return errors.New("agent recent_messages and summary_batch_messages must be positive")
	}
	return ValidateSettings(Settings{Model: c.DeepSeek.Model, Reasoning: c.DeepSeek.Reasoning, Temperature: c.DeepSeek.Temperature, Approach: "none"})
}

func DefaultSettings(c Config) Settings {
	return Settings{Model: c.DeepSeek.Model, Reasoning: c.DeepSeek.Reasoning, Temperature: c.DeepSeek.Temperature, Approach: "none", Roles: []string{"Business analyst", "Engineer", "Critic"}, Compression: c.Agent.CompressionEnabled}
}

func ValidateSettings(s Settings) error {
	if s.Model != FlashModel && s.Model != ProModel {
		return fmt.Errorf("unknown model %q", s.Model)
	}
	switch s.Reasoning {
	case "none", "low", "high", "max":
	default:
		return fmt.Errorf("unknown reasoning %q", s.Reasoning)
	}
	switch s.Approach {
	case "", "none", "step-by-step", "self-prompt", "multi-role":
	default:
		return fmt.Errorf("unknown approach %q", s.Approach)
	}
	if s.Temperature != nil && (*s.Temperature < 0 || *s.Temperature > 2) {
		return errors.New("temperature must be between 0 and 2")
	}
	if s.Temperature != nil && s.Reasoning != "none" {
		return errors.New("temperature cannot be used when reasoning is enabled")
	}
	if s.Approach == "multi-role" && len(s.Roles) < 2 {
		return errors.New("multi-role requires at least two roles")
	}
	return nil
}
