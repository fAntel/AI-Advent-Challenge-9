package agent

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"
)

func TestAPIClientExplainsMissingDaemon(t *testing.T) {
	client := NewAPIClient(filepath.Join("/private/tmp", "missing-agent-"+randomID()+".sock"), time.Second)
	_, err := client.List()
	if !errors.Is(err, ErrDaemonUnavailable) {
		t.Fatalf("error = %v", err)
	}
}

func TestRequireLocalRejectsOldOrCloudDaemon(t *testing.T) {
	for _, local := range []bool{false, true} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if local {
				w.Write([]byte(`{"provider_ollama":true}`))
			} else {
				w.Write([]byte(`{"rag":true}`))
			}
		}))
		client := &APIClient{HTTP: server.Client(), BaseURL: server.URL}
		err := client.RequireLocal()
		server.Close()
		if (err == nil) != local {
			t.Fatalf("local=%t error=%v", local, err)
		}
	}
}
