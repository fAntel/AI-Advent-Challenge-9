package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testAPIKey   = "super-secret-test-key"
	testEndpoint = "https://example.test/chat/completions"
)

func TestOneShotRequestIsUnchanged(t *testing.T) {
	client := newFakeClient(answerResponse("The answer"))
	code, stdout, stderr := runTest(t, []string{"--prompt", "from flag"}, "ignored stdin", false, client)
	if code != 0 || stdout != "The answer\n" || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(client.requests) != 1 {
		t.Fatalf("request count = %d", len(client.requests))
	}
	request := client.requests[0]
	if request.Model != defaultModel || request.Thinking.Type != "disabled" || request.ReasoningEffort != "" || request.Stream {
		t.Errorf("unexpected request settings: %+v", request)
	}
	if request.Temperature != nil || strings.Contains(client.bodies[0], `"temperature"`) {
		t.Errorf("omitted temperature was sent: request=%+v body=%s", request, client.bodies[0])
	}
	assertMessages(t, request.Messages, []chatMessage{{Role: "user", Content: "from flag"}})
	if got := client.headers[0].Get("Authorization"); got != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q", got)
	}
	if got := client.headers[0].Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
}

func TestModelAndReasoningFlags(t *testing.T) {
	tests := []struct {
		name              string
		args              []string
		wantModel         string
		wantThinking      string
		wantReasoning     string
		wantReasoningJSON bool
	}{
		{name: "flash without reasoning", args: []string{"-m", flashModel, "-r", "none"}, wantModel: flashModel, wantThinking: "disabled"},
		{name: "pro low", args: []string{"--model", proModel, "--reasoning", "low"}, wantModel: proModel, wantThinking: "enabled", wantReasoning: "low", wantReasoningJSON: true},
		{name: "pro high", args: []string{"--model", proModel, "--reasoning", "high"}, wantModel: proModel, wantThinking: "enabled", wantReasoning: "high", wantReasoningJSON: true},
		{name: "pro max", args: []string{"--model", proModel, "--reasoning", "max"}, wantModel: proModel, wantThinking: "enabled", wantReasoning: "max", wantReasoningJSON: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(answerResponse("ok"))
			args := append(tt.args, "-p", "question")
			code, _, stderr := runTest(t, args, "", false, client)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			request := client.requests[0]
			if request.Model != tt.wantModel || request.Thinking.Type != tt.wantThinking || request.ReasoningEffort != tt.wantReasoning {
				t.Errorf("request settings = %+v", request)
			}
			if got := strings.Contains(client.bodies[0], `"reasoning_effort"`); got != tt.wantReasoningJSON {
				t.Errorf("reasoning_effort presence = %v, want %v: %s", got, tt.wantReasoningJSON, client.bodies[0])
			}
		})
	}
}

func TestStatsReportUsesAPIUsageAndPeakPricing(t *testing.T) {
	usage := tokenUsage{
		PromptTokens:          300,
		PromptCacheHitTokens:  100,
		PromptCacheMissTokens: 200,
		CompletionTokens:      300,
		TotalTokens:           600,
	}
	usage.CompletionTokensDetails.ReasoningTokens = 250
	client := newFakeClient(answerResponseWithUsage("answer", usage))
	started := time.Date(2026, time.September, 7, 2, 0, 0, 0, time.UTC)
	times := []time.Time{started, started.Add(1234 * time.Millisecond)}
	var stdout, stderr bytes.Buffer
	code := run([]string{"--stats", "-p", "question"}, strings.NewReader(""), &stdout, &stderr, dependencies{
		client: client, endpoint: testEndpoint,
		getenv: func(string) string { return testAPIKey },
		now:    sequenceClock(times...),
	})
	if code != 0 || stdout.String() != "answer\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"model: deepseek-v4-flash",
		"reasoning: none",
		"requests: 1",
		"answer time: 1.234s",
		"input tokens: 300 (cache hit: 100, cache miss: 200)",
		"output tokens: 300 (reasoning: 250)",
		"total tokens: 600",
		"pricing: peak",
		"estimated cost: $0.00048540 USD",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestStatsAggregateMultipleRequests(t *testing.T) {
	first := tokenUsage{PromptTokens: 10, PromptCacheMissTokens: 10, CompletionTokens: 20, TotalTokens: 30}
	first.CompletionTokensDetails.ReasoningTokens = 5
	second := tokenUsage{PromptTokens: 30, PromptCacheHitTokens: 20, PromptCacheMissTokens: 10, CompletionTokens: 40, TotalTokens: 70}
	second.CompletionTokensDetails.ReasoningTokens = 15
	client := newFakeClient(
		answerResponseWithUsage("generated prompt", first),
		answerResponseWithUsage("final", second),
	)
	start := time.Date(2026, time.September, 6, 12, 0, 0, 0, time.UTC)
	var stdout, stderr bytes.Buffer
	code := run([]string{"--stats", "--model", proModel, "--reasoning", "max", "--approach", "self-prompt", "-p", "question"}, strings.NewReader(""), &stdout, &stderr, dependencies{
		client: client, endpoint: testEndpoint,
		getenv: func(string) string { return testAPIKey },
		now: sequenceClock(
			start, start.Add(time.Second),
			start.Add(2*time.Second), start.Add(4*time.Second),
		),
	})
	if code != 0 || stdout.String() != "final\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"requests: 2",
		"answer time: 3s",
		"input tokens: 40 (cache hit: 20, cache miss: 20)",
		"output tokens: 60 (reasoning: 20)",
		"total tokens: 100",
		"pricing: off-peak",
		"estimated cost: $0.00013244 USD",
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestPricingBandAt(t *testing.T) {
	tests := []struct {
		name string
		at   time.Time
		want pricingBand
	}{
		{name: "weekday before first peak", at: time.Date(2026, 9, 7, 0, 59, 59, 0, time.UTC), want: pricingOffPeak},
		{name: "first peak starts", at: time.Date(2026, 9, 7, 1, 0, 0, 0, time.UTC), want: pricingPeak},
		{name: "first peak ends", at: time.Date(2026, 9, 7, 4, 0, 0, 0, time.UTC), want: pricingOffPeak},
		{name: "second peak starts", at: time.Date(2026, 9, 7, 6, 0, 0, 0, time.UTC), want: pricingPeak},
		{name: "second peak ends", at: time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC), want: pricingOffPeak},
		{name: "weekend", at: time.Date(2026, 9, 6, 2, 0, 0, 0, time.UTC), want: pricingOffPeak},
		{name: "convert to UTC", at: time.Date(2026, 9, 7, 5, 0, 0, 0, time.FixedZone("UTC+3", 3*60*60)), want: pricingPeak},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pricingBandAt(tt.at); got != tt.want {
				t.Errorf("pricingBandAt(%s) = %s, want %s", tt.at, got, tt.want)
			}
		})
	}
}

func TestEstimateCostUSDForEveryModelAndBand(t *testing.T) {
	usage := tokenUsage{
		PromptCacheHitTokens:  1_000_000,
		PromptCacheMissTokens: 1_000_000,
		CompletionTokens:      1_000_000,
	}
	tests := []struct {
		name  string
		model string
		band  pricingBand
		want  float64
	}{
		{name: "flash off-peak", model: flashModel, band: pricingOffPeak, want: 0.887},
		{name: "flash peak", model: flashModel, band: pricingPeak, want: 1.774},
		{name: "pro off-peak", model: proModel, band: pricingOffPeak, want: 2.662},
		{name: "pro peak", model: proModel, band: pricingPeak, want: 5.324},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := estimateCostUSD(tt.model, tt.band, usage); math.Abs(got-tt.want) > 1e-12 {
				t.Errorf("estimateCostUSD(%s, %s) = %f, want %f", tt.model, tt.band, got, tt.want)
			}
		})
	}
}

func TestTemperatureFlagsAndBoundaries(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want float64
	}{
		{name: "short zero", args: []string{"-t", "0"}, want: 0},
		{name: "long decimal", args: []string{"--temperature", "1.3"}, want: 1.3},
		{name: "upper boundary", args: []string{"--temperature", "2"}, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(answerResponse("ok"))
			args := append(tt.args, "-p", "question")
			code, _, stderr := runTest(t, args, "", false, client)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if got := client.requests[0].Temperature; got == nil || *got != tt.want {
				t.Errorf("temperature = %v, want %v", got, tt.want)
			}
			if !strings.Contains(client.bodies[0], `"temperature":`) {
				t.Errorf("request body lacks temperature: %s", client.bodies[0])
			}
		})
	}
}

func TestTemperatureAppliesOnlyToFinalAnswerRequests(t *testing.T) {
	t.Run("clarification", func(t *testing.T) {
		client := newFakeClient(answerResponse("question"), answerResponse("final"))
		code, _, stderr := runTest(t, []string{"-t", "0.4", "-s", "done", "-p", "question"}, "done\n", false, client)
		if code != 0 {
			t.Fatalf("code=%d stderr=%q", code, stderr)
		}
		if client.requests[0].Temperature != nil {
			t.Errorf("clarification request temperature = %v, want nil", client.requests[0].Temperature)
		}
		if got := client.requests[1].Temperature; got == nil || *got != 0.4 {
			t.Errorf("final request temperature = %v, want 0.4", got)
		}
	})

	t.Run("self prompt", func(t *testing.T) {
		client := newFakeClient(answerResponse("generated prompt"), answerResponse("final"))
		code, _, stderr := runTest(t, []string{"--temperature", "1.2", "-a", "self-prompt", "-p", "question"}, "", false, client)
		if code != 0 {
			t.Fatalf("code=%d stderr=%q", code, stderr)
		}
		if client.requests[0].Temperature != nil {
			t.Errorf("prompt-generation request temperature = %v, want nil", client.requests[0].Temperature)
		}
		if got := client.requests[1].Temperature; got == nil || *got != 1.2 {
			t.Errorf("final request temperature = %v, want 1.2", got)
		}
	})
}

func TestPromptInputModes(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		stdin       string
		interactive bool
		wantPrompt  string
		wantStderr  string
	}{
		{name: "short flag", args: []string{"-p", "short"}, stdin: "ignored", wantPrompt: "short"},
		{name: "prompt flag on terminal", args: []string{"-p", "short"}, stdin: "ignored", interactive: true, wantPrompt: "short"},
		{name: "piped multiline", stdin: "line one\nline two\n", wantPrompt: "line one\nline two\n"},
		{
			name: "interactive first line", stdin: "first line\nsecond line\n", interactive: true,
			wantPrompt: "first line", wantStderr: "Prompt: \rWaiting for DeepSeek /\r\x1b[2K",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(answerResponse("ok"))
			code, _, stderr := runTest(t, tt.args, tt.stdin, tt.interactive, client)
			if code != 0 || stderr != tt.wantStderr {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if got := client.requests[0].Messages[0].Content; got != tt.wantPrompt {
				t.Errorf("prompt = %q, want %q", got, tt.wantPrompt)
			}
		})
	}
}

func TestAnswerControlSystemMessages(t *testing.T) {
	formatPath := writeTempFile(t, "  heading\n- item\n")
	wantFormat := "Response requirements:\n" +
		"Follow the answer format below exactly.\n" +
		"Do not add commentary outside the requested format.\n\n" +
		"<answer-format>\n  heading\n- item\n</answer-format>"
	wantLength := "Target final-answer length:\n exactly 3 paragraphs "
	wantStop := "Clarification protocol:\n" +
		"Ask exactly one concise clarifying question per turn.\n" +
		"Do not provide the final answer until the latest user message contains\n" +
		"the following stop sequence, matched case-insensitively: \"DoNe!\"\n" +
		"After it appears, provide the final answer using all information from\n" +
		"the conversation."
	finalTurn := "Clarification status for this turn:\n" +
		"The stop sequence has appeared in the latest user message.\n" +
		"Provide the final answer now using all information from the conversation."

	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "long format", args: []string{"--format", formatPath}, want: wantFormat},
		{name: "short format", args: []string{"-f", formatPath}, want: wantFormat},
		{name: "long length", args: []string{"--length", " exactly 3 paragraphs "}, want: wantLength},
		{name: "short length", args: []string{"-l", " exactly 3 paragraphs "}, want: wantLength},
		{name: "long stop", args: []string{"--stop", "DoNe!"}, want: wantStop + "\n\n" + finalTurn},
		{name: "short stop", args: []string{"-s", "DoNe!"}, want: wantStop + "\n\n" + finalTurn},
		{
			name: "combined deterministic order",
			args: []string{"-s", "DoNe!", "-l", " exactly 3 paragraphs ", "-f", formatPath},
			want: wantFormat + "\n\n" + wantLength + "\n\n" + wantStop + "\n\n" + finalTurn,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append(tt.args, "-p", "already done!")
			client := newFakeClient(answerResponse("ok"))
			code, _, stderr := runTest(t, args, "", false, client)
			if code != 0 {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			messages := client.requests[0].Messages
			if len(messages) != 2 || messages[0] != (chatMessage{Role: "system", Content: tt.want}) || messages[1].Content != "already done!" {
				t.Errorf("messages = %#v", messages)
			}
		})
	}
}

func TestApproachSystemMessages(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "short step by step",
			args: []string{"-a", "step-by-step"},
			want: "Final-answer approach:\n" +
				"Solve the request step by step and present the resulting steps clearly.\n" +
				"Follow all final-answer requirements above.",
		},
		{
			name: "long multi role",
			args: []string{"--approach", "multi-role"},
			want: "Multi-role final-answer approach:\n" +
				"Analyze the request independently from each of these roles:\n" +
				"1. Business analyst\n" +
				"2. Engineer\n" +
				"3. Critic\n" +
				"Use exactly these Markdown section headings, in this order:\n" +
				"## Business analyst\n" +
				"## Engineer\n" +
				"## Critic\n" +
				"Under each heading, write a substantial response using 2-4 short paragraphs.\n" +
				"Separate paragraphs with blank lines; do not compress a role's entire answer into one paragraph.\n" +
				"When useful, cover the role's assessment, concrete recommendations, and risks or trade-offs.\n" +
				"Use bullets, numbered steps, or tables when they make the answer easier to scan.\n" +
				"Write each role's content only below its matching heading.\n" +
				"Do not add an unlabeled introduction, summary, or conclusion.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(answerResponse("ok"))
			args := append(tt.args, "-p", "question")
			code, _, stderr := runTest(t, args, "", false, client)
			if code != 0 {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			assertMessages(t, client.requests[0].Messages, []chatMessage{
				{Role: "system", Content: tt.want},
				{Role: "user", Content: "question"},
			})
		})
	}

	t.Run("explicit none", func(t *testing.T) {
		client := newFakeClient(answerResponse("ok"))
		code, _, stderr := runTest(t, []string{"--approach", "none", "-p", "question"}, "", false, client)
		if code != 0 {
			t.Fatalf("code=%d stderr=%q", code, stderr)
		}
		assertMessages(t, client.requests[0].Messages, []chatMessage{{Role: "user", Content: "question"}})
	})
}

func TestCustomMultiRoleHeadings(t *testing.T) {
	client := newFakeClient(answerResponse("ok"))
	code, _, stderr := runTest(t, []string{
		"--approach", "multi-role",
		"--roles", " Miniature painter, Space Wolves lore expert , Critical reviewer ",
		"--prompt", "question",
	}, "", false, client)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	system := client.requests[0].Messages[0].Content
	for _, want := range []string{
		"1. Miniature painter\n2. Space Wolves lore expert\n3. Critical reviewer",
		"## Miniature painter\n## Space Wolves lore expert\n## Critical reviewer",
		"Under each heading, write a substantial response using 2-4 short paragraphs.",
		"Separate paragraphs with blank lines; do not compress a role's entire answer into one paragraph.",
	} {
		if !strings.Contains(system, want) {
			t.Errorf("system message missing %q: %q", want, system)
		}
	}
}

func TestCustomFormatReplacesMultiRoleMarkdownHeadings(t *testing.T) {
	formatPath := writeTempFile(t, "role={role}; answer={answer}")
	client := newFakeClient(answerResponse("ok"))
	code, _, stderr := runTest(t, []string{
		"--approach", "multi-role",
		"--roles", "Painter, Critic",
		"--format", formatPath,
		"--prompt", "question",
	}, "", false, client)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	system := client.requests[0].Messages[0].Content
	if !strings.Contains(system, "Within the supplied answer format, clearly attribute every answer to its role.") {
		t.Errorf("system message lacks custom-format attribution: %q", system)
	}
	if strings.Contains(system, "Use exactly these Markdown section headings") || strings.Contains(system, "## Painter") {
		t.Errorf("system message contains built-in headings with a custom format: %q", system)
	}
}

func TestSelfPromptMakesTwoRequestsAndPrintsOnlyFinalAnswer(t *testing.T) {
	client := newFakeClient(
		answerResponse("Improved solving prompt\n"),
		answerResponse("Final answer\n\n"),
	)
	code, stdout, stderr := runTest(t, []string{"-a", "self-prompt", "-p", "Original request"}, "", false, client)
	if code != 0 || stdout != "Final answer\n" || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(client.requests) != 2 {
		t.Fatalf("request count = %d", len(client.requests))
	}
	first := client.requests[0].Messages
	if len(first) != 2 || first[0].Role != "system" || !strings.Contains(first[0].Content, "Return only the rewritten prompt") {
		t.Fatalf("first request messages = %#v", first)
	}
	if first[1] != (chatMessage{Role: "user", Content: "Original request"}) {
		t.Errorf("first user message = %#v", first[1])
	}
	assertMessages(t, client.requests[1].Messages, []chatMessage{{Role: "user", Content: "Improved solving prompt\n"}})
}

func TestSelfPromptAppliesAnswerControlsOnlyToFinalRequest(t *testing.T) {
	formatPath := writeTempFile(t, "Title: {title}\n")
	client := newFakeClient(answerResponse("generated prompt"), answerResponse("final"))
	code, _, stderr := runTest(t, []string{
		"--approach", "self-prompt",
		"--format", formatPath,
		"--length", "two paragraphs",
		"--prompt", "question",
	}, "", false, client)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if len(client.requests) != 2 {
		t.Fatalf("request count = %d", len(client.requests))
	}
	generatorSystem := client.requests[0].Messages[0].Content
	for _, unwanted := range []string{"Response requirements:", "Target final-answer length:"} {
		if strings.Contains(generatorSystem, unwanted) {
			t.Errorf("generator system contains %q: %q", unwanted, generatorSystem)
		}
	}
	finalSystem := client.requests[1].Messages[0].Content
	for _, want := range []string{"<answer-format>\nTitle: {title}\n</answer-format>", "Target final-answer length:\ntwo paragraphs"} {
		if !strings.Contains(finalSystem, want) {
			t.Errorf("final system missing %q: %q", want, finalSystem)
		}
	}
	if strings.Contains(finalSystem, "Prompt-generation approach:") {
		t.Errorf("final system still contains prompt-generation instructions: %q", finalSystem)
	}
	if got := client.requests[1].Messages[len(client.requests[1].Messages)-1]; got != (chatMessage{Role: "user", Content: "generated prompt"}) {
		t.Errorf("final user message = %#v", got)
	}
}

func TestSelfPromptAfterClarificationUsesFullConversation(t *testing.T) {
	client := newFakeClient(
		answerResponse("Which audience?"),
		answerResponse("Self-contained generated prompt"),
		answerResponse("Final response"),
	)
	code, stdout, stderr := runTest(t,
		[]string{"-p", "Write a guide", "-s", "READY", "-a", "self-prompt"},
		"New developers; READY\n", false, client)
	if code != 0 || stdout != "Final response\n" || stderr != "Which audience?\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(client.requests) != 3 {
		t.Fatalf("request count = %d", len(client.requests))
	}
	generatorMessages := client.requests[1].Messages
	if len(generatorMessages) != 4 {
		t.Fatalf("generator message count = %d: %#v", len(generatorMessages), generatorMessages)
	}
	if !strings.Contains(generatorMessages[0].Content, "Produce only the self-contained solving prompt now") {
		t.Errorf("generator system lacks final status: %q", generatorMessages[0].Content)
	}
	assertMessages(t, generatorMessages[1:], []chatMessage{
		{Role: "user", Content: "Write a guide"},
		{Role: "assistant", Content: "Which audience?"},
		{Role: "user", Content: "New developers; READY"},
	})
	assertMessages(t, client.requests[2].Messages, []chatMessage{{Role: "user", Content: "Self-contained generated prompt"}})
}

func TestFormatWithoutTrailingNewline(t *testing.T) {
	path := writeTempFile(t, "[title]\n  exact spacing")
	client := newFakeClient(answerResponse("ok"))
	code, _, stderr := runTest(t, []string{"-f", path, "-p", "prompt"}, "", false, client)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	wantSuffix := "<answer-format>\n[title]\n  exact spacing\n</answer-format>"
	if !strings.HasSuffix(client.requests[0].Messages[0].Content, wantSuffix) {
		t.Errorf("system message = %q", client.requests[0].Messages[0].Content)
	}
}

func TestOptionValidation(t *testing.T) {
	directory := t.TempDir()
	empty := writeTempFile(t, "")
	whitespace := writeTempFile(t, " \n\t")
	missing := filepath.Join(t.TempDir(), "missing")
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "empty format path", args: []string{"-f", ""}, want: "format path is empty"},
		{name: "whitespace format path", args: []string{"--format", " \t"}, want: "format path is empty"},
		{name: "empty length", args: []string{"-l", ""}, want: "length is empty"},
		{name: "whitespace length", args: []string{"--length", " \n"}, want: "length is empty"},
		{name: "empty stop", args: []string{"-s", ""}, want: "stop sequence is empty"},
		{name: "whitespace stop", args: []string{"--stop", " \t"}, want: "stop sequence is empty"},
		{name: "empty approach", args: []string{"--approach", ""}, want: "approach is empty"},
		{name: "whitespace approach", args: []string{"-a", " \t"}, want: "approach is empty"},
		{name: "unknown approach", args: []string{"--approach", "fast"}, want: "valid approaches: none, step-by-step, self-prompt, multi-role"},
		{name: "empty temperature", args: []string{"--temperature", ""}, want: "temperature must be a number between 0 and 2"},
		{name: "non-numeric temperature", args: []string{"-t", "warm"}, want: "temperature must be a number between 0 and 2"},
		{name: "temperature below range", args: []string{"--temperature", "-0.1"}, want: "temperature must be a number between 0 and 2"},
		{name: "temperature above range", args: []string{"--temperature", "2.1"}, want: "temperature must be a number between 0 and 2"},
		{name: "temperature NaN", args: []string{"--temperature", "NaN"}, want: "temperature must be a number between 0 and 2"},
		{name: "temperature infinity", args: []string{"--temperature", "+Inf"}, want: "temperature must be a number between 0 and 2"},
		{name: "unknown model", args: []string{"--model", "deepseek-v4-ultra"}, want: "valid models: deepseek-v4-flash, deepseek-v4-pro"},
		{name: "unknown reasoning", args: []string{"--reasoning", "extreme"}, want: "valid values: none, low, high, max"},
		{name: "temperature with reasoning", args: []string{"--temperature", "1", "--reasoning", "low"}, want: "--temperature cannot be used when reasoning is enabled"},
		{name: "roles without multi role", args: []string{"--roles", "Painter,Critic"}, want: "--roles requires --approach multi-role"},
		{name: "empty roles", args: []string{"--approach", "multi-role", "--roles", " \t"}, want: "roles are empty"},
		{name: "one role", args: []string{"--approach", "multi-role", "--roles", "Painter"}, want: "requires at least two roles"},
		{name: "empty role item", args: []string{"--approach", "multi-role", "--roles", "Painter,,Critic"}, want: "role 2 is empty"},
		{name: "role line break", args: []string{"--approach", "multi-role", "--roles", "Painter,Line one\nLine two"}, want: "contains a line break"},
		{name: "missing format file", args: []string{"-f", missing}, want: "read format file"},
		{name: "directory format", args: []string{"-f", directory}, want: "read format file"},
		{name: "zero length format", args: []string{"-f", empty}, want: "format file"},
		{name: "whitespace format", args: []string{"-f", whitespace}, want: "format file"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(answerResponse("unexpected"))
			args := append(tt.args, "-p", "prompt")
			code, stdout, stderr := runTestWithKey(t, args, "", false, client, testAPIKey)
			if code != 2 || stdout != "" || !strings.Contains(stderr, tt.want) || len(client.requests) != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q requests=%d", code, stdout, stderr, len(client.requests))
			}
		})
	}
}

func TestFormatReadFailureAndSingleRead(t *testing.T) {
	t.Run("unreadable", func(t *testing.T) {
		client := newFakeClient(answerResponse("unexpected"))
		var stdout, stderr bytes.Buffer
		code := run([]string{"-f", "private-format.txt", "-p", "prompt"}, strings.NewReader(""), &stdout, &stderr, dependencies{
			client: client, endpoint: testEndpoint,
			getenv:   func(string) string { return testAPIKey },
			readFile: func(string) ([]byte, error) { return nil, errors.New("permission denied") },
		})
		if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "permission denied") || len(client.requests) != 0 {
			t.Fatalf("code=%d stdout=%q stderr=%q requests=%d", code, stdout.String(), stderr.String(), len(client.requests))
		}
	})

	t.Run("read once across conversation", func(t *testing.T) {
		reads := 0
		client := newFakeClient(answerResponse("question"), answerResponse("final"))
		var stdout, stderr bytes.Buffer
		code := run([]string{"-f", "format.txt", "-s", "done", "-p", "prompt"}, strings.NewReader("done\n"), &stdout, &stderr, dependencies{
			client: client, endpoint: testEndpoint,
			getenv: func(string) string { return testAPIKey },
			readFile: func(string) ([]byte, error) {
				reads++
				return []byte("format contents"), nil
			},
		})
		if code != 0 || reads != 1 || len(client.requests) != 2 {
			t.Fatalf("code=%d reads=%d requests=%d stderr=%q", code, reads, len(client.requests), stderr.String())
		}
	})
}

func TestClarificationConversationHistoryAndStreams(t *testing.T) {
	client := newFakeClient(
		answerResponse("Which region?\n\n"),
		answerResponse("What budget?"),
		answerResponse("Final plan\n"),
	)
	code, stdout, stderr := runTest(t,
		[]string{"-p", "Plan a trip", "-s", "READY"},
		"Europe\nMy budget is ready for this\n", false, client)
	if code != 0 || stdout != "Final plan\n" || stderr != "Which region?\nWhat budget?\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if len(client.requests) != 3 {
		t.Fatalf("request count = %d", len(client.requests))
	}
	nonFinalSystem := client.requests[0].Messages[0]
	finalSystem := client.requests[2].Messages[0]
	if !strings.Contains(nonFinalSystem.Content, "Respond with exactly one concise clarifying question and nothing else.") {
		t.Errorf("non-final system message lacks turn reminder: %q", nonFinalSystem.Content)
	}
	if !strings.Contains(finalSystem.Content, "The stop sequence has appeared") {
		t.Errorf("final system message lacks final-turn status: %q", finalSystem.Content)
	}
	assertMessages(t, client.requests[0].Messages, []chatMessage{
		nonFinalSystem,
		{Role: "user", Content: "Plan a trip"},
	})
	assertMessages(t, client.requests[1].Messages, []chatMessage{
		nonFinalSystem,
		{Role: "user", Content: "Plan a trip"},
		{Role: "assistant", Content: "Which region?\n\n"},
		{Role: "user", Content: "Europe"},
	})
	assertMessages(t, client.requests[2].Messages, []chatMessage{
		finalSystem,
		{Role: "user", Content: "Plan a trip"},
		{Role: "assistant", Content: "Which region?\n\n"},
		{Role: "user", Content: "Europe"},
		{Role: "assistant", Content: "What budget?"},
		{Role: "user", Content: "My budget is ready for this"},
	})
}

func TestInteractiveClarificationPrompts(t *testing.T) {
	client := newFakeClient(answerResponse("Which version?"), answerResponse("final"))
	code, stdout, stderr := runTest(t, []string{"-s", "done"}, "Java\ndone\n", true, client)
	if code != 0 || stdout != "final\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, want := range []string{"Prompt: ", "Which version?\nYour answer: "} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr missing %q: %q", want, stderr)
		}
	}
	if got := strings.Count(stderr, "Waiting for DeepSeek /"); got != 2 {
		t.Errorf("spinner count = %d, want 2: %q", got, stderr)
	}
}

func TestPromptFlagInInteractiveClarificationStillSpins(t *testing.T) {
	client := newFakeClient(answerResponse("Which version?"), answerResponse("final"))
	code, stdout, stderr := runTest(t, []string{"-p", "Java", "-s", "done"}, "done\n", true, client)
	if code != 0 || stdout != "final\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.Contains(stderr, "Prompt: ") || !strings.Contains(stderr, "Your answer: ") {
		t.Errorf("unexpected interactive prompts: %q", stderr)
	}
	if got := strings.Count(stderr, "Waiting for DeepSeek /"); got != 2 {
		t.Errorf("spinner count = %d, want 2: %q", got, stderr)
	}
}

func TestSpinnerCyclesAndClears(t *testing.T) {
	var output bytes.Buffer
	stop := startSpinner(&output)
	time.Sleep(350 * time.Millisecond)
	stop()

	got := output.String()
	for _, frame := range []string{
		"Waiting for DeepSeek /",
		"Waiting for DeepSeek -",
		"Waiting for DeepSeek \\",
		"Waiting for DeepSeek |",
		"\r\x1b[2K",
	} {
		if !strings.Contains(got, frame) {
			t.Errorf("spinner output missing %q: %q", frame, got)
		}
	}
}

func TestStopMatching(t *testing.T) {
	tests := []struct {
		name      string
		prompt    string
		stdin     string
		responses []fakeResponse
		wantCalls int
	}{
		{name: "initial case insensitive substring", prompt: "We are ReAdY now", responses: []fakeResponse{answerResponse("final")}, wantCalls: 1},
		{name: "partial does not match", prompt: "not read yet", stdin: "now READY!\n", responses: []fakeResponse{answerResponse("question"), answerResponse("final")}, wantCalls: 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(tt.responses...)
			code, stdout, stderr := runTest(t, []string{"-p", tt.prompt, "-s", "ready"}, tt.stdin, false, client)
			if code != 0 || !strings.Contains(stdout, "final") || len(client.requests) != tt.wantCalls {
				t.Fatalf("code=%d stdout=%q stderr=%q calls=%d", code, stdout, stderr, len(client.requests))
			}
			last := client.requests[len(client.requests)-1].Messages
			if !containsFold(last[len(last)-1].Content, "ready") {
				t.Errorf("sentinel missing from final user message: %#v", last)
			}
		})
	}
}

func TestStopModeUsesSharedLineReader(t *testing.T) {
	client := newFakeClient(answerResponse("question"), answerResponse("final"))
	code, stdout, stderr := runTest(t, []string{"-s", "done"}, "initial prompt\nall done\nextra buffered line\n", false, client)
	if code != 0 || stdout != "final\n" || stderr != "question\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := client.requests[1].Messages[3].Content; got != "all done" {
		t.Errorf("second user message = %q", got)
	}
}

func TestPromptPrecedenceInStopMode(t *testing.T) {
	client := newFakeClient(answerResponse("question"), answerResponse("final"))
	code, _, stderr := runTest(t, []string{"-p", "flag prompt", "-s", "done"}, "stdin done\n", false, client)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if got := client.requests[0].Messages[1].Content; got != "flag prompt" {
		t.Errorf("initial prompt = %q", got)
	}
	if got := client.requests[1].Messages[3].Content; got != "stdin done" {
		t.Errorf("follow-up = %q", got)
	}
}

func TestEOFBeforeStopIsInputError(t *testing.T) {
	client := newFakeClient(answerResponse("question"))
	code, stdout, stderr := runTest(t, []string{"-p", "prompt", "-s", "done"}, "", false, client)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "question\nerror: read clarification from stdin: EOF") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAPIFailureDuringClarification(t *testing.T) {
	client := newFakeClient(answerResponse("question"), fakeResponse{status: 503, body: `{"error":{"message":"unavailable"}}`})
	code, stdout, stderr := runTest(t, []string{"-p", "prompt", "-s", "done"}, "still discussing\n", false, client)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "question\n") || !strings.Contains(stderr, "503 Service Unavailable: unavailable") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAPIFailureDuringSelfPromptFinalRequest(t *testing.T) {
	client := newFakeClient(
		answerResponse("generated prompt"),
		fakeResponse{status: 503, body: `{"error":{"message":"unavailable"}}`},
	)
	code, stdout, stderr := runTest(t, []string{"-p", "prompt", "-a", "self-prompt"}, "", false, client)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "503 Service Unavailable: unavailable") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if strings.Contains(stdout, "generated prompt") {
		t.Errorf("stdout leaked generated prompt: %q", stdout)
	}
}

func TestSelfPromptDebugLogsBothRequests(t *testing.T) {
	client := newFakeClient(answerResponse("generated prompt"), answerResponse("final"))
	code, stdout, stderr := runTest(t, []string{"-d", "-p", "prompt", "-a", "self-prompt"}, "", false, client)
	if code != 0 || stdout != "final\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := strings.Count(stderr, "> POST "+testEndpoint+" HTTP/1.1"); got != 2 {
		t.Errorf("debug request count = %d, want 2: %q", got, stderr)
	}
	if !strings.Contains(stderr, `"content":"generated prompt"`) {
		t.Errorf("debug output does not contain generated prompt request: %q", stderr)
	}
}

func TestInteractiveSelfPromptSpinsForBothRequests(t *testing.T) {
	client := newFakeClient(answerResponse("generated prompt"), answerResponse("final"))
	code, stdout, stderr := runTest(t, []string{"--approach", "self-prompt"}, "original prompt\n", true, client)
	if code != 0 || stdout != "final\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if !strings.HasPrefix(stderr, "Prompt: ") {
		t.Errorf("interactive prompt missing: %q", stderr)
	}
	if got := strings.Count(stderr, "Waiting for DeepSeek /"); got != 2 {
		t.Errorf("spinner count = %d, want 2: %q", got, stderr)
	}
}

func TestDebugOutputIsRedactedAndAnswerStaysClean(t *testing.T) {
	response := answerResponse("answer")
	response.headers = http.Header{"X-Echo": []string{testAPIKey}}
	for _, debugFlag := range []string{"-d", "--debug"} {
		t.Run(debugFlag, func(t *testing.T) {
			client := newFakeClient(response)
			code, stdout, stderr := runTest(t, []string{debugFlag, "-p", "prompt"}, "", false, client)
			if code != 0 || stdout != "answer\n" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			for _, want := range []string{
				"> POST " + testEndpoint + " HTTP/1.1",
				"> Authorization: Bearer [REDACTED]",
				`"model":"deepseek-v4-flash"`,
				"< HTTP/1.1 200 OK",
			} {
				if !strings.Contains(stderr, want) {
					t.Errorf("debug output missing %q:\n%s", want, stderr)
				}
			}
			if strings.Contains(stderr, testAPIKey) {
				t.Errorf("debug output leaked API key: %s", stderr)
			}
		})
	}
}

func TestHelpDoesNotRequireAPIKey(t *testing.T) {
	for _, arg := range []string{"-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			code, stdout, stderr := runTestWithKey(t, []string{arg}, "", false, newFakeClient(), "")
			if code != 0 || !strings.Contains(stdout, "-f, --format FILE") || !strings.Contains(stdout, "-s, --stop SEQUENCE") || !strings.Contains(stdout, "-a, --approach MODE") || !strings.Contains(stdout, "--roles LIST") || !strings.Contains(stdout, "-m, --model MODEL") || !strings.Contains(stdout, "-r, --reasoning N") || !strings.Contains(stdout, "-t, --temperature N") || !strings.Contains(stdout, "--stats") || stderr != "" {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
		})
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		stdin     io.Reader
		key       string
		wantError string
	}{
		{name: "missing key", args: []string{"-p", "hello"}, stdin: strings.NewReader(""), wantError: "DEEPSEEK_API_KEY is not set"},
		{name: "empty prompt flag", args: []string{"-p", ""}, stdin: strings.NewReader("valid stdin"), key: testAPIKey, wantError: "prompt is empty"},
		{name: "whitespace stdin", stdin: strings.NewReader(" \n\t"), key: testAPIKey, wantError: "prompt is empty"},
		{name: "positional argument", args: []string{"hello"}, stdin: strings.NewReader(""), key: testAPIKey, wantError: "unexpected positional arguments"},
		{name: "stdin read failure", stdin: errorReader{}, key: testAPIKey, wantError: "read prompt from stdin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, tt.stdin, &stdout, &stderr, dependencies{
				client: newFakeClient(), endpoint: testEndpoint,
				getenv: func(string) string { return tt.key },
			})
			if code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), tt.wantError) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestResponseFailures(t *testing.T) {
	tests := []struct {
		name      string
		response  fakeResponse
		wantError string
	}{
		{name: "API JSON error", response: fakeResponse{status: 401, body: `{"error":{"message":"bad key ` + testAPIKey + `","type":"authentication_error"}}`}, wantError: "bad key [REDACTED] (authentication_error)"},
		{name: "API plain error", response: fakeResponse{status: 502, body: "upstream failed"}, wantError: "502 Bad Gateway"},
		{name: "malformed JSON", response: fakeResponse{status: 200, body: "not json"}, wantError: "decode DeepSeek response"},
		{name: "no choices", response: fakeResponse{status: 200, body: `{"choices":[]}`}, wantError: "does not contain an answer"},
		{name: "null answer", response: fakeResponse{status: 200, body: `{"choices":[{"message":{"content":null}}]}`}, wantError: "does not contain an answer"},
		{name: "empty answer", response: fakeResponse{status: 200, body: `{"choices":[{"message":{"content":""}}]}`}, wantError: "returned an empty answer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := newFakeClient(tt.response)
			code, stdout, stderr := runTest(t, []string{"-p", "prompt"}, "", false, client)
			if code != 1 || stdout != "" || !strings.Contains(stderr, tt.wantError) {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			if strings.Contains(stderr, testAPIKey) {
				t.Errorf("stderr leaked API key: %s", stderr)
			}
		})
	}
}

func TestNetworkFailure(t *testing.T) {
	client := newFakeClient(fakeResponse{err: errors.New("connection failed")})
	code, stdout, stderr := runTest(t, []string{"-p", "prompt"}, "", false, client)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "connection failed") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAnswerHasExactlyOneTrailingNewline(t *testing.T) {
	client := newFakeClient(answerResponse("line one\nline two\n\n"))
	code, stdout, stderr := runTest(t, []string{"-p", "prompt"}, "", false, client)
	if code != 0 || stdout != "line one\nline two\n" || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func runTest(t *testing.T, args []string, stdin string, interactive bool, client httpDoer) (int, string, string) {
	t.Helper()
	return runTestWithKey(t, args, stdin, interactive, client, testAPIKey)
}

func runTestWithKey(t *testing.T, args []string, stdin string, interactive bool, client httpDoer, key string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(stdin), &stdout, &stderr, dependencies{
		client: client, endpoint: testEndpoint,
		getenv: func(string) string { return key }, interactiveStdin: interactive,
	})
	return code, stdout.String(), stderr.String()
}

func assertMessages(t *testing.T, got, want []chatMessage) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("message count = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("message %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "format.txt")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type fakeResponse struct {
	status  int
	body    string
	headers http.Header
	err     error
}

func answerResponse(answer string) fakeResponse {
	return answerResponseWithUsage(answer, tokenUsage{})
}

func answerResponseWithUsage(answer string, usage tokenUsage) fakeResponse {
	body, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": answer}}},
		"usage":   usage,
	})
	if err != nil {
		panic(err)
	}
	return fakeResponse{status: http.StatusOK, body: string(body)}
}

func sequenceClock(times ...time.Time) func() time.Time {
	index := 0
	return func() time.Time {
		if index >= len(times) {
			panic("sequence clock exhausted")
		}
		result := times[index]
		index++
		return result
	}
}

type fakeClient struct {
	responses []fakeResponse
	requests  []chatRequest
	headers   []http.Header
	bodies    []string
}

func newFakeClient(responses ...fakeResponse) *fakeClient {
	return &fakeClient{responses: responses}
}

func (f *fakeClient) Do(req *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	var request chatRequest
	if err := json.Unmarshal(body, &request); err != nil {
		return nil, err
	}
	f.requests = append(f.requests, request)
	f.headers = append(f.headers, req.Header.Clone())
	f.bodies = append(f.bodies, string(body))
	index := len(f.requests) - 1
	if index >= len(f.responses) {
		return nil, errors.New("unexpected request")
	}
	spec := f.responses[index]
	if spec.err != nil {
		return nil, spec.err
	}
	status := spec.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		Proto:      "HTTP/1.1",
		Header:     spec.headers.Clone(),
		Body:       io.NopCloser(strings.NewReader(spec.body)),
	}, nil
}
