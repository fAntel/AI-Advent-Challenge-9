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
		},
		Metrics: agent.Metrics{Requests: 2, Usage: agent.Usage{PromptTokens: 220, CompletionTokens: 30, TotalTokens: 250}, CostUSD: 0.0001},
	}
	var output bytes.Buffer
	printSessionStats(&output, &s)
	for _, want := range []string{
		"latest API call: input context=200",
		"model answer=25",
		"context window: 200 / 1000 (20.00%)",
		"whole dialog: calls=2, cumulative input=220, model answers=30, total=250",
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
