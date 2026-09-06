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
	spinnerInterval   = 100 * time.Millisecond
)

type dependencies struct {
	client           httpDoer
	endpoint         string
	getenv           func(string) string
	readFile         func(string) ([]byte, error)
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
		readFile:         os.ReadFile,
		interactiveStdin: isTerminal(os.Stdin),
	}))
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer, deps dependencies) int {
	fs := flag.NewFlagSet("deepseek-asker", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { printUsage(stderr) }

	var prompt promptValue
	var formatPath promptValue
	var length promptValue
	var stop promptValue
	var debug bool
	var help bool
	fs.Var(&prompt, "p", "prompt to send (takes precedence over stdin)")
	fs.Var(&prompt, "prompt", "prompt to send (takes precedence over stdin)")
	fs.Var(&formatPath, "f", "file containing the required final-answer format")
	fs.Var(&formatPath, "format", "file containing the required final-answer format")
	fs.Var(&length, "l", "desired final-answer length")
	fs.Var(&length, "length", "desired final-answer length")
	fs.Var(&stop, "s", "sequence that ends clarification mode")
	fs.Var(&stop, "stop", "sequence that ends clarification mode")
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
	if formatPath.set && strings.TrimSpace(formatPath.value) == "" {
		fmt.Fprintln(stderr, "error: format path is empty")
		return 2
	}
	if length.set && strings.TrimSpace(length.value) == "" {
		fmt.Fprintln(stderr, "error: length is empty")
		return 2
	}
	if stop.set && strings.TrimSpace(stop.value) == "" {
		fmt.Fprintln(stderr, "error: stop sequence is empty")
		return 2
	}

	var format string
	if formatPath.set {
		readFile := deps.readFile
		if readFile == nil {
			readFile = os.ReadFile
		}
		contents, err := readFile(formatPath.value)
		if err != nil {
			fmt.Fprintf(stderr, "error: read format file %q: %v\n", formatPath.value, err)
			return 2
		}
		format = string(contents)
		if strings.TrimSpace(format) == "" {
			fmt.Fprintf(stderr, "error: format file %q is empty\n", formatPath.value)
			return 2
		}
	}

	apiKey := deps.getenv("DEEPSEEK_API_KEY")
	if strings.TrimSpace(apiKey) == "" {
		fmt.Fprintln(stderr, "error: DEEPSEEK_API_KEY is not set")
		return 2
	}

	var lineReader *bufio.Reader
	if stop.set {
		lineReader = bufio.NewReader(stdin)
	}

	promptText := prompt.value
	if !prompt.set {
		var err error
		if stop.set {
			promptText, err = readInputLine(lineReader, stderr, deps.interactiveStdin, "Prompt: ")
		} else {
			promptText, err = readPrompt(stdin, stderr, deps.interactiveStdin)
		}
		if err != nil {
			fmt.Fprintf(stderr, "error: read prompt from stdin: %v\n", err)
			return 2
		}
	}
	if strings.TrimSpace(promptText) == "" {
		fmt.Fprintln(stderr, "error: prompt is empty")
		return 2
	}

	messages := make([]chatMessage, 0, 2)
	if systemMessage := buildSystemMessage(formatPath.set, format, length, stop); systemMessage != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemMessage})
	}
	messages = append(messages, chatMessage{Role: "user", Content: promptText})
	showSpinner := deps.interactiveStdin && (stop.set || !prompt.set)

	for {
		final := !stop.set || containsFold(messages[len(messages)-1].Content, stop.value)
		requestMessages := messages
		if stop.set {
			requestMessages = withClarificationTurnStatus(messages, final)
		}
		answer, ok := requestAnswer(requestMessages, debug, showSpinner, stderr, deps, apiKey)
		if !ok {
			return 1
		}

		if final {
			if err := writeAnswer(stdout, answer); err != nil {
				fmt.Fprintf(stderr, "error: write answer: %v\n", err)
				return 1
			}
			return 0
		}

		if err := writeAnswer(stderr, answer); err != nil {
			fmt.Fprintf(stderr, "error: write clarification: %v\n", err)
			return 1
		}
		messages = append(messages, chatMessage{Role: "assistant", Content: answer})

		next, err := readInputLine(lineReader, stderr, deps.interactiveStdin, "Your answer: ")
		if err != nil {
			fmt.Fprintf(stderr, "error: read clarification from stdin: %v\n", err)
			return 2
		}
		messages = append(messages, chatMessage{Role: "user", Content: next})
	}
}

func buildSystemMessage(hasFormat bool, format string, length, stop promptValue) string {
	sections := make([]string, 0, 3)
	if hasFormat {
		section := "Response requirements:\n" +
			"Follow the answer format below exactly.\n" +
			"Do not add commentary outside the requested format.\n\n" +
			"<answer-format>\n" + format
		if !strings.HasSuffix(format, "\n") {
			section += "\n"
		}
		section += "</answer-format>"
		sections = append(sections, section)
	}
	if length.set {
		sections = append(sections, "Target final-answer length:\n"+length.value)
	}
	if stop.set {
		sections = append(sections, "Clarification protocol:\n"+
			"Ask exactly one concise clarifying question per turn.\n"+
			"Do not provide the final answer until the latest user message contains\n"+
			"the following stop sequence, matched case-insensitively: \""+stop.value+"\"\n"+
			"After it appears, provide the final answer using all information from\n"+
			"the conversation.")
	}
	return strings.Join(sections, "\n\n")
}

func containsFold(value, substring string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(substring))
}

func withClarificationTurnStatus(messages []chatMessage, final bool) []chatMessage {
	result := append([]chatMessage(nil), messages...)
	if len(result) == 0 || result[0].Role != "system" {
		return result
	}

	status := "Clarification status for this turn:\n" +
		"The stop sequence has not appeared in the latest user message.\n" +
		"Respond with exactly one concise clarifying question and nothing else.\n" +
		"Do not answer, explain, suggest, or summarize."
	if final {
		status = "Clarification status for this turn:\n" +
			"The stop sequence has appeared in the latest user message.\n" +
			"Provide the final answer now using all information from the conversation."
	}
	result[0].Content += "\n\n" + status
	return result
}

func requestAnswer(messages []chatMessage, debug, showSpinner bool, stderr io.Writer, deps dependencies, apiKey string) (string, bool) {
	body, err := json.Marshal(chatRequest{
		Model:    defaultModel,
		Messages: messages,
		Thinking: thinkingMode{Type: "disabled"},
		Stream:   false,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: encode request: %v\n", err)
		return "", false
	}

	req, err := http.NewRequest(http.MethodPost, deps.endpoint, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(stderr, "error: create request: %v\n", redact(err.Error(), apiKey))
		return "", false
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	if debug {
		writeDebugRequest(stderr, req, body, apiKey)
	}

	stopSpinner := func() {}
	if showSpinner {
		stopSpinner = startSpinner(stderr)
	}
	resp, err := deps.client.Do(req)
	if err != nil {
		stopSpinner()
		fmt.Fprintf(stderr, "error: send request: %s\n", redact(err.Error(), apiKey))
		return "", false
	}
	responseBody, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	stopSpinner()
	if readErr != nil {
		fmt.Fprintf(stderr, "error: read response: %s\n", redact(readErr.Error(), apiKey))
		return "", false
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
		return "", false
	}

	var result chatResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		fmt.Fprintf(stderr, "error: decode DeepSeek response: %s\n", redact(err.Error(), apiKey))
		return "", false
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == nil {
		fmt.Fprintln(stderr, "error: DeepSeek response does not contain an answer")
		return "", false
	}
	answer := *result.Choices[0].Message.Content
	if answer == "" {
		fmt.Fprintln(stderr, "error: DeepSeek returned an empty answer")
		return "", false
	}
	return answer, true
}

func startSpinner(w io.Writer) func() {
	frames := [...]byte{'/', '-', '\\', '|'}
	done := make(chan struct{})
	finished := make(chan struct{})

	fmt.Fprintf(w, "\rWaiting for DeepSeek %c", frames[0])
	go func() {
		defer close(finished)
		ticker := time.NewTicker(spinnerInterval)
		defer ticker.Stop()
		frame := 1
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Fprintf(w, "\rWaiting for DeepSeek %c", frames[frame])
				frame = (frame + 1) % len(frames)
			}
		}
	}()

	return func() {
		close(done)
		<-finished
		fmt.Fprint(w, "\r\x1b[2K")
	}
}

func writeAnswer(w io.Writer, answer string) error {
	_, err := fmt.Fprintln(w, strings.TrimRight(answer, "\r\n"))
	return err
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

func readInputLine(reader *bufio.Reader, stderr io.Writer, interactive bool, prompt string) (string, error) {
	if interactive && prompt != "" {
		if _, err := fmt.Fprint(stderr, prompt); err != nil {
			return "", err
		}
	}
	line, err := reader.ReadString('\n')
	if err != nil && !(errors.Is(err, io.EOF) && len(line) > 0) {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line, nil
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, `Usage: deepseek-asker [options]

Send a prompt to DeepSeek and print only its answer to stdout.
Interactive prompt and clarification waits show a spinner on stderr.

Options:
  -p, --prompt TEXT   Prompt to send. If omitted, read stdin to EOF when piped,
                      or one line when running interactively. In stop mode,
                      input is always read one line at a time.
  -f, --format FILE   File containing the required final-answer format.
  -l, --length TEXT   Desired final-answer length (for example, "3 paragraphs").
  -s, --stop SEQUENCE Ask one clarifying question per turn until a user line
                      contains SEQUENCE (case-insensitive), prompting for each
                      response in interactive terminals.
  -d, --debug         Print masked HTTP request/response details to stderr.
  -h, --help          Show this help.

Environment:
  DEEPSEEK_API_KEY   DeepSeek API key (required).

Examples:
  deepseek-asker --prompt "Explain goroutines briefly"
  printf 'Explain goroutines briefly' | deepseek-asker
  deepseek-asker -p "Compare Go and Rust" -l "3 paragraphs"
  deepseek-asker -p "Plan a trip" -f itinerary.txt -s READY
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
