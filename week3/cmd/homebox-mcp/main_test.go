package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestToolsAgainstHomeBoxAPI(t *testing.T) {
	type observed struct{ Method, Path, Query, Body string }
	var calls []observed
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("authorization: %q", got)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("accept: %q", got)
		}
		var body map[string]any
		if r.Method == "POST" || r.Method == "PATCH" {
			if got := r.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("content type: %q", got)
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
		}
		encoded, _ := json.Marshal(body)
		calls = append(calls, observed{r.Method, r.URL.Path, r.URL.RawQuery, string(encoded)})
		if r.URL.Path == "/api/v1/entities/missing" {
			http.Error(w, "entity not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == "POST" {
			w.WriteHeader(http.StatusCreated)
		}
		if r.URL.Path == "/api/v1/entity-types" || r.URL.Path == "/api/v1/tags" {
			_, _ = w.Write([]byte(`[{"id":"entity-7","name":"Item"}]`))
			return
		}
		_, _ = w.Write([]byte(`{"id":"entity-7","name":"Desk lamp","total":1}`))
	}))
	defer api.Close()
	h, err := newHomebox(config{URL: api.URL, APIKey: "test-key"})
	if err != nil {
		t.Fatal(err)
	}
	h.http = api.Client()
	s := newServer(h)
	ctx := context.Background()
	ct, st := mcp.NewInMemoryTransports()
	if _, err := s.Connect(ctx, st, nil); err != nil {
		t.Fatal(err)
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	listed, err := cs.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tools) != 6 {
		t.Fatalf("tool count: %d", len(listed.Tools))
	}
	for _, tool := range listed.Tools {
		if tool.Annotations == nil || tool.Annotations.ReadOnlyHint != (tool.Name != "create_entity" && tool.Name != "update_entity") {
			t.Fatalf("annotations for %s: %+v", tool.Name, tool.Annotations)
		}
		if tool.InputSchema == nil || tool.Description == "" {
			t.Fatalf("missing schema or description: %s", tool.Name)
		}
	}
	tests := []struct {
		name string
		args map[string]any
		want observed
	}{
		{"search_entities", map[string]any{"query": "desk lamp", "page": 2, "pageSize": 5, "tags": []string{"red", "blue"}, "parentIds": []string{"shelf"}}, observed{"GET", "/api/v1/entities", "page=2&pageSize=5&parentIds=shelf&q=desk+lamp&tags=red&tags=blue", "null"}},
		{"get_entity", map[string]any{"id": "entity-7"}, observed{"GET", "/api/v1/entities/entity-7", "", "null"}},
		{"list_entity_types", map[string]any{}, observed{"GET", "/api/v1/entity-types", "", "null"}},
		{"list_tags", map[string]any{}, observed{"GET", "/api/v1/tags", "", "null"}},
		{"create_entity", map[string]any{"name": "Desk lamp", "description": "brass", "entityTypeId": "type-1", "parentId": "room-1", "quantity": 2, "tagIds": []string{"tag-1"}}, observed{"POST", "/api/v1/entities", "", `{"description":"brass","entityTypeId":"type-1","name":"Desk lamp","parentId":"room-1","quantity":2,"tagIds":["tag-1"]}`}},
		{"update_entity", map[string]any{"id": "entity-7", "quantity": 0, "tagIds": []string{}}, observed{"PATCH", "/api/v1/entities/entity-7", "", `{"id":"entity-7","quantity":0,"tagIds":[]}`}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil || res.IsError {
				t.Fatalf("call: %+v %v", res, err)
			}
			if len(calls) == 0 || !reflect.DeepEqual(calls[len(calls)-1], tc.want) {
				t.Fatalf("request: got %+v, want %+v", calls[len(calls)-1], tc.want)
			}
			data, _ := json.Marshal(res.StructuredContent)
			if !strings.Contains(string(data), `"entity-7"`) {
				t.Fatalf("result: %s", data)
			}
		})
	}
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_entity", Arguments: map[string]any{"id": "missing"}})
	if err != nil || !res.IsError || !strings.Contains(res.Content[0].(*mcp.TextContent).Text, "HTTP 404") {
		t.Fatalf("API error: %+v %v", res, err)
	}
	before := len(calls)
	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_entity", Arguments: map[string]any{"id": "../tags"}})
	if err != nil || !res.IsError || len(calls) != before {
		t.Fatalf("invalid ID reached API: %+v %v", res, err)
	}
}

func TestConfigValidation(t *testing.T) {
	file := filepath.Join(t.TempDir(), "homebox.json")
	if err := os.WriteFile(file, []byte(`{"url":"https://homebox.example","apiKey":"secret"}`), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadConfig(file); err == nil {
		t.Fatal("accepted public config")
	}
	if err := os.Chmod(file, 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := loadConfig(file)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newHomebox(cfg); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"http://homebox.example", "https://user:password@homebox.example", "https://homebox.example?token=x"} {
		cfg.URL = bad
		if _, err := newHomebox(cfg); err == nil {
			t.Fatalf("accepted %q", bad)
		}
	}
}
