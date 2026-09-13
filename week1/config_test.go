package agent

import (
	"github.com/BurntSushi/toml"
	"testing"
	"time"
)

func TestTOMLDurationAndMultilinePrompt(t *testing.T) {
	cfg := DefaultConfig()
	input := "[daemon]\nlease_timeout = '45s'\n[agent]\nsystem_prompt = '''You are helpful.\nBe concise.'''\n"
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
	if cfg.Agent.SystemPrompt != "You are helpful.\nBe concise." {
		t.Fatalf("prompt=%q", cfg.Agent.SystemPrompt)
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
