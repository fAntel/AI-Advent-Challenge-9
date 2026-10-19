package agent

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type dailyPoint struct {
	Date      string `json:"date"`
	Timestamp int64  `json:"measurementTime"`
}

func dailyTimestamps(now time.Time, days int, at string) ([]dailyPoint, error) {
	if days < 0 || days > 365 {
		return nil, fmt.Errorf("days_prior must be 0..365")
	}
	clock, err := time.Parse("15:04:05", at)
	if err != nil {
		return nil, fmt.Errorf("time_utc must be HH:MM:SS")
	}
	now = now.UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), clock.Hour(), clock.Minute(), clock.Second(), 0, time.UTC)
	points := make([]dailyPoint, 0, days+1)
	for d := -days; d <= 0; d++ {
		date := today.AddDate(0, 0, d)
		points = append(points, dailyPoint{date.Format("2006-01-02"), date.Unix()})
	}
	return points, nil
}

type chartWeight struct {
	ID         int64   `json:"id"`
	Timestamp  int64   `json:"measurementTime"`
	Weight     float64 `json:"weight"`
	Additional struct {
		Muscle *float64 `json:"muscleMass"`
	} `json:"additionalData"`
}

func weightChart(evidence []Evidence, evidenceIDs []string, recordIDs []int64) (string, error) {
	if len(recordIDs) == 0 || len(recordIDs) > 100 {
		return "", fmt.Errorf("select 1..100 record_ids from upload receipts or downloads")
	}
	wanted := map[int64]bool{}
	for _, id := range recordIDs {
		if id <= 0 || wanted[id] {
			return "", fmt.Errorf("record_ids must be positive and unique")
		}
		wanted[id] = true
	}
	sources := map[string]bool{}
	for _, id := range evidenceIDs {
		sources[id] = true
	}
	found := map[int64]chartWeight{}
	for _, ev := range evidence {
		if !sources[ev.ID] {
			continue
		}
		if !strings.HasPrefix(ev.Source, "mcp://healbe/healbe_download_weights_by_") {
			return "", fmt.Errorf("chart requires downloaded weight evidence")
		}
		var page struct {
			Records []chartWeight `json:"records"`
		}
		if json.Unmarshal([]byte(ev.Text), &page) != nil {
			return "", fmt.Errorf("invalid downloaded evidence")
		}
		delete(sources, ev.ID)
		for _, w := range page.Records {
			if wanted[w.ID] {
				found[w.ID] = w
			}
		}
	}
	if len(sources) > 0 || len(found) != len(wanted) {
		return "", fmt.Errorf("selected records missing; download additional pages first")
	}
	rows := make([]chartWeight, 0, len(found))
	for _, w := range found {
		rows = append(rows, w)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Timestamp < rows[j].Timestamp })
	var b strings.Builder
	fmt.Fprintf(&b, "Downloaded weights: %d records; %s to %s (UTC).\n", len(rows), time.Unix(rows[0].Timestamp, 0).UTC().Format("2006-01-02"), time.Unix(rows[len(rows)-1].Timestamp, 0).UTC().Format("2006-01-02"))
	for _, muscle := range []bool{false, true} {
		label := "Weight"
		if muscle {
			label = "Muscle mass"
		}
		minimum, maximum := math.Inf(1), math.Inf(-1)
		for _, w := range rows {
			v := w.Weight
			if muscle {
				if w.Additional.Muscle == nil {
					continue
				}
				v = *w.Additional.Muscle
			}
			minimum = math.Min(minimum, v)
			maximum = math.Max(maximum, v)
		}
		if math.IsInf(minimum, 0) {
			fmt.Fprintf(&b, "\n%s: no downloaded values.\n", label)
			continue
		}
		fmt.Fprintf(&b, "\n%s (kg); bars scaled from %.2f to %.2f, not from zero.\n", label, minimum, maximum)
		for _, w := range rows {
			date := time.Unix(w.Timestamp, 0).UTC().Format("2006-01-02")
			v := w.Weight
			if muscle {
				if w.Additional.Muscle == nil {
					fmt.Fprintf(&b, "%s      n/a |\n", date)
					continue
				}
				v = *w.Additional.Muscle
			}
			width := 1
			if maximum > minimum {
				width += int(math.Round((v - minimum) / (maximum - minimum) * 39))
			}
			fmt.Fprintf(&b, "%s %8.2f |%s\n", date, v, strings.Repeat("#", width))
		}
	}
	return b.String(), nil
}
