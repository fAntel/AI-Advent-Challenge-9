package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type config struct {
	URL    string `json:"url"`
	APIKey string `json:"apiKey"`
	CAFile string `json:"caFile,omitempty"`
}

type homebox struct {
	base *url.URL
	key  string
	http *http.Client
}

var entityID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

func loadConfig(filename string) (config, error) {
	info, err := os.Stat(filename)
	if err != nil {
		return config{}, err
	}
	if info.Mode().Perm()&0077 != 0 {
		return config{}, errors.New("config must be private (chmod 600)")
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		return config{}, err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("decode config: %w", err)
	}
	return cfg, nil
}

func newHomebox(cfg config) (*homebox, error) {
	u, err := url.Parse(cfg.URL)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("url must be an HTTPS HomeBox base URL without credentials, query, or fragment")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("apiKey is required")
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.CAFile != "" {
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read caFile: %w", err)
		}
		roots, err := x509.SystemCertPool()
		if err != nil {
			return nil, err
		}
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("caFile contains no certificates")
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: roots}
	}
	return &homebox{base: u, key: cfg.APIKey, http: &http.Client{Transport: transport, Timeout: 20 * time.Second}}, nil
}

func (h *homebox) request(ctx context.Context, method, endpoint string, query url.Values, body any) (any, error) {
	u := *h.base
	u.Path = path.Join(strings.TrimSuffix(h.base.Path, "/"), "/api/v1", endpoint)
	u.RawQuery = query.Encode()
	var input io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		input = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), input)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+h.key)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HomeBox %s %s: %w", method, endpoint, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		message := strings.TrimSpace(string(data))
		if len(message) > 500 {
			message = message[:500]
		}
		return nil, fmt.Errorf("HomeBox %s %s returned HTTP %d: %s", method, endpoint, resp.StatusCode, message)
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("HomeBox %s %s returned invalid JSON: %w", method, endpoint, err)
	}
	return result, nil
}

func toolResult(value any, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil, nil
	}
	data, _ := json.Marshal(value)
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, value, nil
}

type searchArgs struct {
	Query     string   `json:"query,omitempty" jsonschema:"Search text; omit to list all entities"`
	Page      int      `json:"page,omitempty" jsonschema:"Page number"`
	PageSize  int      `json:"pageSize,omitempty" jsonschema:"Items per page"`
	Tags      []string `json:"tags,omitempty" jsonschema:"Tag IDs to filter by"`
	ParentIDs []string `json:"parentIds,omitempty" jsonschema:"Parent entity IDs to filter by"`
}
type idArgs struct {
	ID string `json:"id" jsonschema:"required,Entity ID"`
}
type createArgs struct {
	Name         string   `json:"name" jsonschema:"required,Entity name"`
	Description  string   `json:"description,omitempty"`
	EntityTypeID string   `json:"entityTypeId,omitempty"`
	ParentID     *string  `json:"parentId,omitempty"`
	Quantity     *float64 `json:"quantity,omitempty"`
	TagIDs       []string `json:"tagIds,omitempty"`
}
type updateArgs struct {
	ID           string   `json:"id" jsonschema:"required,Entity ID"`
	EntityTypeID *string  `json:"entityTypeId,omitempty"`
	ParentID     *string  `json:"parentId,omitempty"`
	Quantity     *float64 `json:"quantity,omitempty"`
	TagIDs       []string `json:"tagIds,omitempty"`
}

func newServer(h *homebox) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "homebox-mcp", Version: "0.1.0"}, nil)
	read := &mcp.ToolAnnotations{ReadOnlyHint: true}
	write := &mcp.ToolAnnotations{ReadOnlyHint: false}
	mcp.AddTool(s, &mcp.Tool{Name: "search_entities", Description: "Search or list HomeBox entities with pagination and optional tag or parent filters.", Annotations: read}, func(ctx context.Context, _ *mcp.CallToolRequest, a searchArgs) (*mcp.CallToolResult, any, error) {
		q := url.Values{}
		if a.Query != "" {
			q.Set("q", a.Query)
		}
		if a.Page > 0 {
			q.Set("page", fmt.Sprint(a.Page))
		}
		if a.PageSize > 0 {
			q.Set("pageSize", fmt.Sprint(a.PageSize))
		}
		for _, v := range a.Tags {
			q.Add("tags", v)
		}
		for _, v := range a.ParentIDs {
			q.Add("parentIds", v)
		}
		v, e := h.request(ctx, "GET", "/entities", q, nil)
		return toolResult(v, e)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "get_entity", Description: "Get a HomeBox entity by its ID, including details and metadata.", Annotations: read}, func(ctx context.Context, _ *mcp.CallToolRequest, a idArgs) (*mcp.CallToolResult, any, error) {
		if !entityID.MatchString(a.ID) {
			return toolResult(nil, errors.New("id must contain only letters, digits, _ or -"))
		}
		v, e := h.request(ctx, "GET", "/entities/"+a.ID, nil, nil)
		return toolResult(v, e)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "list_entity_types", Description: "List available HomeBox entity types and their IDs.", Annotations: read}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		v, e := h.request(ctx, "GET", "/entity-types", nil, nil)
		return toolResult(v, e)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "list_tags", Description: "List HomeBox tags and their IDs.", Annotations: read}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		v, e := h.request(ctx, "GET", "/tags", nil, nil)
		return toolResult(v, e)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "create_entity", Description: "Create a HomeBox entity. Name is required; IDs can be obtained from list_entity_types and list_tags.", Annotations: write}, func(ctx context.Context, _ *mcp.CallToolRequest, a createArgs) (*mcp.CallToolResult, any, error) {
		if strings.TrimSpace(a.Name) == "" {
			return toolResult(nil, errors.New("name is required"))
		}
		v, e := h.request(ctx, "POST", "/entities", nil, a)
		return toolResult(v, e)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "update_entity", Description: "Patch a HomeBox entity's type, parent, quantity, or tags. Only supplied fields change; use an empty tagIds array to clear tags.", Annotations: write}, func(ctx context.Context, _ *mcp.CallToolRequest, a updateArgs) (*mcp.CallToolResult, any, error) {
		if !entityID.MatchString(a.ID) {
			return toolResult(nil, errors.New("id must contain only letters, digits, _ or -"))
		}
		if a.EntityTypeID == nil && a.ParentID == nil && a.Quantity == nil && a.TagIDs == nil {
			return toolResult(nil, errors.New("at least one field to update is required"))
		}
		patch := map[string]any{"id": a.ID}
		if a.EntityTypeID != nil {
			patch["entityTypeId"] = *a.EntityTypeID
		}
		if a.ParentID != nil {
			patch["parentId"] = *a.ParentID
		}
		if a.Quantity != nil {
			patch["quantity"] = *a.Quantity
		}
		if a.TagIDs != nil {
			patch["tagIds"] = a.TagIDs
		}
		v, e := h.request(ctx, "PATCH", "/entities/"+a.ID, nil, patch)
		return toolResult(v, e)
	})
	return s
}

func main() {
	configPath := flag.String("config", "", "private JSON config file (mode 0600)")
	flag.Parse()
	if *configPath == "" {
		fmt.Fprintln(os.Stderr, "--config is required")
		os.Exit(2)
	}
	cfg, err := loadConfig(*configPath)
	if err == nil {
		var h *homebox
		h, err = newHomebox(cfg)
		if err == nil {
			err = newServer(h).Run(context.Background(), &mcp.StdioTransport{})
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
