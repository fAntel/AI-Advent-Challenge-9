package main

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestSavePartialReport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "results", "run.json")
	r := report{Model: "test", Results: []result{{Answer: "partial", Error: "timeout"}}}
	if err := save(path, r); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got report
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Results) != 1 || got.Results[0].Answer != "partial" || got.Results[0].Error != "timeout" {
		t.Fatalf("%+v", got)
	}
	if err := save(path, report{Model: "replacement"}); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Model != "replacement" {
		t.Fatal("report not replaced")
	}
}

func TestArguments(t *testing.T) {
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"--help"}, 0}, {[]string{"demo", "--help"}, 0},
		{[]string{"unknown"}, 2}, {[]string{"demo", "--timeout", "0s"}, 2},
		{[]string{"demo", "--model", ""}, 2}, {[]string{"demo", "unexpected"}, 2},
	} {
		if code := run(context.Background(), tc.args, io.Discard, io.Discard); code != tc.code {
			t.Errorf("%v: code %d", tc.args, code)
		}
	}
}
