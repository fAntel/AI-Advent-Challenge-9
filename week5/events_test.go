package agent

import (
	"bufio"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestEventsPushToConnectedClientsAndReplayLatest(t *testing.T) {
	hub := NewEventHub()
	server := httptest.NewServer(Server{Events: hub}.Handler())
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	ready := make(chan Notice, 1)
	go func() {
		line, _ := bufio.NewReader(response.Body).ReadString('\n')
		var n Notice
		_ = json.Unmarshal([]byte(strings.TrimPrefix(strings.TrimSpace(line), "data: ")), &n)
		ready <- n
	}()
	hub.Publish(Notice{Job: "budget", At: time.Now(), Text: "balance"})
	select {
	case n := <-ready:
		if n.Job != "budget" {
			t.Fatalf("notice=%+v", n)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("push timed out")
	}
	response2, err := server.Client().Get(server.URL + "/v1/events")
	if err != nil {
		t.Fatal(err)
	}
	defer response2.Body.Close()
	line, _ := bufio.NewReader(response2.Body).ReadString('\n')
	if !strings.Contains(line, "balance") {
		t.Fatalf("replay=%q", line)
	}
}
