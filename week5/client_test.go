package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestChatStream(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" || r.Method != "POST" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		var body struct {
			Model    string
			Messages []message
			Stream   bool
			Think    bool
			Options  map[string]any
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		if body.Model != "test" || len(body.Messages) != 1 || body.Messages[0].Content != "question" || !body.Stream || body.Think || body.Options["num_ctx"] != float64(4096) || body.Options["num_predict"] != float64(2048) {
			t.Errorf("unexpected body: %+v", body)
		}
		io.WriteString(w, `{"message":{"thinking":"reason"},"done":false}
{"message":{"content":"Hel"},"done":false}
{"message":{"content":"lo"},"done":true,"done_reason":"stop","total_duration":1000,"load_duration":100,"prompt_eval_count":3,"prompt_eval_duration":200,"eval_count":2,"eval_duration":700}`)
	}))
	defer s.Close()
	var out strings.Builder
	r, err := (client{s.URL, s.Client()}).chat(context.Background(), "test", task{"easy", "question"}, &out)
	if err != nil {
		t.Fatal(err)
	}
	if out.String() != "Hello" || r.Answer != "Hello" || r.Thinking != "reason" || !r.Done || r.Truncated || r.EvalCount != 2 || r.TotalDuration != 1000 || r.LoadDuration != 100 || r.PromptEvalCount != 3 || r.PromptEvalDuration != 200 || r.EvalDuration != 700 || r.ElapsedSeconds <= 0 {
		t.Fatalf("unexpected result: %+v, output %q", r, out.String())
	}
}

func TestChatFailures(t *testing.T) {
	for _, tc := range []struct {
		name, body, contains string
		status               int
		truncated            bool
	}{
		{"HTTP", "out of memory", "HTTP 500", 500, false},
		{"malformed", "not JSON", "malformed", 200, false},
		{"interrupted", `{"message":{"content":"partial"},"done":false}`, "EOF", 200, false},
		{"generation error", `{"error":"runner stopped"}`, "runner stopped", 200, false},
		{"token limit", `{"message":{"content":"partial"},"done":true,"done_reason":"length"}`, "truncated", 200, true},
		{"empty", `{"done":true,"done_reason":"stop"}`, "empty answer", 200, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(tc.status); io.WriteString(w, tc.body) }))
			defer s.Close()
			r, err := (client{s.URL, s.Client()}).chat(context.Background(), "test", task{}, io.Discard)
			if err == nil || !strings.Contains(err.Error(), tc.contains) || r.Error != err.Error() || r.Truncated != tc.truncated {
				t.Fatalf("result %+v; error %v", r, err)
			}
			if tc.name == "interrupted" && r.Answer != "partial" {
				t.Fatal("partial response lost")
			}
		})
	}
}

func TestChatTimeout(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"message":{"content":"partial"},"done":false}`+"\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done()
	}))
	defer s.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	r, err := (client{s.URL, s.Client()}).chat(ctx, "test", task{}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "deadline exceeded") || r.Answer != "partial" || r.Done {
		t.Fatalf("result %+v; error %v", r, err)
	}
}

func TestPreflight(t *testing.T) {
	for _, missing := range []bool{false, true} {
		s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/version":
				io.WriteString(w, `{"version":"0.40.1"}`)
			case "/api/show":
				if missing {
					w.WriteHeader(404)
					io.WriteString(w, `{"error":"model not found"}`)
				} else {
					io.WriteString(w, `{"details":{"quantization_level":"Q4_K_M"}}`)
				}
			default:
				t.Errorf("unexpected path %s", r.URL.Path)
			}
		}))
		version, metadata, err := (client{s.URL, s.Client()}).preflight(context.Background(), "test")
		s.Close()
		if version != "0.40.1" {
			t.Fatal(version)
		}
		if missing {
			if err == nil || !strings.Contains(err.Error(), "ollama pull") {
				t.Fatal(err)
			}
		} else if err != nil || !strings.Contains(string(metadata), "Q4_K_M") {
			t.Fatalf("%s %v", metadata, err)
		}
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	s.Close()
	_, _, err := (client{s.URL, s.Client()}).preflight(context.Background(), "test")
	if err == nil || !strings.Contains(err.Error(), "server unavailable") {
		t.Fatal(err)
	}
}
