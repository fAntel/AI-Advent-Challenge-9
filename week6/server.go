package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

type Request struct {
	ID       string `json:"id"`
	Project  string `json:"project"`
	Question string `json:"question"`
}
type Response struct {
	Session *Session `json:"session,omitempty"`
	Error   string   `json:"error,omitempty"`
}

func Handler(e *Engine, stop context.CancelFunc) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "week6-v2") })
	mux.HandleFunc("POST /stop", func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "stopping"); go stop() })
	for _, action := range []string{"create", "get", "ask", "retry", "discard"} {
		mux.HandleFunc("POST /"+action, func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			var req Request
			var res Response
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				http.Error(w, "invalid request", 400)
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), e.Config.Timeout())
			defer cancel()
			var s Session
			var err error
			switch action {
			case "create":
				if !filepath.IsAbs(req.Project) {
					err = fmt.Errorf("project must be absolute")
				} else {
					s, err = e.Create(req.Project)
				}
			case "get":
				s, err = e.Get(req.ID)
			case "ask":
				s, err = e.Ask(ctx, req.ID, req.Question, false)
			case "retry":
				s, err = e.Ask(ctx, req.ID, "", true)
			case "discard":
				s, err = e.Discard(req.ID)
			}
			if s.ID != "" {
				res.Session = &s
			}
			if err != nil {
				res.Error = err.Error()
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(res)
		})
	}
	return mux
}
func Serve(ctx context.Context, c Config) error {
	if err := os.MkdirAll(c.State, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(c.State, "daemon.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return fmt.Errorf("daemon already running: %w", err)
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	if err = os.Remove(c.Socket()); err != nil && !os.IsNotExist(err) {
		return err
	}
	listener, err := net.Listen("unix", c.Socket())
	if err != nil {
		return err
	}
	defer listener.Close()
	defer os.Remove(c.Socket())
	if err = os.Chmod(c.Socket(), 0600); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	server := &http.Server{Handler: Handler(NewEngine(c), cancel), ReadHeaderTimeout: 5 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() {
		<-ctx.Done()
		shutdown, done := context.WithTimeout(context.Background(), 5*time.Second)
		defer done()
		_ = server.Shutdown(shutdown)
	}()
	err = server.Serve(listener)
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}
func HTTPClient(c Config) *http.Client {
	return &http.Client{Timeout: c.Timeout() + 10*time.Second, Transport: &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", c.Socket())
	}}}
}
func Health(ctx context.Context, c Config) error {
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://unix/health", nil)
	resp, err := HTTPClient(c).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err == nil && string(b) != "week6-v2" {
		err = fmt.Errorf("incompatible daemon")
	}
	return err
}
