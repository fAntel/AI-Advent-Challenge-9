package main

import (
	agent "advent-agent"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func response(text string) *http.Response {
	return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(text)), Header: http.Header{}}
}

func TestCaptureStoresBalanceAndReportsUsage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "budget", "snapshots.json")
	b := &budget{path: path, key: "test", http: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer test" {
			t.Fatal("missing auth")
		}
		return response(`{"is_available":true,"balance_infos":[{"currency":"USD","total_balance":"12.50"}]}`), nil
	})}, api: &agent.APIClient{BaseURL: "http://unix", HTTP: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		return response(`{"last_hour":{"calls":1,"tokens":50,"estimated_cost_usd":0.01},"today":{"calls":2,"tokens":90,"estimated_cost_usd":0.02}}`), nil
	})}}}
	result, err := b.capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, "12.50 USD") || !strings.Contains(result, "$0.010000 USD") {
		t.Fatalf("report=%s", result)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("mode=%v", info.Mode())
	}
	list, err := b.samples()
	if err != nil || len(list) != 1 {
		t.Fatalf("samples=%+v err=%v", list, err)
	}
}
func TestChangeUsesPriorSampleAndLabelsIncrease(t *testing.T) {
	now := time.Now()
	list := []sample{{At: now.Add(-2 * time.Hour), Balances: []balanceInfo{{"USD", "10.00"}}}, {At: now, Balances: []balanceInfo{{"USD", "15.00"}}}}
	if got := change(list, now.Add(-time.Hour)); !strings.Contains(got, "5.00 USD increase/adjustment") {
		t.Fatal(got)
	}
}
