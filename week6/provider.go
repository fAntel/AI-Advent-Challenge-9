package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}
type Usage struct {
	Prompt     int `json:"prompt_tokens"`
	Completion int `json:"completion_tokens"`
	Total      int `json:"total_tokens"`
}
type Call struct {
	Purpose  string        `json:"purpose"`
	Usage    Usage         `json:"usage"`
	Duration time.Duration `json:"duration"`
	Finish   string        `json:"finish"`
}
type CompleteFunc func(context.Context, string, []Message) (string, Call, error)

func Provider(c Config) CompleteFunc {
	return func(ctx context.Context, purpose string, messages []Message) (string, Call, error) {
		call := Call{Purpose: purpose}
		key := os.Getenv("DEEPSEEK_API_KEY")
		if key == "" {
			key, _ = KeychainDeepSeekKey()
		}
		if key == "" {
			return "", call, fmt.Errorf("set DEEPSEEK_API_KEY or the advent-agent.deepseek Keychain entry")
		}
		payload := map[string]any{"model": c.Model, "messages": messages, "thinking": map[string]string{"type": "disabled"}, "stream": false, "max_tokens": 2048, "temperature": 0}
		if purpose != "summary" {
			payload["response_format"] = map[string]string{"type": "json_object"}
		}
		body, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, "POST", c.Endpoint, bytes.NewReader(body))
		if err != nil {
			return "", call, err
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Content-Type", "application/json")
		client := &http.Client{Timeout: c.Timeout(), CheckRedirect: func(*http.Request, []*http.Request) error { return fmt.Errorf("provider redirect refused") }}
		start := time.Now()
		resp, err := client.Do(req)
		call.Duration = time.Since(start)
		if err != nil {
			return "", call, err
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
		if err != nil {
			return "", call, err
		}
		if resp.StatusCode != 200 {
			return "", call, fmt.Errorf("DeepSeek HTTP %d: %s", resp.StatusCode, strings.ReplaceAll(string(raw), key, "[redacted]"))
		}
		var out struct {
			Usage   Usage `json:"usage"`
			Choices []struct {
				Message Message `json:"message"`
				Finish  string  `json:"finish_reason"`
			} `json:"choices"`
		}
		if err = json.Unmarshal(raw, &out); err != nil {
			return "", call, err
		}
		call.Usage = out.Usage
		if len(out.Choices) != 1 {
			return "", call, fmt.Errorf("provider returned no unique answer")
		}
		call.Finish = out.Choices[0].Finish
		answer := out.Choices[0].Message.Content
		if call.Finish != "stop" || strings.TrimSpace(answer) == "" {
			return answer, call, fmt.Errorf("incomplete provider answer (finish=%s)", call.Finish)
		}
		return answer, call, nil
	}
}
