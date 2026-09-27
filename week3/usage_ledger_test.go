package agent

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUsageLedgerAggregatesHourAndLocalDay(t *testing.T) {
	root := t.TempDir()
	a := &Agent{store: Store{Dir: root}}
	now := time.Date(2026, 9, 27, 15, 0, 0, 0, time.Local)
	a.appendUsage(CallMetrics{StartedAt: now.Add(-30 * time.Minute), CostUSD: 0.01, Usage: Usage{TotalTokens: 100}})
	a.appendUsage(CallMetrics{StartedAt: now.Add(-2 * time.Hour), CostUSD: 0.02, Usage: Usage{TotalTokens: 200}})
	a.appendUsage(CallMetrics{StartedAt: now.Add(-24 * time.Hour), CostUSD: 0.03, Usage: Usage{TotalTokens: 300}})
	hour, day, err := a.UsageReport(now)
	if err != nil {
		t.Fatal(err)
	}
	if hour.Calls != 1 || hour.Tokens != 100 || day.Calls != 2 || day.Tokens != 300 {
		t.Fatalf("hour=%+v day=%+v", hour, day)
	}
	info, err := os.Stat(filepath.Join(root, "usage.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%v", info.Mode())
	}
}
