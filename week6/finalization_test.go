package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func capturedCitationFailure(t *testing.T) Operation {
	t.Helper()
	data, err := os.ReadFile("testdata/missing-citation-regression.json")
	if err != nil {
		t.Fatal(err)
	}
	var op Operation
	if err = json.Unmarshal(data, &op); err != nil {
		t.Fatal(err)
	}
	if _, err = ValidateAnswer(op.Raw, op.Evidence); err == nil || err.Error() != "invalid citation [12]" {
		t.Fatalf("failed to reproduce: %v", err)
	}
	return op
}

func TestFinalAnswerRepairsCapturedCitations(t *testing.T) {
	op := capturedCitationFailure(t)
	bad := op.Raw
	op.Error = ""
	e := fakeEngine(t)
	calls := 0
	saves := 0
	e.Complete = func(_ context.Context, purpose string, m []Message) (string, Call, error) {
		calls++
		if calls == 1 {
			return bad, Call{Purpose: purpose}, nil
		}
		if purpose != "answer_repair" || len(m) != 4 || !strings.Contains(m[3].Content, "invalid citation [12]") || !strings.Contains(m[3].Content, "invalid citation [11]") || !strings.Contains(m[3].Content, "invalid citation [8]") || !strings.Contains(m[3].Content, "invalid citation [13]") {
			t.Fatal("missing repair feedback")
		}
		return testFinal(m), Call{Purpose: purpose}, nil
	}
	answer, err := e.finalizeAnswer(context.Background(), &op, "Created the changelog.", func() error { saves++; return nil })
	if err != nil || answer.Status != "answered" || calls != 2 || op.FinalizationPending || len(op.AnswerAttempts) != 2 || op.AnswerAttempts[0].Error != "invalid citation [12]" || saves < 3 {
		t.Fatalf("repair: %+v %v", op, err)
	}
	if len(op.Tools) != 0 || len(op.Changes) != 0 {
		t.Fatal("repair performed tools")
	}
}

func TestAnswerOnlyRetryPreservesSavedFile(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprint(legacy), func(t *testing.T) {
			e := fakeEngine(t)
			root := repo(t)
			path := filepath.Join(root, "CHANGELOG.md")
			put(t, path, "already saved\n")
			session, err := e.Create(root)
			if err != nil {
				t.Fatal(err)
			}
			op := capturedCitationFailure(t)
			op.Question = "create CHANGELOG.md"
			op.FinalizationPending = !legacy
			op.Changes = []Change{{Path: "CHANGELOG.md", Status: "saved", AfterHash: hashText("already saved\n")}}
			session.Operations = append(session.Operations, op)
			if err = e.save(session); err != nil {
				t.Fatal(err)
			}
			e.ToolComplete = func(context.Context, []ToolMessage, bool) (ToolMessage, Call, error) {
				t.Fatal("retried file tools")
				return ToolMessage{}, Call{}, nil
			}
			e.Complete = func(_ context.Context, _ string, m []Message) (string, Call, error) { return testFinal(m), Call{}, nil }
			session, err = e.Ask(context.Background(), session.ID, "", true)
			if err != nil {
				t.Fatal(err)
			}
			result := session.Operations[1]
			if result.Answer == nil || len(result.Tools) != 0 || len(result.Changes) != 0 || !strings.Contains(result.Warning, "saved evidence") {
				t.Fatalf("unexpected retry: %+v", result)
			}
			data, err := os.ReadFile(path)
			if err != nil || string(data) != "already saved\n" {
				t.Fatal("saved file changed")
			}
			loaded, err := e.Get(session.ID)
			if err != nil || loaded.Operations[1].Answer == nil || loaded.Operations[0].Changes[0].Status != "saved" {
				t.Fatal("recovery not persisted")
			}
		})
	}
}

func TestFinalAnswerRepairLimitsAndFailure(t *testing.T) {
	for _, mode := range []string{"invalid", "provider", "cancelled"} {
		t.Run(mode, func(t *testing.T) {
			e := fakeEngine(t)
			op := capturedCitationFailure(t)
			bad := op.Raw
			calls := 0
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancelled" {
				cancel()
			}
			e.Complete = func(context.Context, string, []Message) (string, Call, error) {
				calls++
				if mode == "provider" {
					return "", Call{}, fmt.Errorf("offline")
				}
				return bad, Call{}, nil
			}
			_, err := e.finalizeAnswer(ctx, &op, "draft", func() error { return nil })
			want := 3
			if mode == "provider" {
				want = 1
			}
			if mode == "cancelled" {
				want = 0
			}
			if err == nil || calls != want || !op.FinalizationPending {
				t.Fatalf("unbounded or accepted: %d %v", calls, err)
			}
			if mode == "invalid" && (len(op.AnswerAttempts) != 3 || !strings.Contains(err.Error(), "/retry retries only the final answer")) {
				t.Fatal("missing recovery information")
			}
		})
	}
}

func TestMarkdownQuoteWrappingRestoredExactly(t *testing.T) {
	text := "Supplying credentials through chat sends them to the model and stores\nthe user message in session history."
	evidence := []Evidence{{Number: 1, ID: "e1", Source: "guide.md", Text: text}}
	a := Answer{Status: "answered", Answer: "Credentials enter history [1].", Quotes: []Quote{{ID: "e1", Text: strings.ReplaceAll(text, "\n", " ")}}}
	raw, _ := json.Marshal(a)
	if _, err := ValidateAnswer(string(raw), evidence); err == nil {
		t.Fatal("fixture must fail exact matching")
	}
	fixed := restoreMarkdownQuoteWrapping(string(raw), evidence)
	validated, err := ValidateAnswer(fixed, evidence)
	if err != nil || validated.Quotes[0].Text != text {
		t.Fatalf("%v %+v", err, validated)
	}
	a.Quotes[0].Text = strings.ReplaceAll(a.Quotes[0].Text, "stores", "deletes")
	bad, _ := json.Marshal(a)
	if _, err := ValidateAnswer(restoreMarkdownQuoteWrapping(string(bad), evidence), evidence); err == nil {
		t.Fatal("paraphrase accepted")
	}
	evidence[0].Source = "code.go"
	if restoreMarkdownQuoteWrapping(string(raw), evidence) != string(raw) {
		t.Fatal("code whitespace changed")
	}
}
