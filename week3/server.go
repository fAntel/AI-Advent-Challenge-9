package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Server struct {
	Agent     *Agent
	Shutdown  func()
	Scheduler *Scheduler
	Pipelines *PipelineStore
	Events    *EventHub
}

const maxRequestBodyBytes = 8 << 20

func (s Server) Handler() http.Handler { return http.HandlerFunc(s.serveHTTP) }
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBodyBytes))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
func token(r *http.Request) string { return r.Header.Get("X-Agent-Lease") }
func profile(r *http.Request) string {
	v := r.Header.Get("X-Agent-Profile")
	if v == "" {
		return DefaultProfile
	}
	return v
}
func (s Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
	if strings.HasPrefix(path, "v1/mcp") {
		s.serveMCP(w, r, path)
		return
	}
	if strings.HasPrefix(path, "v1/pipelines") && s.Pipelines != nil {
		s.servePipelines(w, r, path)
		return
	}
	if path == "v1/events" && r.Method == http.MethodGet {
		if s.Events == nil {
			w.WriteHeader(404)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		flusher, ok := w.(http.Flusher)
		if !ok {
			w.WriteHeader(500)
			return
		}
		ch, unsubscribe := s.Events.Subscribe()
		defer unsubscribe()
		w.WriteHeader(http.StatusOK)
		flusher.Flush()
		for {
			select {
			case <-r.Context().Done():
				return
			case n := <-ch:
				data, _ := json.Marshal(n)
				_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
				flusher.Flush()
			case <-time.After(25 * time.Second):
				_, _ = w.Write([]byte(": keepalive\n\n"))
				flusher.Flush()
			}
		}
	}
	if path == "v1/schedules" && s.Scheduler != nil {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, s.Scheduler.List())
		case http.MethodPost:
			var j ScheduleJob
			if e := decodeJSON(w, r, &j); e != nil {
				s.respond(w, nil, e, 0)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), s.Agent.requestTimeout)
			defer cancel()
			s.respond(w, map[string]bool{"ok": true}, s.Scheduler.Add(ctx, j), 201)
		default:
			w.WriteHeader(405)
		}
		return
	}
	if path == "v1/schedules/reload" && s.Scheduler != nil && r.Method == http.MethodPost {
		jobs, err := s.Scheduler.Reload()
		s.respond(w, jobs, err, 200)
		return
	}
	if strings.HasPrefix(path, "v1/schedules/") && s.Scheduler != nil {
		name := strings.TrimPrefix(path, "v1/schedules/")
		if r.Method == http.MethodDelete {
			s.respond(w, map[string]bool{"ok": true}, s.Scheduler.Remove(name), 200)
		} else if r.Method == http.MethodPost && strings.HasSuffix(name, "/run") {
			name = strings.TrimSuffix(name, "/run")
			if err := s.Scheduler.Run(name); err != nil {
				s.respond(w, nil, err, 0)
				return
			}
			for _, job := range s.Scheduler.List() {
				if job.Name == name {
					writeJSON(w, 200, job)
					return
				}
			}
			writeJSON(w, 200, map[string]bool{"completed": true})
		} else {
			w.WriteHeader(405)
		}
		return
	}
	if path == "v1/usage" && r.Method == http.MethodGet {
		end := time.Now()
		hour, day, err := s.Agent.UsageReport(end)
		s.respond(w, map[string]any{"last_hour": hour, "today": day, "generated_at": end}, err, 200)
		return
	}
	if path == "v1/shutdown" {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		writeJSON(w, http.StatusAccepted, map[string]bool{"stopping": true})
		if s.Shutdown != nil {
			s.Shutdown()
		}
		return
	}
	parts := strings.Split(path, "/")
	if len(parts) < 2 || parts[0] != "v1" || parts[1] != "sessions" {
		writeJSON(w, 404, ErrorResponse{"not found"})
		return
	}
	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			list, err := s.Agent.ListProfile(profile(r))
			s.respond(w, list, err, 200)
		case http.MethodPost:
			var req CreateSessionRequest
			if err := decodeJSON(w, r, &req); err != nil {
				s.respond(w, nil, err, 0)
				return
			}
			req.Profile = profile(r)
			session, err := s.Agent.Create(req)
			s.respond(w, session, err, 201)
		default:
			w.WriteHeader(405)
		}
		return
	}
	id := parts[2]
	existing, profileErr := s.Agent.Get(id)
	if profileErr != nil || existing.Profile != profile(r) {
		s.respond(w, nil, ErrNotFound, 0)
		return
	}
	if len(parts) == 3 {
		switch r.Method {
		case http.MethodGet:
			v, e := s.Agent.Get(id)
			s.respond(w, v, e, 200)
		case http.MethodDelete:
			s.respond(w, map[string]bool{"deleted": true}, s.Agent.Delete(id), 200)
		default:
			w.WriteHeader(405)
		}
		return
	}
	action := parts[3]
	switch action {
	case "attach":
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		v, e := s.Agent.Attach(id)
		s.respond(w, LeaseResponse{Token: v}, e, 200)
	case "heartbeat":
		s.respond(w, map[string]bool{"ok": true}, s.Agent.Heartbeat(id, token(r)), 200)
	case "detach":
		s.respond(w, map[string]bool{"ok": true}, s.Agent.Detach(id, token(r)), 200)
	case "messages":
		var req MessageRequest
		if e := decodeJSON(w, r, &req); e != nil {
			s.respond(w, nil, e, 0)
			return
		}
		s.respond(w, map[string]string{"state": "running"}, s.Agent.Submit(id, token(r), req), 202)
	case "retry":
		s.respond(w, map[string]string{"state": "running"}, s.Agent.Retry(id, token(r)), 202)
	case "tool-approval":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Approve bool `json:"approve"`
		}
		if e := decodeJSON(w, r, &req); e != nil {
			s.respond(w, nil, e, 0)
			return
		}
		s.respond(w, map[string]string{"state": "running"}, s.Agent.ApproveTool(id, token(r), req.Approve), 202)
	case "discard":
		s.respond(w, map[string]bool{"ok": true}, s.Agent.Discard(id, token(r)), 200)
	case "continue":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.respond(w, map[string]string{"state": "running"}, s.Agent.ContinueTask(id, token(r)), http.StatusAccepted)
	case "back":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req TaskBackRequest
		if e := decodeJSON(w, r, &req); e != nil {
			s.respond(w, nil, e, 0)
			return
		}
		s.respond(w, map[string]string{"state": "running"}, s.Agent.BackTask(id, token(r), req.Phase), http.StatusAccepted)
	case "checkpoints":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req CheckpointRequest
		if e := decodeJSON(w, r, &req); e != nil {
			s.respond(w, nil, e, 0)
			return
		}
		checkpoint, e := s.Agent.CreateCheckpoint(id, token(r), req.Name)
		s.respond(w, checkpoint, e, http.StatusCreated)
	case "branch":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req BranchRequest
		if e := decodeJSON(w, r, &req); e != nil {
			s.respond(w, nil, e, 0)
			return
		}
		branch, e := s.Agent.CreateBranch(id, token(r), req)
		s.respond(w, branch, e, http.StatusCreated)
	case "branches":
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		branches, e := s.Agent.RelatedBranches(id)
		s.respond(w, branches, e, http.StatusOK)
	case "memory":
		if r.Method == http.MethodGet {
			view, e := s.Agent.Memory(id)
			s.respond(w, view, e, http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req MemoryMutationRequest
		if e := decodeJSON(w, r, &req); e != nil {
			s.respond(w, nil, e, 0)
			return
		}
		s.respond(w, map[string]bool{"ok": true}, s.Agent.MutateMemory(id, token(r), req), http.StatusOK)
	case "invariants":
		if r.Method == http.MethodGet {
			view, e := s.Agent.Invariants(id)
			s.respond(w, view, e, http.StatusOK)
			return
		}
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		var req InvariantMutationRequest
		if e := decodeJSON(w, r, &req); e != nil {
			s.respond(w, nil, e, 0)
			return
		}
		s.respond(w, map[string]bool{"ok": true}, s.Agent.MutateInvariant(id, token(r), req), http.StatusOK)
	case "clear":
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		s.respond(w, map[string]bool{"ok": true}, s.Agent.Clear(id, token(r)), http.StatusOK)
	default:
		writeJSON(w, 404, ErrorResponse{"not found"})
	}
}

func (s Server) servePipelines(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(path, "/")
	if len(parts) == 2 {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, 200, s.Pipelines.List())
		case http.MethodPost:
			var input struct {
				Name string `json:"name"`
			}
			if err := decodeJSON(w, r, &input); err != nil {
				s.respond(w, nil, err, 0)
				return
			}
			s.respond(w, map[string]bool{"ok": true}, s.Pipelines.Add(input.Name), 201)
		default:
			w.WriteHeader(405)
		}
		return
	}
	if len(parts) < 3 || len(parts) > 5 {
		writeJSON(w, 404, ErrorResponse{"not found"})
		return
	}
	name := parts[2]
	if len(parts) == 3 {
		switch r.Method {
		case http.MethodGet:
			item, err := s.Pipelines.Get(name)
			s.respond(w, item, err, 200)
		case http.MethodDelete:
			if s.Scheduler != nil && s.Scheduler.UsesPipeline(name) {
				s.respond(w, nil, fmt.Errorf("pipeline %q is used by a schedule", name), 0)
				return
			}
			s.respond(w, map[string]bool{"ok": true}, s.Pipelines.Remove(name), 200)
		default:
			w.WriteHeader(405)
		}
		return
	}
	if len(parts) == 4 && parts[3] == "run" && r.Method == http.MethodPost {
		ctx, cancel := context.WithTimeout(r.Context(), s.Agent.requestTimeout)
		defer cancel()
		run, err := s.Pipelines.Run(ctx, name)
		s.respond(w, run, err, 200)
		return
	}
	if len(parts) == 4 && parts[3] == "steps" && r.Method == http.MethodPost {
		var step PipelineStep
		if err := decodeJSON(w, r, &step); err != nil {
			s.respond(w, nil, err, 0)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.Agent.requestTimeout)
		defer cancel()
		s.respond(w, map[string]bool{"ok": true}, s.Pipelines.AddStep(ctx, name, step), 201)
		return
	}
	if len(parts) == 5 && parts[3] == "steps" && r.Method == http.MethodDelete {
		s.respond(w, map[string]bool{"ok": true}, s.Pipelines.RemoveStep(name, parts[4]), 200)
		return
	}
	writeJSON(w, 404, ErrorResponse{"not found"})
}

func (s Server) serveMCP(w http.ResponseWriter, r *http.Request, path string) {
	parts := strings.Split(path, "/")
	if len(parts) == 3 && parts[2] == "call" && r.Method == http.MethodPost {
		var input struct {
			Server    string         `json:"server"`
			Tool      string         `json:"tool"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := decodeJSON(w, r, &input); err != nil {
			s.respond(w, nil, err, 0)
			return
		}
		if input.Arguments == nil || !mcpName.MatchString(input.Server) || !mcpName.MatchString(input.Tool) {
			s.respond(w, nil, errors.New("server, tool, and object arguments are required"), 0)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), s.Agent.requestTimeout)
		defer cancel()
		tools, err := s.Agent.MCP.Tools(ctx, input.Server, false)
		if err != nil {
			s.respond(w, nil, err, 0)
			return
		}
		var found bool
		for _, tool := range tools {
			if tool.Name == input.Tool {
				found = true
				if err := validateMCPArguments(tool.InputSchema, input.Arguments); err != nil {
					s.respond(w, nil, err, 0)
					return
				}
				break
			}
		}
		if !found {
			s.respond(w, nil, fmt.Errorf("MCP tool %s/%s not found", input.Server, input.Tool), 0)
			return
		}
		result, err := s.Agent.MCP.Call(ctx, input.Server, input.Tool, input.Arguments)
		if err == nil && result == nil {
			err = errors.New("MCP tool returned no result")
		}
		s.respond(w, result, err, 200)
		return
	}
	if len(parts) == 2 && r.Method == http.MethodGet {
		writeJSON(w, 200, s.Agent.MCP.List())
		return
	}
	if len(parts) == 2 && r.Method == http.MethodPost {
		var req struct {
			Name, Description string
			Command           []string
		}
		if err := decodeJSON(w, r, &req); err != nil {
			s.respond(w, nil, err, 0)
			return
		}
		s.respond(w, map[string]bool{"ok": true}, s.Agent.MCP.Add(req.Name, req.Description, req.Command), 201)
		return
	}
	if len(parts) < 3 || len(parts) > 4 {
		writeJSON(w, 404, ErrorResponse{"not found"})
		return
	}
	name := parts[2]
	if len(parts) == 3 && r.Method == http.MethodDelete {
		s.respond(w, map[string]bool{"ok": true}, s.Agent.MCP.Remove(name), 200)
		return
	}
	if len(parts) == 4 && parts[3] == "tools" && r.Method == http.MethodGet {
		ctx, cancel := context.WithTimeout(r.Context(), s.Agent.requestTimeout)
		defer cancel()
		tools, err := s.Agent.MCP.Tools(ctx, name, r.URL.Query().Get("refresh") == "true")
		s.respond(w, tools, err, 200)
		return
	}
	writeJSON(w, 404, ErrorResponse{"not found"})
}
func (s Server) respond(w http.ResponseWriter, v any, err error, success int) {
	if err == nil {
		writeJSON(w, success, v)
		return
	}
	status := 400
	if errors.Is(err, ErrNotFound) {
		status = 404
	} else if errors.Is(err, ErrConflict) || errors.Is(err, ErrInvalidTransition) {
		status = 409
	}
	writeJSON(w, status, ErrorResponse{err.Error()})
}
