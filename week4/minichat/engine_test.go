package minichat

import (
	agent "advent-agent"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const evidence = `[{"id":"c1","document_id":"doc","strategy":"semantic","text":"A Kotlin declaration can be exported.","title":"Fixture guide","source":"https://example.org/guide","sections":["Export"],"line_start":1,"line_end":1,"availability":"unknown","score":0.9}]`
const validAnswer = `{"status":"answered","answer":"A Kotlin declaration can be exported [1].","quotes":[{"chunk_id":"c1","text":"A Kotlin declaration can be exported."}],"clarification":""}`

type fakeProvider struct {
	calls   []agent.CompletionRequest
	failure string
	invalid bool
	extract func(*Turn) Extraction
}

func (f *fakeProvider) Complete(_ context.Context, r agent.CompletionRequest) (agent.CompletionResponse, error) {
	f.calls = append(f.calls, r)
	purpose := "answer"
	answer := validAnswer
	if r.Messages[0].Content == memoryPrompt {
		purpose = "memory"
		var input struct {
			Current struct {
				ID      string `json:"id"`
				Message string `json:"message"`
			} `json:"current_turn"`
		}
		if err := json.Unmarshal([]byte(r.Messages[1].Content), &input); err != nil {
			return agent.Completion{}, err
		}
		x := Extraction{Platform: "js", Query: input.Current.Message, Changes: []Change{}}
		if f.extract != nil {
			x = f.extract(&Turn{ID: input.Current.ID, Question: input.Current.Message})
		}
		answer = jsonText(x)
	} else if strings.HasPrefix(r.Messages[0].Content, "Score each candidate") {
		purpose = "retrieval"
		score := 3
		if strings.Contains(r.Messages[1].Content, "Kotlin 3.0") {
			score = 0
		}
		answer = jsonText(map[string]any{"scores": []map[string]any{{"id": "c1", "score": score}}})
	} else if f.invalid {
		answer = strings.Replace(validAnswer, "A Kotlin declaration can be exported.\"}", "fabricated.\"}", 1)
	}
	if purpose == f.failure {
		f.failure = ""
		return agent.Completion{}, errors.New("simulated outage")
	}
	return agent.Completion{Answer: answer, Call: agent.CallMetrics{FinishReason: "stop"}}, nil
}
func fixture(t *testing.T) (*Engine, Settings) {
	t.Helper()
	dir := t.TempDir()
	exe, db := filepath.Join(dir, "indexer"), filepath.Join(dir, "index.sqlite")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$0.args\"\nprintf '%s' '"+evidence+"'\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(db, []byte("fixture index"), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := Open(filepath.Join(dir, "chat.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return &Engine{Store: store, Provider: &fakeProvider{}}, Defaults(agent.RAGConfig{Executable: exe, Database: db})
}
func create(t *testing.T, e *Engine, settings Settings) *Session {
	t.Helper()
	s, err := e.Store.Create(settings)
	if err != nil {
		t.Fatal(err)
	}
	return s
}
func ask(t *testing.T, e *Engine, id, q string) *Session {
	t.Helper()
	s, err := e.Ask(context.Background(), id, q)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestTwoTwelveQuestionDialogs(t *testing.T) {
	data, err := os.ReadFile("../mini-chat-suite.json")
	if err != nil {
		t.Fatal(err)
	}
	var suite Suite
	if err = json.Unmarshal(data, &suite); err != nil {
		t.Fatal(err)
	}
	for _, d := range suite.Dialogs {
		t.Run(d.Name, func(t *testing.T) {
			e, settings := fixture(t)
			fake := e.Provider.(*fakeProvider)
			fake.extract = func(turn *Turn) Extraction {
				x := Extraction{Platform: "js", Query: turn.Question, Changes: []Change{}}
				add := func(kind, key, value string, replace bool) {
					x.Changes = append(x.Changes, Change{Kind: kind, Key: key, Value: value, Turn: turn.ID, Quote: turn.Question, Replace: replace})
				}
				switch turn.Question {
				case d.Questions[0]:
					add("goal", "", d.Name, false)
					add("constraint", "examples", "Kotlin-only", false)
					add("definition", "client", "original caller", false)
				case d.Questions[6]:
					add("constraint", "examples", "both languages", true)
				case d.Questions[7]:
					add("constraint", "examples", "contradicting examples", false)
				}
				return x
			}
			s := create(t, e, settings)
			for i, q := range d.Questions {
				if i == 6 {
					path := e.Store.path
					if err := e.Store.Close(); err != nil {
						t.Fatal(err)
					}
					e.Store, err = Open(path)
					if err != nil {
						t.Fatal(err)
					}
					defer e.Store.Close()
				}
				got := ask(t, e, s.ID, q)
				turn := got.Turns[i]
				if len(got.Turns) != i+1 || len(got.Questions()) != i+1 || got.Memory.Goal.Value != d.Name || got.Memory.Definitions["client"].Value != "original caller" {
					t.Fatal("lost original memory or question history")
				}
				expected := "Kotlin-only"
				if i >= 6 {
					expected = "both languages"
				}
				if got.Memory.Constraints["examples"].Value != expected {
					t.Fatal("lost or silently replaced constraint")
				}
				if turn.Evidence == nil || turn.Evidence.Query != q || turn.Preparation == nil || !turn.Preparation.Retrieved {
					t.Fatal("turn did not retrieve")
				}
				args, err := os.ReadFile(settings.Retrieval.Executable + ".args")
				if err != nil || !strings.Contains(string(args), "--platform\njs\n") {
					t.Fatal("missing platform filter", string(args), err)
				}
				if !strings.Contains(turn.Answer, "Sources") {
					t.Fatal("missing Sources section")
				}
				if i == 7 {
					if len(turn.Conflicts) != 1 || turn.Result != nil {
						t.Fatal("ambiguous conflict not clarified")
					}
				} else if i == 10 {
					if turn.Result.Status != "unknown" {
						t.Fatal("unsupported question answered")
					}
				} else {
					if _, err := agent.ValidateRAGAnswer(validAnswer, turn.Evidence); err != nil || len(turn.Result.Sources) != 1 {
						t.Fatal("invalid citation", err)
					}
					last := turn.Calls[len(turn.Calls)-1]
					if last.Purpose != "answer" {
						t.Fatal("no answer call")
					}
					combined := jsonText(last.Messages)
					for _, want := range []string{d.Name, "original caller", expected, d.Questions[0]} {
						if !strings.Contains(combined, want) {
							t.Fatalf("answer lost %q", want)
						}
					}
				}
				if len(got.Recent()) > 8 {
					t.Fatal("unbounded recent history")
				}
			}
			other := create(t, e, settings)
			if other.Memory.Goal.Value != "" || len(other.Turns) != 0 {
				t.Fatal("session leakage")
			}
		})
	}
}

func TestRetryResumeAndDiscard(t *testing.T) {
	for _, stage := range []string{"memory", "retrieval", "answer"} {
		t.Run(stage, func(t *testing.T) {
			e, settings := fixture(t)
			fake := e.Provider.(*fakeProvider)
			fake.failure = stage
			s := create(t, e, settings)
			if _, err := e.Ask(context.Background(), s.ID, "How to export?"); err == nil {
				t.Fatal("expected failure")
			}
			failed, _ := e.Store.Load(s.ID)
			if failed.Pending() == nil || failed.Turns[0].Answer != "" {
				t.Fatal("failure published")
			}
			if _, err := e.Ask(context.Background(), s.ID, "another"); err == nil {
				t.Fatal("accepted while pending")
			}
			if err := e.Correct(s.ID, "goal", "", "new", false); err == nil {
				t.Fatal("edited while pending")
			}
			if stage == "answer" {
				if err := os.WriteFile(settings.Retrieval.Executable, []byte("#!/bin/sh\nexit 99\n"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			path := e.Store.path
			e.Store.Close()
			var err error
			e.Store, err = Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer e.Store.Close()
			got, err := e.Retry(context.Background(), s.ID)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Turns) != 1 || got.Pending() != nil || got.Turns[0].State != "completed" {
				t.Fatal("retry duplicated or lost turn")
			}
		})
	}
	e, settings := fixture(t)
	s := create(t, e, settings)
	if err := e.Correct(s.ID, "goal", "", "old", false); err != nil {
		t.Fatal(err)
	}
	fake := e.Provider.(*fakeProvider)
	fake.extract = func(t *Turn) Extraction {
		return Extraction{Platform: "js", Query: t.Question, Changes: []Change{{Kind: "goal", Value: "new", Turn: t.ID, Quote: t.Question, Replace: true}}}
	}
	fake.failure = "answer"
	if _, err := e.Ask(context.Background(), s.ID, "change goal to new"); err == nil {
		t.Fatal("expected failure")
	}
	if err := e.Discard(s.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := e.Store.Load(s.ID)
	if got.Memory.Goal.Value != "old" || got.Turns[0].State != "discarded" || len(got.Questions()) != 0 {
		t.Fatal("discard lost audit or failed rollback")
	}
}
func TestMalformedExtractionAndGrounding(t *testing.T) {
	turn := &Turn{ID: "current", Question: "Use Kotlin"}
	for _, x := range []Extraction{
		{Query: "q", Changes: []Change{{Kind: "constraint", Key: "language", Value: "Kotlin", Turn: "other", Quote: "Use Kotlin"}}},
		{Query: "q", Changes: []Change{{Kind: "constraint", Key: "language", Value: "Kotlin", Turn: "current", Quote: "invented"}}},
		{Query: "q", Changes: []Change{{Kind: "constraint", Key: "language", Value: "", Turn: "current", Quote: "Use Kotlin"}}},
		{Query: "q", Changes: nil},
	} {
		if _, err := decodeExtraction(jsonText(x), turn); err == nil {
			t.Fatal("accepted invalid extraction")
		}
	}
	e, settings := fixture(t)
	fake := e.Provider.(*fakeProvider)
	fake.invalid = true
	s := create(t, e, settings)
	got, err := e.Ask(context.Background(), s.ID, "How to export?")
	if err == nil {
		t.Fatal("accepted invented quote")
	}
	if got.Turns[0].Answer != "" || got.Turns[0].Result != nil {
		t.Fatal("published invalid answer")
	}
	fake.invalid = false
	got, err = e.Retry(context.Background(), s.ID)
	if err != nil || got.Turns[0].State != "completed" {
		t.Fatal(err)
	}
}
func TestLockingManualCorrectionsAndContextLimit(t *testing.T) {
	e, settings := fixture(t)
	s := create(t, e, settings)
	unlock, err := e.Store.lock(s.ID)
	if err != nil {
		t.Fatal(err)
	}
	other, err := Open(e.Store.path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	if release, err := other.lock(s.ID); err == nil {
		release()
		t.Fatal("two writers accepted")
	}
	unlock()
	for _, c := range []struct {
		kind, key, val string
		remove         bool
	}{{"goal", "", "Build a bridge", false}, {"constraint", "language", "Kotlin", false}, {"definition", "client", "Swift", false}, {"constraint", "language", "", true}} {
		if err := e.Correct(s.ID, c.kind, c.key, c.val, c.remove); err != nil {
			t.Fatal(err)
		}
	}
	got, _ := e.Store.Load(s.ID)
	if len(got.Memory.Constraints) != 0 || got.Memory.Definitions["client"].Value != "Swift" || len(got.Revisions) != 4 {
		t.Fatal("manual corrections not persisted")
	}
	// Context exhaustion is explicit; neither the goal nor transcript is truncated.
	for i := 0; i < 9; i++ {
		got.Turns = append(got.Turns, &Turn{ID: ID(), State: "completed", Question: strings.Repeat("x", 16000), Answer: "old"})
	}
	if err := e.Store.Save(got); err != nil {
		t.Fatal(err)
	}
	_, err = e.Ask(context.Background(), s.ID, "Follow up")
	if err == nil || !strings.Contains(err.Error(), "nothing was silently dropped") {
		t.Fatal(err)
	}
	got, _ = e.Store.Load(s.ID)
	if len(got.Turns) != 10 || got.Memory.Goal.Value != "Build a bridge" {
		t.Fatal("context silently removed")
	}
}

func TestEvaluationCheckpoints(t *testing.T) {
	e, settings := fixture(t)
	output := filepath.Join(t.TempDir(), "run.json")
	if err := Evaluate(context.Background(), e, settings, "../mini-chat-suite.json", output, io.Discard); err != nil {
		t.Fatal(err)
	}
	calls := len(e.Provider.(*fakeProvider).calls)
	if err := Evaluate(context.Background(), e, settings, "../mini-chat-suite.json", output, io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(e.Provider.(*fakeProvider).calls) != calls {
		t.Fatal("completed evaluation repeated calls")
	}
	settings.Generation.Model = agent.ProModel
	if err := Evaluate(context.Background(), e, settings, "../mini-chat-suite.json", output, io.Discard); err == nil {
		t.Fatal("accepted changed settings")
	}
}
