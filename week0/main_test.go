package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	if request.Model != defaultModel || request.Thinking.Type != "disabled" || request.Stream {
		t.Errorf("unexpected request settings: %+v", request)
	}
	assertMessages(t, request.Messages, []chatMessage{{Role: "user", Content: "from flag"}})
	if got := client.headers[0].Get("Authorization"); got != "Bearer "+testAPIKey {
		t.Errorf("Authorization = %q", got)
	}
	if got := client.headers[0].Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
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
			if code != 0 || !strings.Contains(stdout, "-f, --format FILE") || !strings.Contains(stdout, "-s, --stop SEQUENCE") || stderr != "" {
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
	body, err := json.Marshal(map[string]any{
		"choices": []any{map[string]any{"message": map[string]any{"content": answer}}},
	})
	if err != nil {
		panic(err)
	}
	return fakeResponse{status: http.StatusOK, body: string(body)}
}

type fakeClient struct {
	responses []fakeResponse
	requests  []chatRequest
	headers   []http.Header
}

func newFakeClient(responses ...fakeResponse) *fakeClient {
	return &fakeClient{responses: responses}
}

func (f *fakeClient) Do(req *http.Request) (*http.Response, error) {
	var request chatRequest
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		return nil, err
	}
	f.requests = append(f.requests, request)
	f.headers = append(f.headers, req.Header.Clone())
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
