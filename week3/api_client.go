package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"time"
)

func (c *APIClient) MCPAdd(name, description string, command []string) error {
	_, e := c.do("POST", "/v1/mcp", "", map[string]any{"name": name, "description": description, "command": command}, nil)
	return e
}
func (c *APIClient) MCPList() ([]MCPServer, error) {
	var out []MCPServer
	_, e := c.do("GET", "/v1/mcp", "", nil, &out)
	return out, e
}
func (c *APIClient) MCPTools(name string, refresh bool) ([]*mcp.Tool, error) {
	var out []*mcp.Tool
	path := "/v1/mcp/" + url.PathEscape(name) + "/tools"
	if refresh {
		path += "?refresh=true"
	}
	_, e := c.do("GET", path, "", nil, &out)
	return out, e
}
func (c *APIClient) MCPRemove(name string) error {
	_, e := c.do("DELETE", "/v1/mcp/"+url.PathEscape(name), "", nil, nil)
	return e
}

var ErrDaemonUnavailable = errors.New("advent-agentd is unavailable")

type APIClient struct {
	HTTP    *http.Client
	BaseURL string
	Profile string
}

func NewAPIClient(socket string, timeout time.Duration) *APIClient {
	tr := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "unix", socket)
	}}
	return &APIClient{HTTP: &http.Client{Transport: tr, Timeout: timeout}, BaseURL: "http://unix", Profile: DefaultProfile}
}
func (c *APIClient) WithProfile(profile string) *APIClient { c.Profile = profile; return c }
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
	if c.Profile != "" {
		req.Header.Set("X-Agent-Profile", c.Profile)
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
func (c *APIClient) ApproveTool(id, t string, approve bool) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/tool-approval", t, map[string]bool{"approve": approve}, nil)
	return e
}
func (c *APIClient) Discard(id, t string) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/discard", t, nil, nil)
	return e
}
func (c *APIClient) ContinueTask(id, t string) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/continue", t, nil, nil)
	return e
}
func (c *APIClient) BackTask(id, t, phase string) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/back", t, TaskBackRequest{Phase: phase}, nil)
	return e
}
func (c *APIClient) CreateCheckpoint(id, t, name string) (Checkpoint, error) {
	var checkpoint Checkpoint
	_, e := c.do("POST", "/v1/sessions/"+id+"/checkpoints", t, CheckpointRequest{Name: name}, &checkpoint)
	return checkpoint, e
}
func (c *APIClient) CreateBranch(id, t string, req BranchRequest) (Session, error) {
	var session Session
	_, e := c.do("POST", "/v1/sessions/"+id+"/branch", t, req, &session)
	return session, e
}
func (c *APIClient) Branches(id string) ([]Session, error) {
	var sessions []Session
	_, e := c.do("GET", "/v1/sessions/"+id+"/branches", "", nil, &sessions)
	return sessions, e
}
func (c *APIClient) Delete(id string) error {
	_, e := c.do("DELETE", "/v1/sessions/"+id, "", nil, nil)
	return e
}
func (c *APIClient) Memory(id string) (MemoryView, error) {
	var view MemoryView
	_, e := c.do("GET", "/v1/sessions/"+id+"/memory", "", nil, &view)
	return view, e
}
func (c *APIClient) MutateMemory(id, token string, req MemoryMutationRequest) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/memory", token, req, nil)
	return e
}
func (c *APIClient) Invariants(id string) (InvariantView, error) {
	var view InvariantView
	_, e := c.do("GET", "/v1/sessions/"+id+"/invariants", "", nil, &view)
	return view, e
}
func (c *APIClient) MutateInvariant(id, token string, req InvariantMutationRequest) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/invariants", token, req, nil)
	return e
}
func (c *APIClient) Clear(id, token string) error {
	_, e := c.do("POST", "/v1/sessions/"+id+"/clear", token, nil, nil)
	return e
}

func (c *APIClient) Shutdown() error {
	_, e := c.do("POST", "/v1/shutdown", "", nil, nil)
	return e
}
