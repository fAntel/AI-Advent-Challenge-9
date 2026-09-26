package agent

import (
	"github.com/BurntSushi/toml"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestTOMLDuration(t *testing.T) {
	cfg := DefaultConfig()
	input := "[daemon]\nlease_timeout = '45s'\n"
	meta, err := toml.Decode(input, &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Undecoded()) != 0 {
		t.Fatalf("undecoded=%v", meta.Undecoded())
	}
	if time.Duration(cfg.Daemon.LeaseTimeout) != 45*time.Second {
		t.Fatalf("lease=%s", time.Duration(cfg.Daemon.LeaseTimeout))
	}
}

func TestXDGPathsAndProfileConfigPrecedence(t *testing.T) {
	configHome := t.TempDir()
	stateHome := t.TempDir()
	runtimeDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	t.Setenv("XDG_STATE_HOME", stateHome)
	t.Setenv("XDG_RUNTIME_DIR", runtimeDir)
	root := filepath.Join(configHome, "advent-agent")
	profileDir := filepath.Join(root, "profiles", "coding")
	if err := os.MkdirAll(profileDir, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "config.toml"), []byte("[deepseek]\nmodel='deepseek-v4-pro'\ntemperature=0.3\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(profileDir, "config.toml"), []byte("[deepseek]\nmodel='deepseek-v4-flash'\ntemperature=0.7\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadProfileConfig("coding")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Profile != "coding" || cfg.DeepSeek.Model != FlashModel || cfg.DeepSeek.Temperature == nil || *cfg.DeepSeek.Temperature != 0.7 {
		t.Fatalf("config=%+v", cfg)
	}
	if cfg.ConfigDir != root || cfg.StateDir != filepath.Join(stateHome, "advent-agent") || cfg.Daemon.SocketPath != filepath.Join(runtimeDir, "advent-agent", "agent.sock") {
		t.Fatalf("paths config=%s state=%s socket=%s", cfg.ConfigDir, cfg.StateDir, cfg.Daemon.SocketPath)
	}
}

func TestProfileDiscoveryAndValidation(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"writer", "coding", "Bad Name"} {
		if err := os.MkdirAll(filepath.Join(root, "profiles", name), 0700); err != nil {
			t.Fatal(err)
		}
	}
	got, err := DiscoverProfiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"coding", "writer"}) {
		t.Fatalf("profiles=%v", got)
	}
	for _, name := range []string{"", "Upper", "../escape", "space name"} {
		if ValidateProfileName(name) == nil {
			t.Fatalf("profile %q accepted", name)
		}
	}
}

func TestInstructionResolutionOrderAndSnapshot(t *testing.T) {
	configRoot := t.TempDir()
	project := t.TempDir()
	cfg := DefaultConfig()
	cfg.ConfigDir = configRoot
	cfg.Profile = "coding"
	profileDir := filepath.Join(configRoot, "profiles", "coding")
	if err := os.MkdirAll(profileDir, 0700); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{filepath.Join(configRoot, "AGENTS.md"): "shared", filepath.Join(profileDir, "AGENTS.md"): "coding", filepath.Join(project, "AGENTS.md"): "project"}
	for path, content := range files {
		if err := os.WriteFile(path, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ResolveInstructions(cfg, project, "session", "<test>")
	if err != nil {
		t.Fatal(err)
	}
	contents := []string{}
	for _, source := range got {
		contents = append(contents, source.Content)
	}
	if !reflect.DeepEqual(contents, []string{"shared", "coding", "project", "session"}) {
		t.Fatalf("instructions=%+v", got)
	}
	if err := os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if got[2].Content != "project" {
		t.Fatal("snapshot changed after source edit")
	}
}

func TestUnknownTOMLKeyIsDetectable(t *testing.T) {
	cfg := DefaultConfig()
	meta, err := toml.Decode("[daemon]\nunknown=true", &cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(meta.Undecoded()) != 1 {
		t.Fatalf("undecoded=%v", meta.Undecoded())
	}
}

func TestClientAutostartDefaultsOn(t *testing.T) {
	if !DefaultConfig().Client.Autostart {
		t.Fatal("autostart should default to true")
	}
}

func TestHistoryCompressionDefaults(t *testing.T) {
	cfg := DefaultConfig()
	if !cfg.Agent.CompressionEnabled || cfg.Agent.ContextStrategy != "summary" || cfg.Agent.RecentMessages != 5 || cfg.Agent.SummaryBatchMessages != 5 {
		t.Fatalf("agent config=%+v", cfg.Agent)
	}
	if !DefaultSettings(cfg).Compression {
		t.Fatal("new sessions should enable compression by default")
	}
}

func TestContextStrategyValidation(t *testing.T) {
	settings := DefaultSettings(DefaultConfig())
	settings.ContextStrategy = "unknown"
	if ValidateSettings(settings) == nil {
		t.Fatal("expected unknown context strategy to be rejected")
	}
}
