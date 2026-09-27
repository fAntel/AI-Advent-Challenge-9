package agent

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSchedulerRunsAndPersistsMCPJob(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "calls")
	t.Setenv("ADVENT_MCP_WRITE_FILE", marker)
	catalog, err := NewMCPCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close()
	if err := catalog.Add("fixture", "fixture", []string{fixtureBinary(t)}); err != nil {
		t.Fatal(err)
	}
	hub := NewEventHub()
	path := filepath.Join(root, "config", "schedules.toml")
	s, err := NewScheduler(path, catalog, hub, 10*time.Second, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local)
	s.now = func() time.Time { return now }
	if err := s.Add(context.Background(), ScheduleJob{Name: "sample", Server: "fixture", Tool: "write", ArgumentsJSON: "{}", Every: "1h", Notify: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.Add(context.Background(), ScheduleJob{Name: "bad", Server: "fixture", Tool: "read_00", ArgumentsJSON: "{}", Every: "1h"}); err == nil {
		t.Fatal("invalid MCP arguments were accepted")
	}
	now = now.Add(time.Hour + time.Second)
	s.runDue()
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "1\n" {
		t.Fatalf("calls=%q", data)
	}
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()
	select {
	case n := <-ch:
		if n.Job != "sample" || n.Text != "written" {
			t.Fatalf("notice=%+v", n)
		}
	default:
		t.Fatal("missing latest notice")
	}
	loaded, err := NewScheduler(path, catalog, NewEventHub(), 10*time.Second, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	jobs := loaded.List()
	if len(jobs) != 1 || jobs[0].LastResult != "written" || jobs[0].NextRun == "" {
		t.Fatalf("jobs=%+v", jobs)
	}
	if err := loaded.Remove("sample"); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "sample") {
		t.Fatalf("not removed: %s", data)
	}
}

func TestSchedulerCatchUpOnce(t *testing.T) {
	root := t.TempDir()
	marker := filepath.Join(root, "calls")
	t.Setenv("ADVENT_MCP_WRITE_FILE", marker)
	catalog, _ := NewMCPCatalog(root)
	defer catalog.Close()
	if err := catalog.Add("fixture", "fixture", []string{fixtureBinary(t)}); err != nil {
		t.Fatal(err)
	}
	s, err := NewScheduler(filepath.Join(root, "jobs.toml"), catalog, NewEventHub(), 10*time.Second, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	s.now = func() time.Time { return now }
	if err := s.Add(context.Background(), ScheduleJob{Name: "once", Server: "fixture", Tool: "write", Every: "1h"}); err != nil {
		t.Fatal(err)
	}
	now = now.Add(24 * time.Hour)
	s.runDue()
	s.runDue()
	data, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "1\n" {
		t.Fatalf("missed jobs replayed: %q", data)
	}
}

func TestSchedulerReloadRecalculatesEditedInterval(t *testing.T) {
	root := t.TempDir()
	catalog, err := NewMCPCatalog(root)
	if err != nil {
		t.Fatal(err)
	}
	defer catalog.Close()
	if err := catalog.Add("fixture", "fixture", []string{fixtureBinary(t)}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "schedules.toml")
	s, err := NewScheduler(path, catalog, NewEventHub(), 10*time.Second, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Add(context.Background(), ScheduleJob{Name: "budget", Server: "fixture", Tool: "write", Every: "1h"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(string(data), `every = "1h"`, `every = "5m"`, 1)
	if edited == string(data) {
		t.Fatal("interval not found in TOML")
	}
	if err := os.WriteFile(path, []byte(edited), 0600); err != nil {
		t.Fatal(err)
	}
	before := time.Now()
	jobs, err := s.Reload()
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Every != "5m" || jobs[0].ScheduledEvery != "5m0s" {
		t.Fatalf("jobs=%+v", jobs)
	}
	next, err := time.Parse(time.RFC3339Nano, jobs[0].NextRun)
	if err != nil {
		t.Fatal(err)
	}
	if next.Before(before.Add(5*time.Minute)) || next.After(time.Now().Add(5*time.Minute+5*time.Second)) {
		t.Fatalf("next_run=%v", next)
	}
	restarted, err := NewScheduler(path, catalog, NewEventHub(), 10*time.Second, time.Hour, nil)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.List()[0].NextRun != jobs[0].NextRun {
		t.Fatal("restart changed an unchanged next_run")
	}
}
