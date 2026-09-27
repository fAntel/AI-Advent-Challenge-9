package main

import (
	"strings"
	"testing"
)

func TestServicePlistKeepsKeyOutAndEscapesPaths(t *testing.T) {
	content := servicePlist("/tmp/agent & tools/advent-agentd", "/tmp/error <log>.txt")
	for _, want := range []string{"<key>RunAtLoad</key><true/>", "<key>KeepAlive</key><true/>", "agent &amp; tools", "error &lt;log&gt;.txt"} {
		if !strings.Contains(content, want) {
			t.Fatalf("missing %q", want)
		}
	}
	if strings.Contains(content, "DEEPSEEK_API_KEY") {
		t.Fatal("plist contains key environment field")
	}
}
