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
