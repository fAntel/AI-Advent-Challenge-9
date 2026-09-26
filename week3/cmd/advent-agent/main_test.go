package main

import (
	agent "advent-agent"
	"bytes"
	"strings"
	"testing"
)

func TestSessionStatsShowLatestCallDialogTotalsAndGrowth(t *testing.T) {
	s := agent.Session{
		ContextWindowTokens:     1_000,
		TokenAccountingComplete: true,
		Calls: []agent.CallMetrics{
			{Usage: agent.Usage{PromptTokens: 20, CompletionTokens: 5, TotalTokens: 25}, FinishReason: "stop"},
			{Usage: agent.Usage{PromptTokens: 200, PromptCacheHitTokens: 30, PromptCacheMissTokens: 170, CompletionTokens: 25, TotalTokens: 225}, FinishReason: "stop"},
			{Usage: agent.Usage{PromptTokens: 50, CompletionTokens: 10, TotalTokens: 60}, CostUSD: 0.00002, FinishReason: "stop", Purpose: "summary"},
		},
		Metrics:            agent.Metrics{Requests: 3, Usage: agent.Usage{PromptTokens: 270, CompletionTokens: 40, TotalTokens: 310}, CostUSD: 0.00012},
		Summary:            "The user prefers concise answers.",
		SummaryTokens:      10,
		SummarizedMessages: 5,
		Messages:           []agent.Message{{Role: "user", Content: "recent"}, {Role: "assistant", Content: "answer"}},
	}
	var output bytes.Buffer
	printSessionStats(&output, &s)
	for _, want := range []string{
		"latest answer request: input context=200",
		"model answer=25",
		"context window: 200 / 1000 (20.00%)",
		"whole dialog: calls=3, cumulative input=270, model output=40, total=310",
		"history memory: summarized=5 messages, verbatim=2 messages",
		"compression overhead: calls=1, input=50, output=10, total=60",
		"input-context growth: 20 -> 200 (+180 tokens)",
	} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("output does not contain %q:\n%s", want, output.String())
		}
	}
}

func TestSessionStatsExplainTruncation(t *testing.T) {
	s := agent.Session{ContextWindowTokens: 100, Calls: []agent.CallMetrics{{Usage: agent.Usage{PromptTokens: 95}, FinishReason: "length"}}}
	var output bytes.Buffer
	printSessionStats(&output, &s)
	if !strings.Contains(output.String(), "answer was truncated") {
		t.Fatalf("output=%s", output.String())
	}
}

func TestTaskStateDisplaysExpectedActionAndHistory(t *testing.T) {
	s := agent.Session{Task: &agent.TaskState{
		ID: "task-1", Objective: "ship it", Phase: agent.TaskPhaseExecution, Status: agent.TaskStatusPaused,
		ExpectedAction: "use /continue, provide feedback, or use /back",
		Attempts:       []agent.TaskAttempt{{Phase: agent.TaskPhasePlanning, Status: "completed", Superseded: true}, {Phase: agent.TaskPhasePlanning, Status: "completed"}},
	}}
	var output bytes.Buffer
	printTaskState(&output, &s, true)
	for _, want := range []string{"objective: ship it", "state: execution", "status: paused", "expected action: use /continue", "planning — completed, superseded", "planning — completed, authoritative"} {
		if !strings.Contains(output.String(), want) {
			t.Fatalf("output does not contain %q:\n%s", want, output.String())
		}
	}
}

func TestSessionLifecycleLabelDistinguishesTasksLegacyAndEmptySessions(t *testing.T) {
	tests := []struct {
		name    string
		session agent.Session
		label   string
		preview string
	}{
		{
			name:    "paused task",
			session: agent.Session{Task: &agent.TaskState{Objective: "write Fibonacci", Phase: agent.TaskPhaseExecution, Status: agent.TaskStatusPaused}, Operation: &agent.Operation{State: "completed"}},
			label:   "execution/paused",
			preview: "write Fibonacci",
		},
		{
			name:    "finished task",
			session: agent.Session{Task: &agent.TaskState{Objective: "finished", Phase: agent.TaskPhaseDone, Status: agent.TaskStatusTerminal}, Operation: &agent.Operation{State: "completed"}},
			label:   "done/terminal",
			preview: "finished",
		},
		{
			name:    "legacy conversation",
			session: agent.Session{Operation: &agent.Operation{State: "completed"}, Messages: []agent.Message{{Role: "user", Content: "old question"}}},
			label:   "legacy/completed",
			preview: "old question",
		},
		{
			name:    "empty session",
			session: agent.Session{},
			label:   "empty",
			preview: "(no task started)",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := sessionLifecycleLabel(test.session); got != test.label {
				t.Fatalf("label=%q want=%q", got, test.label)
			}
			if got := sessionListPreview(test.session); got != test.preview {
				t.Fatalf("preview=%q want=%q", got, test.preview)
			}
		})
	}
}
