package main

import (
	agent "advent-agent"
	"fmt"
	"os"
	"testing"
	"time"
)

func TestStopDaemonSucceedsWhenDaemonIsAlreadyStopped(t *testing.T) {
	socket := fmt.Sprintf("/private/tmp/advent-agentd-%d-%d.sock", os.Getpid(), time.Now().UnixNano())
	stopped, err := stopDaemon(agent.NewAPIClient(socket, time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if stopped {
		t.Fatal("reported a stopped daemon when none was running")
	}
}

func TestLocalStartupSkipsCloudCredentials(t *testing.T) {
	cfg := agent.DefaultConfig()
	cfg.Provider.Name = "ollama"
	called := false
	key := providerKey(cfg, func() (string, error) { called = true; return "cloud-secret", nil })
	if called || key != "" {
		t.Fatal("local daemon accessed cloud credentials")
	}
	cfg.Provider.Name = "deepseek"
	if providerKey(cfg, func() (string, error) { return "cloud-secret", nil }) != "cloud-secret" {
		t.Fatal("cloud credentials changed")
	}
}
