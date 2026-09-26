package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

const DefaultProfile = "default"

type Config struct {
	Daemon    DaemonConfig   `toml:"daemon"`
	Provider  ProviderConfig `toml:"provider"`
	DeepSeek  DeepSeekConfig `toml:"deepseek"`
	Agent     AgentConfig    `toml:"agent"`
	Client    ClientConfig   `toml:"client"`
	Profile   string         `toml:"-"`
	ConfigDir string         `toml:"-"`
	StateDir  string         `toml:"-"`
}

type Duration time.Duration

func (d *Duration) UnmarshalText(text []byte) error {
	v, e := time.ParseDuration(string(text))
	if e == nil {
		*d = Duration(v)
	}
	return e
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
type ProviderConfig struct {
	Name string `toml:"name"`
}
type DeepSeekConfig struct {
	Endpoint    string   `toml:"endpoint"`
	Model       string   `toml:"model"`
	Reasoning   string   `toml:"reasoning"`
	Temperature *float64 `toml:"temperature"`
}
type AgentConfig struct {
	ContextWindowTokens  int    `toml:"context_window_tokens"`
	CompressionEnabled   bool   `toml:"compression_enabled"`
	RecentMessages       int    `toml:"recent_messages"`
	SummaryBatchMessages int    `toml:"summary_batch_messages"`
	ContextStrategy      string `toml:"context_strategy"`
}
type ClientConfig struct {
	PollInterval Duration `toml:"poll_interval"`
	Autostart    bool     `toml:"autostart"`
}

func xdgDir(env, fallback string) string {
	if value := os.Getenv(env); filepath.IsAbs(value) {
		return value
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, fallback)
}

func DefaultConfig() Config {
	configDir := filepath.Join(xdgDir("XDG_CONFIG_HOME", ".config"), "advent-agent")
	stateDir := filepath.Join(xdgDir("XDG_STATE_HOME", filepath.Join(".local", "state")), "advent-agent")
	runtimeBase := os.Getenv("XDG_RUNTIME_DIR")
	socket := filepath.Join(stateDir, "agent.sock")
	if filepath.IsAbs(runtimeBase) {
		socket = filepath.Join(runtimeBase, "advent-agent", "agent.sock")
	}
	return Config{
		Daemon:   DaemonConfig{SocketPath: socket, SessionDir: stateDir, RequestTimeout: Duration(5 * time.Minute), LeaseTimeout: Duration(30 * time.Second), EvictAfter: Duration(2 * time.Second), LogPath: filepath.Join(stateDir, "agentd.log"), LogMaxBytes: 10 << 20, LogBackups: 3},
		Provider: ProviderConfig{Name: "deepseek"},
		DeepSeek: DeepSeekConfig{Endpoint: "https://api.deepseek.com/chat/completions", Model: FlashModel, Reasoning: "none"},
		Agent:    AgentConfig{ContextWindowTokens: 1_000_000, CompressionEnabled: true, RecentMessages: 5, SummaryBatchMessages: 5, ContextStrategy: "summary"},
		Client:   ClientConfig{PollInterval: Duration(250 * time.Millisecond), Autostart: true}, ConfigDir: configDir, StateDir: stateDir, Profile: DefaultProfile,
	}
}

func LoadConfig() (Config, error) { return LoadProfileConfig(DefaultProfile) }
func LoadProfileConfig(profile string) (Config, error) {
	if err := ValidateProfileName(profile); err != nil {
		return Config{}, err
	}
	cfg := DefaultConfig()
	cfg.Profile = profile
	paths := []string{filepath.Join(cfg.ConfigDir, "config.toml"), filepath.Join(cfg.ConfigDir, "profiles", profile, "config.toml")}
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
		if u := meta.Undecoded(); len(u) > 0 {
			return cfg, fmt.Errorf("config %q: unknown key %s", path, u[0])
		}
	}
	home, _ := os.UserHomeDir()
	cfg.Daemon.SocketPath = expandHome(cfg.Daemon.SocketPath, home)
	cfg.Daemon.SessionDir = expandHome(cfg.Daemon.SessionDir, home)
	cfg.Daemon.LogPath = expandHome(cfg.Daemon.LogPath, home)
	cfg.StateDir = cfg.Daemon.SessionDir
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

var profileNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func ValidateProfileName(name string) error {
	if !profileNameRE.MatchString(name) {
		return fmt.Errorf("invalid profile name %q: use lowercase letters, digits, hyphens, or underscores", name)
	}
	return nil
}
func DiscoverProfiles(configDir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(configDir, "profiles"))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && ValidateProfileName(e.Name()) == nil {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func (c Config) Validate() error {
	if c.Daemon.SocketPath == "" || c.Daemon.SessionDir == "" {
		return errors.New("socket_path and session_dir are required")
	}
	if c.Provider.Name != "deepseek" {
		return fmt.Errorf("unknown provider %q", c.Provider.Name)
	}
	if c.Daemon.RequestTimeout <= 0 || c.Daemon.LeaseTimeout <= 0 || c.Daemon.EvictAfter < 0 || c.Client.PollInterval <= 0 {
		return errors.New("timeouts and poll_interval must be positive")
	}
	if c.Daemon.LogMaxBytes <= 0 || c.Daemon.LogBackups < 0 {
		return errors.New("invalid log rotation settings")
	}
	if c.Agent.ContextWindowTokens <= 0 || c.Agent.RecentMessages <= 0 || c.Agent.SummaryBatchMessages <= 0 {
		return errors.New("agent token and message limits must be positive")
	}
	if !validContextStrategy(c.Agent.ContextStrategy) {
		return fmt.Errorf("unknown agent.context_strategy %q", c.Agent.ContextStrategy)
	}
	return ValidateSettings(Settings{Provider: c.Provider.Name, Model: c.DeepSeek.Model, Reasoning: c.DeepSeek.Reasoning, Temperature: c.DeepSeek.Temperature, Approach: "none"})
}
func DefaultSettings(c Config) Settings {
	strategy := c.Agent.ContextStrategy
	if strategy == "summary" && !c.Agent.CompressionEnabled {
		strategy = "full"
	}
	return Settings{Provider: c.Provider.Name, Model: c.DeepSeek.Model, Reasoning: c.DeepSeek.Reasoning, Temperature: c.DeepSeek.Temperature, Approach: "none", Roles: []string{"Business analyst", "Engineer", "Critic"}, Compression: strategy == "summary", ContextStrategy: strategy}
}
func ValidateSettings(s Settings) error {
	if s.Provider != "" && s.Provider != "deepseek" {
		return fmt.Errorf("unknown provider %q", s.Provider)
	}
	if err := ValidateDeepSeekSettings(s); err != nil {
		return err
	}
	switch s.Approach {
	case "", "none", "step-by-step", "self-prompt", "multi-role":
	default:
		return fmt.Errorf("unknown approach %q", s.Approach)
	}
	if s.Approach == "multi-role" && len(s.Roles) < 2 {
		return errors.New("multi-role requires at least two roles")
	}
	if s.ContextStrategy != "" && !validContextStrategy(s.ContextStrategy) {
		return fmt.Errorf("unknown context strategy %q", s.ContextStrategy)
	}
	return nil
}
func validContextStrategy(v string) bool {
	switch v {
	case "full", "summary", "sliding", "branching":
		return true
	}
	return false
}
