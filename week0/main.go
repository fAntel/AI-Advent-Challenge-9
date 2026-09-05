package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	defaultEndpoint   = "https://api.deepseek.com/chat/completions"
	defaultModel      = "deepseek-v4-flash"
	connectionTimeout = 10 * time.Second
	requestTimeout    = 5 * time.Minute
)

type dependencies struct {
	client           httpDoer
	endpoint         string
	getenv           func(string) string
	interactiveStdin bool
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type promptValue struct {
	value string
	set   bool
}

func (p *promptValue) String() string { return p.value }

func (p *promptValue) Set(value string) error {
	p.value = value
	p.set = true
	return nil
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Thinking thinkingMode  `json:"thinking"`
	Stream   bool          `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type thinkingMode struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content *string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type apiErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

func main() {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{
		Timeout:   connectionTimeout,
		KeepAlive: 30 * time.Second,
	}).DialContext
	client := &http.Client{
		Transport: transport,
		Timeout:   requestTimeout,
	}
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, dependencies{
		client:           client,
		endpoint:         defaultEndpoint,
		getenv:           os.Getenv,
		interactiveStdin: isTerminal(os.Stdin),
	}))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, deps dependencies) int {
	fs := flag.NewFlagSet("deepseek-asker", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(stderr) }

	var prompt promptValue
	var debug bool
	var help bool
	fs.Var(&prompt, "p", "prompt to send (takes precedence over stdin)")
	fs.Var(&prompt, "prompt", "prompt to send (takes precedence over stdin)")
	fs.BoolVar(&debug, "d", false, "print masked HTTP diagnostics to stderr")
	fs.BoolVar(&debug, "debug", false, "print masked HTTP diagnostics to stderr")
	fs.BoolVar(&help, "h", false, "show this help")
	fs.BoolVar(&help, "help", false, "show this help")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if help {
		printUsage(stdout)
		return 0
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(stderr, "error: unexpected positional arguments")
		return 2
	}

	apiKey := deps.getenv("DEEPSEEK_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		fmt.Fprintln(stderr, "error: DEEPSEEK_API_KEY is not set")
		return 2
	}

	promptText := prompt.value
	if !prompt.set {
		var err error
		promptText, err = readPrompt(stdin, stderr, deps.interactiveStdin)
		if err != nil {
			fmt.Fprintf(stderr, "error: read prompt from stdin: %v\n", err)
			return 2
		}
	}
	if strings.TrimSpace(promptText) == "" {
		fmt.Fprintln(stderr, "error: prompt is empty")
		return 2
	}

	body, err := json.Marshal(chatRequest{
		Model: defaultModel,
		Messages: []chatMessage{{
			Role:    "user",
			Content: promptText,
		}},
		Thinking: thinkingMode{Type: "disabled"},
		Stream:   false,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: encode request: %v\n", err)
		return 1
	}

	req, err := http.NewRequest(http.MethodPost, deps.endpoint, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(stderr, "error: create request: %v\n", redact(err.Error(), apiKey))
		return 1
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if debug {
		writeDebugRequest(stderr, req, body, apiKey)
	}

	resp, err := deps.client.Do(req)
	if err != nil {
		fmt.Fprintf(stderr, "error: send request: %s\n", redact(err.Error(), apiKey))
		return 1
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(stderr, "error: read response: %s\n", redact(err.Error(), apiKey))
		return 1
	}
	if debug {
		writeDebugResponse(stderr, resp, responseBody, apiKey)
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		fmt.Fprintf(stderr, "error: DeepSeek API returned %s", resp.Status)
		if message := apiErrorMessage(responseBody); message != "" {
			fmt.Fprintf(stderr, ": %s", redact(message, apiKey))
		}
		fmt.Fprintln(stderr)
		return 1
	}

	var result chatResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		fmt.Fprintf(stderr, "error: decode DeepSeek response: %s\n", redact(err.Error(), apiKey))
		return 1
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == nil {
		fmt.Fprintln(stderr, "error: DeepSeek response does not contain an answer")
		return 1
	}
	answer := *result.Choices[0].Message.Content
	if answer == "" {
		fmt.Fprintln(stderr, "error: DeepSeek returned an empty answer")
		return 1
	}

	answer = strings.TrimRight(answer, "\r\n")
	if _, err := fmt.Fprintln(stdout, answer); err != nil {
		fmt.Fprintf(stderr, "error: write answer: %v\n", err)
		return 1
	}
	return 0
}

func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func readPrompt(stdin io.Reader, stderr io.Writer, interactive bool) (string, error) {
	if !interactive {
		data, err := io.ReadAll(stdin)
		return string(data), err
	}

	if _, err := fmt.Fprint(stderr, "Prompt: "); err != nil {
		return "", err
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: deepseek-asker [options]

Send a prompt to DeepSeek and print only its answer to stdout.

Options:
  -p, --prompt TEXT  Prompt to send. If omitted, read stdin to EOF when piped,
                     or one line when running interactively.
  -d, --debug        Print masked HTTP request/response details to stderr.
  -h, --help         Show this help.

Environment:
  DEEPSEEK_API_KEY   DeepSeek API key (required).

Examples:
  deepseek-asker --prompt "Explain goroutines briefly"
  printf 'Explain goroutines briefly' | deepseek-asker
  deepseek-asker --debug -p "Hello" >answer.txt`)
}

func writeDebugRequest(w io.Writer, req *http.Request, body []byte, apiKey string) {
	fmt.Fprintf(w, "> %s %s %s\n", req.Method, req.URL.String(), req.Proto)
	writeDebugHeaders(w, ">", req.Header, apiKey)
	fmt.Fprintln(w, ">")
	fmt.Fprintf(w, "%s\n", redact(string(body), apiKey))
}

func writeDebugResponse(w io.Writer, resp *http.Response, body []byte, apiKey string) {
	fmt.Fprintf(w, "< %s %s\n", resp.Proto, resp.Status)
	writeDebugHeaders(w, "<", resp.Header, apiKey)
	fmt.Fprintln(w, "<")
	fmt.Fprintf(w, "%s\n", redact(string(body), apiKey))
}

func writeDebugHeaders(w io.Writer, prefix string, headers http.Header, apiKey string) {
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		for _, value := range headers.Values(key) {
			if strings.EqualFold(key, "Authorization") {
				value = "Bearer [REDACTED]"
			}
			fmt.Fprintf(w, "%s %s: %s\n", prefix, key, redact(value, apiKey))
		}
	}
}

func redact(value, apiKey string) string {
	if apiKey == "" {
		return value
	}
	return strings.ReplaceAll(value, apiKey, "[REDACTED]")
}

func apiErrorMessage(body []byte) string {
	var response apiErrorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ""
	}
	if response.Error.Message == "" {
		return ""
	}
	if response.Error.Type == "" {
		return response.Error.Message
	}
	return fmt.Sprintf("%s (%s)", response.Error.Message, response.Error.Type)
}
