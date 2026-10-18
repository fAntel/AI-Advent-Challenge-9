package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Captured from the real failure: the quote exactly matched a decoded search
// row, but not the JSON representation previously stored as evidence text.
func TestCapturedSearchQuote(t *testing.T) {
	data, err := os.ReadFile("testdata/search-quote-regression.json")
	if err != nil {
		t.Fatal(err)
	}
	var captured struct {
		Raw      string     `json:"raw"`
		Evidence []Evidence `json:"evidence"`
	}
	if err = json.Unmarshal(data, &captured); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateAnswer(captured.Raw, captured.Evidence); err == nil || err.Error() != "unsupported quote e9" {
		t.Fatalf("did not reproduce original failure: %v", err)
	}
	var page struct {
		Results []string `json:"results"`
	}
	if err = json.Unmarshal([]byte(captured.Evidence[8].Text), &page); err != nil {
		t.Fatal(err)
	}
	captured.Evidence[8].Text = strings.Join(page.Results, "\n")
	if _, err = ValidateAnswer(captured.Raw, captured.Evidence); err != nil {
		t.Fatalf("unchanged captured answer rejected with plain evidence: %v", err)
	}
}

func TestDiscoveryEvidencePreservesSourceCharacters(t *testing.T) {
	e := fakeEngine(t)
	fs, root := newFS(t)
	s := Session{Root: root}
	line := "\tvalue := \"Lookup(42)\\nC:\\catalog\\lookup\" // café <tag> & data"
	put(t, filepath.Join(root, "quoted.go"), line+"\n")
	for _, name := range []string{"list_files", "search_files"} {
		t.Run(name, func(t *testing.T) {
			op := Operation{}
			args := map[string]any{}
			expected := "quoted.go"
			if name == "search_files" {
				args["query"] = "Lookup("
				expected = "quoted.go:1:" + line
			}
			raw, err := e.executeTool(context.Background(), fs, &s, &op, testCall("call", name, args), func() error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			var result struct {
				Evidence Evidence `json:"evidence"`
				HasMore  bool     `json:"has_more"`
				Next     int      `json:"next_offset"`
			}
			if err = json.Unmarshal([]byte(raw), &result); err != nil {
				t.Fatal(err)
			}
			if result.Evidence.Text != expected || result.HasMore || result.Next != 1 {
				t.Fatalf("incorrect tool result: %+v", result)
			}
			answer := Answer{Status: "answered", Answer: "Found [1].", Quotes: []Quote{{ID: result.Evidence.ID, Text: expected}}}
			encoded, _ := json.Marshal(answer)
			if _, err = ValidateAnswer(string(encoded), op.Evidence); err != nil {
				t.Fatal(err)
			}
			answer.Quotes[0].Text = expected + " fabricated"
			encoded, _ = json.Marshal(answer)
			if _, err = ValidateAnswer(string(encoded), op.Evidence); err == nil {
				t.Fatal("fabricated quote accepted")
			}
		})
	}
}
