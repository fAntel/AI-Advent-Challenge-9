package main

import (
	agent "advent-agent"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type balanceInfo struct {
	Currency string `json:"currency"`
	Total    string `json:"total_balance"`
}
type balanceResponse struct {
	Available bool          `json:"is_available"`
	Infos     []balanceInfo `json:"balance_infos"`
}
type sample struct {
	At        time.Time     `json:"at"`
	Available bool          `json:"available"`
	Balances  []balanceInfo `json:"balances"`
}
type budget struct {
	mu   sync.Mutex
	path string
	api  *agent.APIClient
	http *http.Client
	key  string
}

func (b *budget) samples() ([]sample, error) {
	data, err := os.ReadFile(b.path)
	if errors.Is(err, os.ErrNotExist) {
		return []sample{}, nil
	}
	if err != nil {
		return nil, err
	}
	var out []sample
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}
func (b *budget) capture(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://api.deepseek.com/user/balance", nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+b.key)
	req.Header.Set("Accept", "application/json")
	resp, err := b.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("DeepSeek balance returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var result balanceResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if len(result.Infos) == 0 {
		return "", errors.New("DeepSeek returned no balances")
	}
	for _, v := range result.Infos {
		if v.Currency != "USD" && v.Currency != "CNY" {
			return "", errors.New("unexpected balance currency")
		}
		if _, err := strconv.ParseFloat(v.Total, 64); err != nil {
			return "", err
		}
	}
	b.mu.Lock()
	list, err := b.samples()
	if err == nil {
		list = append(list, sample{At: time.Now().UTC(), Available: result.Available, Balances: result.Infos})
		err = b.save(list)
	}
	b.mu.Unlock()
	if err != nil {
		return "", err
	}
	return b.report()
}
func (b *budget) save(list []sample) error {
	if err := os.MkdirAll(filepath.Dir(b.path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(b.path), ".balance-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	data, _ := json.MarshalIndent(list, "", "  ")
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(append(data, '\n'))
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), b.path)
}
func totals(s sample) map[string]float64 {
	out := map[string]float64{}
	for _, v := range s.Balances {
		n, _ := strconv.ParseFloat(v.Total, 64)
		out[v.Currency] = n
	}
	return out
}
func change(list []sample, start time.Time) string {
	if len(list) < 2 {
		return "unknown (need two samples)"
	}
	first := 0
	for i := range list {
		if !list[i].At.After(start) {
			first = i
		} else {
			break
		}
	}
	last := list[len(list)-1]
	if first == len(list)-1 {
		return "unknown (need two samples)"
	}
	a, z := totals(list[first]), totals(last)
	keys := make([]string, 0, len(z))
	for k := range z {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := []string{}
	for _, k := range keys {
		before, ok := a[k]
		if !ok {
			continue
		}
		delta := before - z[k]
		if delta >= 0 {
			parts = append(parts, fmt.Sprintf("%.2f %s observed decrease", delta, k))
		} else {
			parts = append(parts, fmt.Sprintf("%.2f %s increase/adjustment", -delta, k))
		}
	}
	if len(parts) == 0 {
		return "unknown (currency changed)"
	}
	return strings.Join(parts, ", ") + " since " + list[first].At.Local().Format("15:04")
}
func (b *budget) report() (string, error) {
	b.mu.Lock()
	list, err := b.samples()
	b.mu.Unlock()
	if err != nil {
		return "", err
	}
	if len(list) == 0 {
		return "No balance samples yet.", nil
	}
	latest := list[len(list)-1]
	local := time.Now()
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, local.Location())
	parts := []string{}
	for _, v := range latest.Balances {
		parts = append(parts, v.Total+" "+v.Currency)
	}
	usage, uerr := b.api.UsageReport()
	if uerr != nil {
		return "", uerr
	}
	return fmt.Sprintf("DeepSeek balance: %s (sampled %s, available=%t). Last hour: %s; harness estimate $%.6f USD / %d tokens. Today: %s; harness estimate $%.6f USD / %d tokens.", strings.Join(parts, ", "), latest.At.Local().Format("2006-01-02 15:04"), latest.Available, change(list, local.Add(-time.Hour)), usage.LastHour.CostUSD, usage.LastHour.Tokens, change(list, start), usage.Today.CostUSD, usage.Today.Tokens), nil
}
func toolResult(value string, err error) (*mcp.CallToolResult, any, error) {
	if err != nil {
		return &mcp.CallToolResult{IsError: true, Content: []mcp.Content{&mcp.TextContent{Text: err.Error()}}}, nil, nil
	}
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: value}}}, map[string]string{"report": value}, nil
}
func newServer(b *budget) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "budget-mcp", Version: "0.1.0"}, nil)
	mcp.AddTool(s, &mcp.Tool{Name: "capture_balance", Description: "Sample DeepSeek account balance, store it, and return an hourly and local-day budget report.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: false}}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		v, e := b.capture(ctx)
		return toolResult(v, e)
	})
	mcp.AddTool(s, &mcp.Tool{Name: "get_report", Description: "Return the latest stored DeepSeek balance and aggregate spending report.", Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true}}, func(_ context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		v, e := b.report()
		return toolResult(v, e)
	})
	return s
}
func main() {
	flag.Parse()
	cfg, err := agent.LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	key, err := agent.DeepSeekAPIKey()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	b := &budget{path: filepath.Join(cfg.StateDir, "budget", "snapshots.json"), api: agent.NewAPIClient(cfg.Daemon.SocketPath, 10*time.Second), http: &http.Client{Timeout: 20 * time.Second}, key: key}
	if err := newServer(b).Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
