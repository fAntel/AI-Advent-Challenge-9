package agent

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestDailyTimestampsInclusiveAndUTC(t *testing.T) {
	now := time.Date(2026, 10, 19, 13, 0, 0, 0, time.FixedZone("MSK", 3*3600))
	rows, err := dailyTimestamps(now, 30, "10:00:00")
	if err != nil || len(rows) != 31 || rows[0].Date != "2026-09-19" || rows[30].Date != "2026-10-19" {
		t.Fatalf("%v %+v", err, rows)
	}
	if rows[0].Timestamp != 1789812000 || rows[30].Timestamp != 1792404000 {
		t.Fatalf("incorrect timestamps: %+v", rows)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].Timestamp-rows[i-1].Timestamp != 86400 {
			t.Fatal("not daily")
		}
	}
	if _, err := dailyTimestamps(now, 366, "10:00:00"); err == nil {
		t.Fatal("unbounded")
	}
	if _, err := dailyTimestamps(now, 30, "bad"); err == nil {
		t.Fatal("invalid time")
	}
}
func TestWeightChartUsesOnlySelectedDownloadedValues(t *testing.T) {
	body := `{"records":[{"id":2,"measurementTime":1792404000,"weight":80.7,"additionalData":{"muscleMass":34.4}},{"id":99,"measurementTime":1792404000,"weight":99},{"id":1,"measurementTime":1789812000,"weight":80,"additionalData":{"muscleMass":34}}]}`
	ev := []Evidence{{ID: "e1", Source: "mcp://healbe/healbe_download_weights_by_measurement_time", Text: body}}
	chart, err := weightChart(ev, []string{"e1"}, []int64{1, 2})
	if err != nil || !strings.Contains(chart, "2026-09-19 to 2026-10-19") || strings.Contains(chart, "99.00") || !strings.Contains(chart, "80.70") || !strings.Contains(chart, "34.40") {
		t.Fatalf("%v %s", err, chart)
	}
	for _, r := range chart {
		if r > 127 {
			t.Fatal("not ASCII")
		}
	}
	if _, err := weightChart(ev, []string{"e1"}, []int64{3}); err == nil {
		t.Fatal("missing data accepted")
	}
	ev[0].Source = "fixture.md"
	if _, err := weightChart(ev, []string{"e1"}, []int64{1}); err == nil {
		t.Fatal("unverified data accepted")
	}
}
func TestCapturedUploadedDatesAndChart(t *testing.T) {
	// Exact endpoints from the observed bad run: these are September 13–October 13,
	// regardless of the model's incorrect September 19–October 19 labels.
	if got := time.Unix(1789293600, 0).UTC().Format("2006-01-02"); got != "2026-09-13" {
		t.Fatal(got)
	}
	if got := time.Unix(1791885600, 0).UTC().Format("2006-01-02"); got != "2026-10-13" {
		t.Fatal(got)
	}
	a := Answer{Status: "answered", Answer: "Verified [1].", Quotes: []Quote{{ID: "e1", Text: "Downloaded weights"}}}
	ev := []Evidence{{ID: "e1", Source: "chart://weights", Text: "Downloaded weights\n2026-09-13 80.70 |#\n"}}
	b, _ := json.Marshal(a)
	if _, err := ValidateAnswer(string(b), ev); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(Render(a, ev), "```text\n"+ev[0].Text+"```") {
		t.Fatal("chart not rendered exactly")
	}
}
