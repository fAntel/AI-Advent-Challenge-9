package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveReportIsConfinedAndReplacesAtomically(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "reports")
	for _, name := range []string{"../escape.md", "/tmp/escape.md", "sub/report.md"} {
		if _, err := save(dir, name, "unsafe"); err == nil {
			t.Fatalf("accepted filename %q", name)
		}
	}
	path, err := save(dir, "report.md", "first")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := save(dir, "report.md", "second"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "second" {
		t.Fatalf("content=%q err=%v", data, err)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("permissions=%v err=%v", info, err)
	}
	if got := summarize([]map[string]any{{"id": "item-1", "name": "Desk lamp", "quantity": float64(1), "tags": []any{"Book"}}}); !strings.Contains(got, "Desk lamp") || !strings.Contains(got, "item-1") {
		t.Fatalf("summary=%q", got)
	}
}
