package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const endpoint = "http://127.0.0.1:11434"
const defaultModel = "hf.co/XHToken/Spark-X2.5-4B-GGUF:Q4_K_M"

type client struct {
	base string
	http *http.Client
}
type message struct {
	Role     string `json:"role"`
	Content  string `json:"content"`
	Thinking string `json:"thinking,omitempty"`
}
type metrics struct {
	TotalDuration      int64 `json:"total_duration"`
	LoadDuration       int64 `json:"load_duration"`
	PromptEvalCount    int   `json:"prompt_eval_count"`
	PromptEvalDuration int64 `json:"prompt_eval_duration"`
	EvalCount          int   `json:"eval_count"`
	EvalDuration       int64 `json:"eval_duration"`
}
type result struct {
	task
	Answer         string  `json:"answer"`
	Thinking       string  `json:"thinking,omitempty"`
	Done           bool    `json:"done"`
	DoneReason     string  `json:"done_reason,omitempty"`
	Truncated      bool    `json:"truncated"`
	ElapsedSeconds float64 `json:"elapsed_seconds"`
	Error          string  `json:"error,omitempty"`
	metrics
}

func (c client) request(ctx context.Context, method, path string, body any) (*http.Response, error) {
	var data io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		data = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base+path, data)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("Ollama %s: HTTP %d: %s", path, resp.StatusCode, b)
	}
	return resp, nil
}

func (c client) json(ctx context.Context, method, path string, body, dst any) error {
	resp, err := c.request(ctx, method, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func (c client) preflight(ctx context.Context, model string) (string, json.RawMessage, error) {
	var version struct {
		Version string `json:"version"`
	}
	if err := c.json(ctx, "GET", "/api/version", nil, &version); err != nil {
		return "", nil, fmt.Errorf("server unavailable; start Ollama: %w", err)
	}
	var metadata json.RawMessage
	if err := c.json(ctx, "POST", "/api/show", map[string]string{"model": model}, &metadata); err != nil {
		return version.Version, nil, fmt.Errorf("cannot inspect model %q; install it with ollama pull: %w", model, err)
	}
	return version.Version, metadata, nil
}

func (c client) chat(ctx context.Context, model string, t task, out io.Writer) (r result, err error) {
	r.task = t
	start := time.Now()
	defer func() {
		r.ElapsedSeconds = time.Since(start).Seconds()
		if err != nil {
			r.Error = err.Error()
		}
	}()
	body := map[string]any{
		"model": model, "messages": []message{{Role: "user", Content: t.Prompt}},
		"stream": true, "think": false, "keep_alive": "10m",
		"options": map[string]any{"num_ctx": 4096, "num_predict": 2048, "temperature": 1.0, "top_p": 0.95, "top_k": -1},
	}
	resp, err := c.request(ctx, "POST", "/api/chat", body)
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)
	for {
		var chunk struct {
			Message    message `json:"message"`
			Done       bool    `json:"done"`
			DoneReason string  `json:"done_reason"`
			Error      string  `json:"error"`
			metrics
		}
		if e := decoder.Decode(&chunk); e != nil {
			return r, fmt.Errorf("incomplete or malformed response stream: %w", e)
		}
		if chunk.Error != "" {
			return r, fmt.Errorf("Ollama generation: %s", chunk.Error)
		}
		r.Answer += chunk.Message.Content
		r.Thinking += chunk.Message.Thinking
		if _, e := io.WriteString(out, chunk.Message.Content); e != nil {
			return r, fmt.Errorf("write answer: %w", e)
		}
		if chunk.Done {
			r.Done, r.DoneReason, r.metrics = true, chunk.DoneReason, chunk.metrics
			r.Truncated = chunk.DoneReason == "length"
			if r.Truncated {
				return r, fmt.Errorf("answer truncated at generation token limit")
			}
			if r.Answer == "" {
				return r, fmt.Errorf("model returned an empty answer")
			}
			return r, nil
		}
	}
}
