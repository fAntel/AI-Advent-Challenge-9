package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Pipeline is an ordered, model-free composition of registered MCP tools.
type Pipeline struct {
	Name  string         `toml:"name" json:"name"`
	Steps []PipelineStep `toml:"steps" json:"steps"`
}

type PipelineStep struct {
	Name          string `toml:"name" json:"name"`
	Server        string `toml:"server" json:"server"`
	Tool          string `toml:"tool" json:"tool"`
	ArgumentsJSON string `toml:"arguments_json" json:"arguments_json"`
	BindingsJSON  string `toml:"bindings_json" json:"bindings_json"`
}

type PipelineRun struct {
	Pipeline string   `json:"pipeline"`
	Steps    []string `json:"steps"`
	Result   string   `json:"result"`
}

type pipelineFile struct {
	Pipelines []Pipeline `toml:"pipelines"`
}

type PipelineStore struct {
	mu      sync.Mutex
	path    string
	mcp     *MCPCatalog
	items   map[string]Pipeline
	running map[string]bool
}

func NewPipelineStore(path string, catalog *MCPCatalog) (*PipelineStore, error) {
	p := &PipelineStore{path: path, mcp: catalog, items: map[string]Pipeline{}, running: map[string]bool{}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return p, nil
	}
	if err != nil {
		return nil, err
	}
	var file pipelineFile
	meta, err := toml.Decode(string(data), &file)
	if err != nil {
		return nil, fmt.Errorf("decode pipelines: %w", err)
	}
	if unknown := meta.Undecoded(); len(unknown) != 0 {
		return nil, fmt.Errorf("unknown pipeline key %s", unknown[0])
	}
	for _, item := range file.Pipelines {
		if !mcpName.MatchString(item.Name) || p.items[item.Name].Name != "" {
			return nil, errors.New("invalid or duplicate pipeline name")
		}
		if err := validatePipelineSteps(item.Steps); err != nil {
			return nil, fmt.Errorf("pipeline %s: %w", item.Name, err)
		}
		p.items[item.Name] = item
	}
	return p, nil
}

func (p *PipelineStore) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(p.path), 0700); err != nil {
		return err
	}
	file := pipelineFile{Pipelines: p.listLocked()}
	f, err := os.CreateTemp(filepath.Dir(p.path), ".pipelines-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		err = toml.NewEncoder(f).Encode(file)
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
	return os.Rename(f.Name(), p.path)
}

func (p *PipelineStore) listLocked() []Pipeline {
	out := make([]Pipeline, 0, len(p.items))
	for _, item := range p.items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func (p *PipelineStore) List() []Pipeline {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.listLocked()
}

func (p *PipelineStore) Get(name string) (Pipeline, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, ok := p.items[name]
	if !ok {
		return Pipeline{}, fmt.Errorf("pipeline %q not found", name)
	}
	return item, nil
}

func (p *PipelineStore) Add(name string) error {
	if !mcpName.MatchString(name) {
		return errors.New("invalid pipeline name")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.items[name]; exists {
		return fmt.Errorf("pipeline %q already exists", name)
	}
	p.items[name] = Pipeline{Name: name, Steps: []PipelineStep{}}
	if err := p.saveLocked(); err != nil {
		delete(p.items, name)
		return err
	}
	return nil
}

func (p *PipelineStore) Remove(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, exists := p.items[name]
	if !exists {
		return fmt.Errorf("pipeline %q not found", name)
	}
	if p.running[name] {
		return fmt.Errorf("pipeline %q is running", name)
	}
	delete(p.items, name)
	if err := p.saveLocked(); err != nil {
		p.items[name] = item
		return err
	}
	return nil
}

func parseObject(value string) (map[string]any, error) {
	if value == "" {
		value = "{}"
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(value), &out); err != nil || out == nil {
		return nil, errors.New("arguments must be a JSON object")
	}
	return out, nil
}

func parseBindings(value string) (map[string]string, error) {
	if value == "" {
		value = "{}"
	}
	var out map[string]string
	if err := json.Unmarshal([]byte(value), &out); err != nil || out == nil {
		return nil, errors.New("bindings must be a JSON object of strings")
	}
	return out, nil
}

func validatePipelineSteps(steps []PipelineStep) error {
	seen := map[string]bool{}
	for _, step := range steps {
		if !mcpName.MatchString(step.Name) || !mcpName.MatchString(step.Server) || !mcpName.MatchString(step.Tool) || seen[step.Name] {
			return errors.New("invalid or duplicate step, server, or tool name")
		}
		args, err := parseObject(step.ArgumentsJSON)
		if err != nil {
			return fmt.Errorf("step %s: %w", step.Name, err)
		}
		bindings, err := parseBindings(step.BindingsJSON)
		if err != nil {
			return fmt.Errorf("step %s: %w", step.Name, err)
		}
		for target, source := range bindings {
			if target == "" || !strings.Contains(source, ":") {
				return fmt.Errorf("step %s: invalid binding %q", step.Name, target)
			}
			from, pointer, _ := strings.Cut(source, ":")
			if !seen[from] || (pointer != "$text" && pointer != "" && !strings.HasPrefix(pointer, "/")) {
				return fmt.Errorf("step %s: binding %q must reference an earlier step and a JSON pointer or $text", step.Name, target)
			}
			if _, exists := args[target]; exists {
				return fmt.Errorf("step %s: binding %q conflicts with a literal argument", step.Name, target)
			}
		}
		seen[step.Name] = true
	}
	return nil
}

func (p *PipelineStore) AddStep(ctx context.Context, name string, step PipelineStep) error {
	if step.ArgumentsJSON == "" {
		step.ArgumentsJSON = "{}"
	}
	if step.BindingsJSON == "" {
		step.BindingsJSON = "{}"
	}
	p.mu.Lock()
	item, exists := p.items[name]
	p.mu.Unlock()
	if !exists {
		return fmt.Errorf("pipeline %q not found", name)
	}
	if err := validatePipelineSteps(append(append([]PipelineStep(nil), item.Steps...), step)); err != nil {
		return err
	}
	tools, err := p.mcp.Tools(ctx, step.Server, false)
	if err != nil {
		return err
	}
	found := false
	for _, tool := range tools {
		if tool.Name == step.Tool {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("MCP tool %s/%s not found", step.Server, step.Tool)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	item, exists = p.items[name]
	if !exists || p.running[name] {
		return fmt.Errorf("pipeline %q is missing or running", name)
	}
	item.Steps = append(item.Steps, step)
	if err := validatePipelineSteps(item.Steps); err != nil {
		return err
	}
	p.items[name] = item
	if err := p.saveLocked(); err != nil {
		item.Steps = item.Steps[:len(item.Steps)-1]
		p.items[name] = item
		return err
	}
	return nil
}

func (p *PipelineStore) RemoveStep(name, stepName string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	item, exists := p.items[name]
	if !exists || p.running[name] {
		return fmt.Errorf("pipeline %q is missing or running", name)
	}
	index := -1
	for i, step := range item.Steps {
		if step.Name == stepName {
			index = i
		}
	}
	if index < 0 {
		return fmt.Errorf("step %q not found", stepName)
	}
	previous := item
	item.Steps = append(append([]PipelineStep(nil), item.Steps[:index]...), item.Steps[index+1:]...)
	if err := validatePipelineSteps(item.Steps); err != nil {
		return err
	}
	p.items[name] = item
	if err := p.saveLocked(); err != nil {
		p.items[name] = previous
		return err
	}
	return nil
}

func (p *PipelineStore) Validate(ctx context.Context, name string) error {
	item, err := p.Get(name)
	if err != nil {
		return err
	}
	if len(item.Steps) < 2 {
		return fmt.Errorf("pipeline %q needs at least two steps", name)
	}
	if err := validatePipelineSteps(item.Steps); err != nil {
		return err
	}
	for _, step := range item.Steps {
		tools, err := p.mcp.Tools(ctx, step.Server, false)
		if err != nil {
			return err
		}
		found := false
		for _, tool := range tools {
			if tool.Name == step.Tool {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("MCP tool %s/%s not found", step.Server, step.Tool)
		}
	}
	return nil
}

func selectPipelineValue(root any, pointer string) (any, error) {
	if pointer == "" {
		return root, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, errors.New("invalid JSON pointer")
	}
	for _, raw := range strings.Split(pointer[1:], "/") {
		part := strings.ReplaceAll(strings.ReplaceAll(raw, "~1", "/"), "~0", "~")
		switch value := root.(type) {
		case map[string]any:
			var ok bool
			root, ok = value[part]
			if !ok {
				return nil, fmt.Errorf("field %q not found", part)
			}
		case []any:
			i, err := strconv.Atoi(part)
			if err != nil || i < 0 || i >= len(value) {
				return nil, fmt.Errorf("array index %q not found", part)
			}
			root = value[i]
		default:
			return nil, fmt.Errorf("cannot select field %q", part)
		}
	}
	return root, nil
}

func resultText(result *mcp.CallToolResult) string {
	for _, content := range result.Content {
		if value, ok := content.(*mcp.TextContent); ok {
			return value.Text
		}
	}
	data, _ := json.Marshal(result.StructuredContent)
	return string(data)
}

func (p *PipelineStore) Run(ctx context.Context, name string) (PipelineRun, error) {
	p.mu.Lock()
	item, ok := p.items[name]
	if !ok {
		p.mu.Unlock()
		return PipelineRun{}, fmt.Errorf("pipeline %q not found", name)
	}
	if p.running[name] {
		p.mu.Unlock()
		return PipelineRun{}, fmt.Errorf("pipeline %q is already running", name)
	}
	if len(item.Steps) < 2 {
		p.mu.Unlock()
		return PipelineRun{}, fmt.Errorf("pipeline %q needs at least two steps", name)
	}
	p.running[name] = true
	item.Steps = append([]PipelineStep(nil), item.Steps...)
	p.mu.Unlock()
	defer func() { p.mu.Lock(); delete(p.running, name); p.mu.Unlock() }()

	type output struct {
		structured any
		text       string
	}
	outputs := map[string]output{}
	run := PipelineRun{Pipeline: name, Steps: []string{}}
	for _, step := range item.Steps {
		args, err := parseObject(step.ArgumentsJSON)
		if err != nil {
			return run, fmt.Errorf("step %s: %w", step.Name, err)
		}
		bindings, err := parseBindings(step.BindingsJSON)
		if err != nil {
			return run, fmt.Errorf("step %s: %w", step.Name, err)
		}
		for target, source := range bindings {
			from, pointer, _ := strings.Cut(source, ":")
			prior, exists := outputs[from]
			if !exists {
				return run, fmt.Errorf("step %s: output %q unavailable", step.Name, from)
			}
			if pointer == "$text" {
				args[target] = prior.text
			} else {
				if prior.structured == nil {
					return run, fmt.Errorf("step %s: %s has no structured result", step.Name, from)
				}
				value, err := selectPipelineValue(prior.structured, pointer)
				if err != nil {
					return run, fmt.Errorf("step %s binding %s: %w", step.Name, target, err)
				}
				args[target] = value
			}
		}
		tools, err := p.mcp.Tools(ctx, step.Server, false)
		if err != nil {
			return run, fmt.Errorf("step %s: %w", step.Name, err)
		}
		var tool *mcp.Tool
		for _, candidate := range tools {
			if candidate.Name == step.Tool {
				tool = candidate
				break
			}
		}
		if tool == nil {
			return run, fmt.Errorf("step %s: MCP tool %s/%s not found", step.Name, step.Server, step.Tool)
		}
		if err := validateMCPArguments(tool.InputSchema, args); err != nil {
			return run, fmt.Errorf("step %s: %w", step.Name, err)
		}
		result, err := p.mcp.Call(ctx, step.Server, step.Tool, args)
		if err != nil {
			return run, fmt.Errorf("step %s: %w", step.Name, err)
		}
		if result == nil {
			return run, fmt.Errorf("step %s: MCP tool returned no result", step.Name)
		}
		message := resultText(result)
		if result.IsError {
			return run, fmt.Errorf("step %s: MCP tool: %s", step.Name, boundedString(message))
		}
		var structured any
		if result.StructuredContent != nil {
			data, err := json.Marshal(result.StructuredContent)
			if err != nil {
				return run, fmt.Errorf("step %s: encode structured result: %w", step.Name, err)
			}
			if err := json.Unmarshal(data, &structured); err != nil {
				return run, fmt.Errorf("step %s: decode structured result: %w", step.Name, err)
			}
		}
		outputs[step.Name] = output{structured: structured, text: message}
		run.Steps = append(run.Steps, step.Name)
		run.Result = message
	}
	return run, nil
}
