package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"
)

var ErrDaemonUnavailable = errors.New("deepseek-agentd is unavailable")

type APIClient struct {
	HTTP    *http.Client
	BaseURL string
}

func NewAPIClient(socket string, timeout time.Duration) *APIClient {
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	return &APIClient{HTTP: &http.Client{Transport: tr, Timeout: timeout}, BaseURL: "http://unix"}
}
func (c *APIClient) do(method, path, lease string, in, out any) (int, error) {
	var body io.Reader
	if in != nil {
		data, e := json.Marshal(in)
		if e != nil {
			return 0, e
		}
		body = bytes.NewReader(data)
	}
	req, e := http.NewRequest(method, c.BaseURL+path, body)
	if e != nil {
		return 0, e
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if lease != "" {
		req.Header.Set("X-Agent-Lease", lease)
	}
	resp, e := c.HTTP.Do(req)
	if e != nil {
		if errors.Is(e, os.ErrNotExist) || errors.Is(e, syscall.ECONNREFUSED) {
			return 0, fmt.Errorf("%w: Unix socket is not accepting connections", ErrDaemonUnavailable)
		}
		return 0, e
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var failure ErrorResponse
		_ = json.NewDecoder(resp.Body).Decode(&failure)
		if failure.Error == "" {
			failure.Error = resp.Status
		}
		return resp.StatusCode, fmt.Errorf("%s", failure.Error)
	}
	if out != nil {
		e = json.NewDecoder(resp.Body).Decode(out)
	}
	return resp.StatusCode, e
}
func (c *APIClient) Create(req CreateSessionRequest) (Session, error) {
	var s Session
	_, e := c.do("POST", "/v1/sessions", "", req, &s)
	return s, e
}
func (c *APIClient) List() ([]Session, error) {
	var s []Session
	_, e := c.do("GET", "/v1/sessions", "", nil, &s)
	return s, e
}
func (c *APIClient) Get(id string) (Session, error) {
	var s Session
	_, e := c.do("GET", "/v1/sessions/"+id, "", nil, &s)
	return s, e
}
func (c *APIClient) Attach(id string) (string, error) {
	var v LeaseResponse
	_, e := c.do("POST", "/v1/sessions/"+id+"/attach", "", nil, &v)
	return v.Token, e
}
func (c *APIClient) Heartbeat(id, t string) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/heartbeat", t, nil, nil)
	return e
}
func (c *APIClient) Detach(id, t string) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/detach", t, nil, nil)
	return e
}
func (c *APIClient) Submit(id, t, msg string, settings Settings) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/messages", t, MessageRequest{Content: msg, Settings: &settings}, nil)
	return e
}
func (c *APIClient) Retry(id, t string) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/retry", t, nil, nil)
	return e
}
func (c *APIClient) Discard(id, t string) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/discard", t, nil, nil)
	return e
}
func (c *APIClient) Delete(id string) error {
	_, e := c.do("DELETE", "/v1/sessions/"+id, "", nil, nil)
	return e
}

func (c *APIClient) Shutdown() error {
	_, e := c.do("POST", "/v1/shutdown", "", nil, nil)
	return e
}
