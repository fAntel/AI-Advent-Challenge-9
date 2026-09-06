package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"net/http"
	"os"
	"sort"
	"strconv"
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

const (
	flashModel = "deepseek-v4-flash"
	proModel   = "deepseek-v4-pro"
)

// DeepSeek prices in USD per 1M tokens, verified on 2026-09-06 at
// https://api-docs.deepseek.com/quick_start/pricing/.
var modelPricing = map[string]map[pricingBand]tokenRates{
	flashModel: {
		pricingOffPeak: {cacheHit: 0.007, cacheMiss: 0.22, output: 0.66},
		pricingPeak:    {cacheHit: 0.014, cacheMiss: 0.44, output: 1.32},
	},
	proModel: {
		pricingOffPeak: {cacheHit: 0.022, cacheMiss: 0.66, output: 1.98},
		pricingPeak:    {cacheHit: 0.044, cacheMiss: 1.32, output: 3.96},
	},
}

type dependencies struct {
	client           httpDoer
	endpoint         string
	getenv           func(string) string
	readFile         func(string) ([]byte, error)
	now              func() time.Time
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

type approachMode string

const (
	approachNone       approachMode = "none"
	approachStepByStep approachMode = "step-by-step"
	approachSelfPrompt approachMode = "self-prompt"
	approachMultiRole  approachMode = "multi-role"
)

const validApproaches = "none, step-by-step, self-prompt, multi-role"

var defaultMultiRoleRoles = []string{"Business analyst", "Engineer", "Critic"}

type approachValue struct {
	value approachMode
}

func (a *approachValue) String() string { return string(a.value) }

func (a *approachValue) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("approach is empty; valid approaches: %s", validApproaches)
	}

	mode := approachMode(value)
	switch mode {
	case approachNone, approachStepByStep, approachSelfPrompt, approachMultiRole:
		a.value = mode
		return nil
	default:
		return fmt.Errorf("unknown approach %q; valid approaches: %s", value, validApproaches)
	}
}

type temperatureValue struct {
	value float64
	set   bool
}

type modelValue struct {
	value string
}

func (m *modelValue) String() string { return m.value }

func (m *modelValue) Set(value string) error {
	switch value {
	case flashModel, proModel:
		m.value = value
		return nil
	default:
		return fmt.Errorf("unknown model %q; valid models: %s, %s", value, flashModel, proModel)
	}
}

type reasoningMode string

const (
	reasoningNone reasoningMode = "none"
	reasoningLow  reasoningMode = "low"
	reasoningHigh reasoningMode = "high"
	reasoningMax  reasoningMode = "max"
)

const validReasoning = "none, low, high, max"

type reasoningValue struct {
	value reasoningMode
}

func (r *reasoningValue) String() string { return string(r.value) }

func (r *reasoningValue) Set(value string) error {
	mode := reasoningMode(value)
	switch mode {
	case reasoningNone, reasoningLow, reasoningHigh, reasoningMax:
		r.value = mode
		return nil
	default:
		return fmt.Errorf("unknown reasoning effort %q; valid values: %s", value, validReasoning)
	}
}

func (t *temperatureValue) String() string {
	if !t.set {
		return ""
	}
	return strconv.FormatFloat(t.value, 'g', -1, 64)
}

func (t *temperatureValue) Set(value string) error {
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 || parsed > 2 {
		return errors.New("temperature must be a number between 0 and 2")
	}
	t.value = parsed
	t.set = true
	return nil
}

type chatRequest struct {
	Model           string        `json:"model"`
	Messages        []chatMessage `json:"messages"`
	Thinking        thinkingMode  `json:"thinking"`
	ReasoningEffort string        `json:"reasoning_effort,omitempty"`
	Stream          bool          `json:"stream"`
	Temperature     *float64      `json:"temperature,omitempty"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type thinkingMode struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Model   string     `json:"model"`
	Usage   tokenUsage `json:"usage"`
	Choices []struct {
		Message struct {
			Content *string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type tokenUsage struct {
	CompletionTokens        int `json:"completion_tokens"`
	PromptTokens            int `json:"prompt_tokens"`
	PromptCacheHitTokens    int `json:"prompt_cache_hit_tokens"`
	PromptCacheMissTokens   int `json:"prompt_cache_miss_tokens"`
	TotalTokens             int `json:"total_tokens"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

type pricingBand string

const (
	pricingOffPeak pricingBand = "off-peak"
	pricingPeak    pricingBand = "peak"
)

type tokenRates struct {
	cacheHit  float64
	cacheMiss float64
	output    float64
}

type requestMetrics struct {
	duration time.Duration
	usage    tokenUsage
	band     pricingBand
	costUSD  float64
}

type aggregateMetrics struct {
	requests    int
	duration    time.Duration
	usage       tokenUsage
	costUSD     float64
	pricingBand string
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
		now:              time.Now,
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
	var roles promptValue
	approach := approachValue{value: approachNone}
	model := modelValue{value: defaultModel}
	reasoning := reasoningValue{value: reasoningNone}
	var temperature temperatureValue
	var debug bool
	var stats bool
	var help bool
	fs.Var(&prompt, "p", "prompt to send (takes precedence over stdin)")
	fs.Var(&prompt, "prompt", "prompt to send (takes precedence over stdin)")
	fs.Var(&formatPath, "f", "file containing the required final-answer format")
	fs.Var(&formatPath, "format", "file containing the required final-answer format")
	fs.Var(&length, "l", "desired final-answer length")
	fs.Var(&length, "length", "desired final-answer length")
	fs.Var(&stop, "s", "sequence that ends clarification mode")
	fs.Var(&stop, "stop", "sequence that ends clarification mode")
	fs.Var(&approach, "a", "prompt approach: "+validApproaches)
	fs.Var(&approach, "approach", "prompt approach: "+validApproaches)
	fs.Var(&roles, "roles", "comma-separated roles for the multi-role approach")
	fs.Var(&model, "m", "model: "+flashModel+" or "+proModel)
	fs.Var(&model, "model", "model: "+flashModel+" or "+proModel)
	fs.Var(&reasoning, "r", "reasoning effort: "+validReasoning)
	fs.Var(&reasoning, "reasoning", "reasoning effort: "+validReasoning)
	fs.Var(&temperature, "t", "sampling temperature from 0 to 2")
	fs.Var(&temperature, "temperature", "sampling temperature from 0 to 2")
	fs.BoolVar(&debug, "d", false, "print masked HTTP diagnostics to stderr")
	fs.BoolVar(&debug, "debug", false, "print masked HTTP diagnostics to stderr")
	fs.BoolVar(&stats, "stats", false, "print timing, token usage, and estimated cost to stderr")
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
	if roles.set && approach.value != approachMultiRole {
		fmt.Fprintln(stderr, "error: --roles requires --approach multi-role")
		return 2
	}
	if temperature.set && reasoning.value != reasoningNone {
		fmt.Fprintln(stderr, "error: --temperature cannot be used when reasoning is enabled")
		return 2
	}
	var selectedRoles []string
	if approach.value == approachMultiRole {
		selectedRoles = append([]string(nil), defaultMultiRoleRoles...)
		if roles.set {
			var err error
			selectedRoles, err = parseRoles(roles.value)
			if err != nil {
				fmt.Fprintf(stderr, "error: %v\n", err)
				return 2
			}
		}
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
	if systemMessage := buildSystemMessage(formatPath.set, format, length, stop, approach.value, selectedRoles); systemMessage != "" {
		messages = append(messages, chatMessage{Role: "system", Content: systemMessage})
	}
	messages = append(messages, chatMessage{Role: "user", Content: promptText})
	showSpinner := deps.interactiveStdin && (stop.set || !prompt.set)
	var selectedTemperature *float64
	if temperature.set {
		selectedTemperature = &temperature.value
	}
	metrics := aggregateMetrics{}

	for {
		final := !stop.set || containsFold(messages[len(messages)-1].Content, stop.value)
		requestMessages := messages
		if stop.set {
			requestMessages = withClarificationTurnStatus(messages, final, approach.value)
		}
		var requestTemperature *float64
		if final && approach.value != approachSelfPrompt {
			requestTemperature = selectedTemperature
		}
		answer, requestStats, ok := requestAnswer(requestMessages, model.value, reasoning.value, requestTemperature, debug, showSpinner, stderr, deps, apiKey)
		if !ok {
			return 1
		}
		metrics.add(requestStats)

		if final {
			if approach.value == approachSelfPrompt {
				finalMessages := make([]chatMessage, 0, 2)
				if systemMessage := buildSystemMessage(formatPath.set, format, length, promptValue{}, approachNone, nil); systemMessage != "" {
					finalMessages = append(finalMessages, chatMessage{Role: "system", Content: systemMessage})
				}
				finalMessages = append(finalMessages, chatMessage{Role: "user", Content: answer})
				answer, requestStats, ok = requestAnswer(finalMessages, model.value, reasoning.value, selectedTemperature, debug, showSpinner, stderr, deps, apiKey)
				if !ok {
					return 1
				}
				metrics.add(requestStats)
			}
			if err := writeAnswer(stdout, answer); err != nil {
				fmt.Fprintf(stderr, "error: write answer: %v\n", err)
				return 1
			}
			if stats {
				writeStats(stderr, model.value, reasoning.value, metrics)
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

func parseRoles(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("roles are empty")
	}

	parts := strings.Split(value, ",")
	roles := make([]string, 0, len(parts))
	for index, part := range parts {
		role := strings.TrimSpace(part)
		if role == "" {
			return nil, fmt.Errorf("role %d is empty", index+1)
		}
		if strings.ContainsAny(role, "\r\n") {
			return nil, fmt.Errorf("role %d contains a line break", index+1)
		}
		roles = append(roles, role)
	}
	if len(roles) < 2 {
		return nil, errors.New("multi-role approach requires at least two roles")
	}
	return roles, nil
}

func buildSystemMessage(hasFormat bool, format string, length, stop promptValue, approach approachMode, roles []string) string {
	sections := make([]string, 0, 4)
	if hasFormat && approach != approachSelfPrompt {
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
	if length.set && approach != approachSelfPrompt {
		sections = append(sections, "Target final-answer length:\n"+length.value)
	}
	if instruction := approachInstruction(approach, roles, hasFormat); instruction != "" {
		sections = append(sections, instruction)
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

func approachInstruction(approach approachMode, roles []string, hasFormat bool) string {
	switch approach {
	case approachStepByStep:
		return "Final-answer approach:\n" +
			"Solve the request step by step and present the resulting steps clearly.\n" +
			"Follow all final-answer requirements above."
	case approachSelfPrompt:
		return "Prompt-generation approach:\n" +
			"At the prompt-generation stage, write a clear, self-contained prompt that\n" +
			"instructs another model to solve the user's request. Incorporate all relevant\n" +
			"information from the conversation.\n" +
			"Return only the rewritten prompt and do not solve the request."
	case approachMultiRole:
		var instruction strings.Builder
		instruction.WriteString("Multi-role final-answer approach:\n")
		instruction.WriteString("Analyze the request independently from each of these roles:\n")
		for index, role := range roles {
			fmt.Fprintf(&instruction, "%d. %s\n", index+1, role)
		}
		if hasFormat {
			instruction.WriteString("Within the supplied answer format, clearly attribute every answer to its role.\n")
		} else {
			instruction.WriteString("Use exactly these Markdown section headings, in this order:\n")
			for _, role := range roles {
				fmt.Fprintf(&instruction, "## %s\n", role)
			}
			instruction.WriteString("Under each heading, write a substantial response using 2-4 short paragraphs.\n")
			instruction.WriteString("Separate paragraphs with blank lines; do not compress a role's entire answer into one paragraph.\n")
			instruction.WriteString("When useful, cover the role's assessment, concrete recommendations, and risks or trade-offs.\n")
			instruction.WriteString("Use bullets, numbered steps, or tables when they make the answer easier to scan.\n")
			instruction.WriteString("Write each role's content only below its matching heading.\n")
		}
		instruction.WriteString("Do not add an unlabeled introduction, summary, or conclusion.")
		return instruction.String()
	default:
		return ""
	}
}

func containsFold(value, substring string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(substring))
}

func withClarificationTurnStatus(messages []chatMessage, final bool, approach approachMode) []chatMessage {
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
		if approach == approachSelfPrompt {
			status = "Clarification status for this turn:\n" +
				"The stop sequence has appeared in the latest user message.\n" +
				"Produce only the self-contained solving prompt now. Do not solve the request."
		}
	}
	result[0].Content += "\n\n" + status
	return result
}

func requestAnswer(messages []chatMessage, model string, reasoning reasoningMode, temperature *float64, debug, showSpinner bool, stderr io.Writer, deps dependencies, apiKey string) (string, requestMetrics, bool) {
	thinkingType := "disabled"
	reasoningEffort := ""
	if reasoning != reasoningNone {
		thinkingType = "enabled"
		reasoningEffort = string(reasoning)
	}
	body, err := json.Marshal(chatRequest{
		Model:           model,
		Messages:        messages,
		Thinking:        thinkingMode{Type: thinkingType},
		ReasoningEffort: reasoningEffort,
		Stream:          false,
		Temperature:     temperature,
	})
	if err != nil {
		fmt.Fprintf(stderr, "error: encode request: %v\n", err)
		return "", requestMetrics{}, false
	}

	req, err := http.NewRequest(http.MethodPost, deps.endpoint, bytes.NewReader(body))
	if err != nil {
		fmt.Fprintf(stderr, "error: create request: %v\n", redact(err.Error(), apiKey))
		return "", requestMetrics{}, false
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
	now := deps.now
	if now == nil {
		now = time.Now
	}
	startedAt := now()
	resp, err := deps.client.Do(req)
	if err != nil {
		stopSpinner()
		fmt.Fprintf(stderr, "error: send request: %s\n", redact(err.Error(), apiKey))
		return "", requestMetrics{}, false
	}
	responseBody, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	finishedAt := now()
	stopSpinner()
	if readErr != nil {
		fmt.Fprintf(stderr, "error: read response: %s\n", redact(readErr.Error(), apiKey))
		return "", requestMetrics{}, false
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
		return "", requestMetrics{}, false
	}

	var result chatResponse
	if err := json.Unmarshal(responseBody, &result); err != nil {
		fmt.Fprintf(stderr, "error: decode DeepSeek response: %s\n", redact(err.Error(), apiKey))
		return "", requestMetrics{}, false
	}
	if len(result.Choices) == 0 || result.Choices[0].Message.Content == nil {
		fmt.Fprintln(stderr, "error: DeepSeek response does not contain an answer")
		return "", requestMetrics{}, false
	}
	answer := *result.Choices[0].Message.Content
	if answer == "" {
		fmt.Fprintln(stderr, "error: DeepSeek returned an empty answer")
		return "", requestMetrics{}, false
	}
	duration := finishedAt.Sub(startedAt)
	if duration < 0 {
		duration = 0
	}
	band := pricingBandAt(startedAt)
	requestStats := requestMetrics{
		duration: duration,
		usage:    result.Usage,
		band:     band,
		costUSD:  estimateCostUSD(model, band, result.Usage),
	}
	return answer, requestStats, true
}

func pricingBandAt(at time.Time) pricingBand {
	utc := at.UTC()
	weekday := utc.Weekday()
	if weekday == time.Saturday || weekday == time.Sunday {
		return pricingOffPeak
	}
	hour := utc.Hour()
	if (hour >= 1 && hour < 4) || (hour >= 6 && hour < 10) {
		return pricingPeak
	}
	return pricingOffPeak
}

func estimateCostUSD(model string, band pricingBand, usage tokenUsage) float64 {
	rates := modelPricing[model][band]
	return (float64(usage.PromptCacheHitTokens)*rates.cacheHit +
		float64(usage.PromptCacheMissTokens)*rates.cacheMiss +
		float64(usage.CompletionTokens)*rates.output) / 1_000_000
}

func (a *aggregateMetrics) add(request requestMetrics) {
	a.requests++
	a.duration += request.duration
	a.usage.PromptTokens += request.usage.PromptTokens
	a.usage.PromptCacheHitTokens += request.usage.PromptCacheHitTokens
	a.usage.PromptCacheMissTokens += request.usage.PromptCacheMissTokens
	a.usage.CompletionTokens += request.usage.CompletionTokens
	a.usage.CompletionTokensDetails.ReasoningTokens += request.usage.CompletionTokensDetails.ReasoningTokens
	a.usage.TotalTokens += request.usage.TotalTokens
	a.costUSD += request.costUSD
	if a.pricingBand == "" {
		a.pricingBand = string(request.band)
	} else if a.pricingBand != string(request.band) {
		a.pricingBand = "mixed"
	}
}

func writeStats(w io.Writer, model string, reasoning reasoningMode, metrics aggregateMetrics) {
	fmt.Fprintln(w, "Stats:")
	fmt.Fprintf(w, "  model: %s\n", model)
	fmt.Fprintf(w, "  reasoning: %s\n", reasoning)
	fmt.Fprintf(w, "  requests: %d\n", metrics.requests)
	fmt.Fprintf(w, "  answer time: %s\n", metrics.duration.Round(time.Millisecond))
	fmt.Fprintf(w, "  input tokens: %d (cache hit: %d, cache miss: %d)\n",
		metrics.usage.PromptTokens, metrics.usage.PromptCacheHitTokens, metrics.usage.PromptCacheMissTokens)
	fmt.Fprintf(w, "  output tokens: %d (reasoning: %d)\n",
		metrics.usage.CompletionTokens, metrics.usage.CompletionTokensDetails.ReasoningTokens)
	fmt.Fprintf(w, "  total tokens: %d\n", metrics.usage.TotalTokens)
	fmt.Fprintf(w, "  pricing: %s\n", metrics.pricingBand)
	fmt.Fprintf(w, "  estimated cost: $%.8f USD\n", metrics.costUSD)
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
  -a, --approach MODE Prompt approach: none (default), step-by-step,
                      self-prompt, or multi-role.
      --roles LIST    Comma-separated roles for multi-role (default:
                      "Business analyst, Engineer, Critic").
  -m, --model MODEL   Model: deepseek-v4-flash (default) or deepseek-v4-pro.
  -r, --reasoning N   Reasoning effort: none (default), low, high, or max.
  -t, --temperature N Sampling temperature from 0 to 2. If omitted, do not send
                      a temperature value and use the API default. Cannot be
                      combined with enabled reasoning.
      --stats         Print timing, token usage, and estimated cost to stderr.
  -d, --debug         Print masked HTTP request/response details to stderr.
  -h, --help          Show this help.

Environment:
  DEEPSEEK_API_KEY   DeepSeek API key (required).

Examples:
  deepseek-asker --prompt "Explain goroutines briefly"
  printf 'Explain goroutines briefly' | deepseek-asker
  deepseek-asker -p "Compare Go and Rust" -l "3 paragraphs"
  deepseek-asker -p "Write a surprising story" --temperature 1.3
  deepseek-asker -p "Solve this problem" --model deepseek-v4-pro \
    --reasoning max --stats
  deepseek-asker -p "Review this proposal" --approach multi-role
  deepseek-asker -p "Design an army palette" -a multi-role \
    --roles "Miniature painter, Lore expert, Critic"
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
