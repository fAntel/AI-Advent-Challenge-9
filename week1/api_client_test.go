package agent

import (
	"errors"
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
