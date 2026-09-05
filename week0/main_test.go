package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const testAPIKey = "super-secret-test-key"

func TestPromptFlagAndRequest(t *testing.T) {
	var received chatRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testAPIKey {
			t.Errorf("Authorization = %q", got)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("Content-Type = %q", got)
		}
		if err := decodeJSON(r.Body, &received); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"The answer"}}]}`)
	}))
	defer server.Close()

	code, stdout, stderr := runTest(t, []string{"--prompt", "from flag"}, "ignored stdin", server)
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr)
	}
	if stdout != "The answer\n" {
		t.Errorf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q", stderr)
	}
	if received.Model != defaultModel || received.Thinking.Type != "disabled" || received.Stream {
		t.Errorf("unexpected request settings: %+v", received)
	}
	if len(received.Messages) != 1 || received.Messages[0] != (chatMessage{Role: "user", Content: "from flag"}) {
		t.Errorf("messages = %+v", received.Messages)
	}
}

func TestShortPromptFlagAndMultilineStdin(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{name: "short flag", args: []string{"-p", "short"}, stdin: "ignored", want: "short"},
		{name: "stdin", stdin: "line one\nline two\n", want: "line one\nline two\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request chatRequest
				if err := decodeJSON(r.Body, &request); err != nil {
					t.Fatal(err)
				}
				got = request.Messages[0].Content
				io.WriteString(w, `{"choices":[{"message":{"content":"ok"}}]}`)
			}))
			defer server.Close()

			code, _, stderr := runTest(t, tt.args, tt.stdin, server)
			if code != 0 {
				t.Fatalf("exit code = %d, stderr = %q", code, stderr)
			}
			if got != tt.want {
				t.Errorf("prompt = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestInteractiveStdinReadsOnlyFirstLine(t *testing.T) {
	var received string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request chatRequest
		if err := decodeJSON(r.Body, &request); err != nil {
			t.Fatal(err)
		}
		received = request.Messages[0].Content
		io.WriteString(w, `{"choices":[{"message":{"content":"interactive answer"}}]}`)
	}))
	defer server.Close()

	var stdout, stderr bytes.Buffer
	code := run(nil, strings.NewReader("first line\nsecond line\n"), &stdout, &stderr, dependencies{
		client:           server.Client(),
		endpoint:         server.URL,
		getenv:           func(string) string { return testAPIKey },
		interactiveStdin: true,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr = %q", code, stderr.String())
	}
	if received != "first line" {
		t.Errorf("prompt = %q, want first line", received)
	}
	if stdout.String() != "interactive answer\n" {
		t.Errorf("stdout = %q", stdout.String())
	}
	if stderr.String() != "Prompt: " {
		t.Errorf("stderr = %q", stderr.String())
	}
}

func TestDebugOutputIsRedactedAndAnswerStaysClean(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Echo", testAPIKey)
		io.WriteString(w, `{"id":"response-id","choices":[{"message":{"content":"answer"}}]}`)
	}))
	defer server.Close()

	for _, debugFlag := range []string{"-d", "--debug"} {
		t.Run(debugFlag, func(t *testing.T) {
			code, stdout, stderr := runTest(t, []string{debugFlag, "-p", "prompt"}, "", server)
			if code != 0 {
				t.Fatalf("exit code = %d, stderr = %q", code, stderr)
			}
			if stdout != "answer\n" {
				t.Errorf("stdout = %q", stdout)
			}
			for _, want := range []string{
				"> POST " + server.URL + " HTTP/1.1",
				"> Authorization: Bearer [REDACTED]",
				`"model":"deepseek-v4-flash"`,
				"< HTTP/1.1 200 OK",
				`"id":"response-id"`,
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
			var stdout, stderr bytes.Buffer
			code := run([]string{arg}, strings.NewReader(""), &stdout, &stderr, dependencies{
				client: http.DefaultClient,
				getenv: func(string) string { return "" },
			})
			if code != 0 || !strings.Contains(stdout.String(), "Usage: deepseek-asker") || stderr.Len() != 0 {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
			}
		})
	}
}

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		key        string
		wantError  string
		readerFail bool
	}{
		{name: "missing key", args: []string{"-p", "hello"}, wantError: "DEEPSEEK_API_KEY is not set"},
		{name: "empty flag takes precedence", args: []string{"-p", ""}, stdin: "valid stdin", key: testAPIKey, wantError: "prompt is empty"},
		{name: "whitespace stdin", stdin: " \n\t", key: testAPIKey, wantError: "prompt is empty"},
		{name: "positional argument", args: []string{"hello"}, key: testAPIKey, wantError: "unexpected positional arguments"},
		{name: "stdin read failure", key: testAPIKey, wantError: "read prompt from stdin", readerFail: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdin io.Reader = strings.NewReader(tt.stdin)
			if tt.readerFail {
				stdin = errorReader{}
			}
			var stdout, stderr bytes.Buffer
			code := run(tt.args, stdin, &stdout, &stderr, dependencies{
				client: http.DefaultClient,
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
		status    int
		body      string
		wantError string
	}{
		{name: "API JSON error", status: 401, body: `{"error":{"message":"bad key ` + testAPIKey + `","type":"authentication_error"}}`, wantError: "bad key [REDACTED] (authentication_error)"},
		{name: "API plain error", status: 502, body: "upstream failed", wantError: "502 Bad Gateway"},
		{name: "malformed JSON", status: 200, body: "not json", wantError: "decode DeepSeek response"},
		{name: "no choices", status: 200, body: `{"choices":[]}`, wantError: "does not contain an answer"},
		{name: "null answer", status: 200, body: `{"choices":[{"message":{"content":null}}]}`, wantError: "does not contain an answer"},
		{name: "empty answer", status: 200, body: `{"choices":[{"message":{"content":""}}]}`, wantError: "returned an empty answer"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				io.WriteString(w, tt.body)
			}))
			defer server.Close()
			code, stdout, stderr := runTest(t, []string{"-p", "prompt"}, "", server)
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
	var stdout, stderr bytes.Buffer
	code := run([]string{"-p", "prompt"}, strings.NewReader(""), &stdout, &stderr, dependencies{
		client: roundTripClient(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("connection failed")
		}),
		endpoint: "https://example.invalid/chat/completions",
		getenv:   func(string) string { return testAPIKey },
	})
	if code != 1 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "connection failed") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestAnswerHasExactlyOneTrailingNewline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		io.WriteString(w, `{"choices":[{"message":{"content":"line one\nline two\n\n"}}]}`)
	}))
	defer server.Close()
	code, stdout, stderr := runTest(t, []string{"-p", "prompt"}, "", server)
	if code != 0 || stdout != "line one\nline two\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func runTest(t *testing.T, args []string, stdin string, server *httptest.Server) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, strings.NewReader(stdin), &stdout, &stderr, dependencies{
		client:   server.Client(),
		endpoint: server.URL,
		getenv:   func(string) string { return testAPIKey },
	})
	return code, stdout.String(), stderr.String()
}

func decodeJSON(r io.Reader, value any) error {
	return json.NewDecoder(r).Decode(value)
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

type roundTripClient func(*http.Request) (*http.Response, error)

func (f roundTripClient) Do(req *http.Request) (*http.Response, error) { return f(req) }
