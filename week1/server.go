package agent

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type Server struct {
	Agent    *Agent
	Shutdown func()
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
func (s Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(r.URL.Path, "/")
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
			list, err := s.Agent.List()
			s.respond(w, list, err, 200)
		case http.MethodPost:
			var req CreateSessionRequest
			if err := decodeJSON(w, r, &req); err != nil {
				s.respond(w, nil, err, 0)
				return
			}
			session, err := s.Agent.Create(req)
			s.respond(w, session, err, 201)
		default:
			w.WriteHeader(405)
		}
		return
	}
	id := parts[2]
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
	case "discard":
		s.respond(w, map[string]bool{"ok": true}, s.Agent.Discard(id, token(r)), 200)
	default:
		writeJSON(w, 404, ErrorResponse{"not found"})
	}
}
func (s Server) respond(w http.ResponseWriter, v any, err error, success int) {
	if err == nil {
		writeJSON(w, success, v)
		return
	}
	status := 400
	if errors.Is(err, ErrNotFound) {
		status = 404
	} else if errors.Is(err, ErrConflict) {
		status = 409
	}
	writeJSON(w, status, ErrorResponse{err.Error()})
}
