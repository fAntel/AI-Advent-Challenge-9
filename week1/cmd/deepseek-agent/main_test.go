package main

import (
	"bytes"
	agent "deepseek-agent"
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
		"whole dialog: calls=3, cumulative input=270, model answers=40, total=310",
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
