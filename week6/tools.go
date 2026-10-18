package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
)

type ToolRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result,omitempty"`
	Error     string `json:"error,omitempty"`
}

func toolDefinitions() []map[string]any {
	result := []map[string]any{}
	add := func(name, description string, properties map[string]any, required ...string) {
		if required == nil {
			required = []string{}
		}
		result = append(result, map[string]any{"type": "function", "function": map[string]any{"name": name, "description": description, "parameters": map[string]any{"type": "object", "properties": properties, "required": required, "additionalProperties": false}}})
	}
	str := map[string]string{"type": "string"}
	integer := map[string]string{"type": "integer"}
	boolean := map[string]string{"type": "boolean"}
	add("list_files", "Discover project files. Optional glob matches project-relative paths (Go filepath.Match); offset paginates. Hidden Git metadata and runtime directories are excluded.", map[string]any{"glob": str, "offset": integer})
	add("search_files", "Search literal text in project text files; results include paths and line numbers. Optional glob and offset.", map[string]any{"query": str, "glob": str, "offset": integer}, "query")
	add("read_file", "Read UTF-8 text (at most 256 KiB), returning source lines and full-file hash. start_line defaults to 1. count defaults to 200 (maximum 500).", map[string]any{"path": str, "start_line": integer, "count": integer}, "path")
	add("write_file", "Save complete UTF-8 file content directly. Read before editing and pass expected_hash. For a new file set create=true; parent directory must exist. Returns a saved receipt; actual diffs are displayed by the harness.", map[string]any{"path": str, "content": str, "expected_hash": str, "create": boolean}, "path", "content")
	add("rag_search", "Search the project's Markdown index. Missing/stale indexes are errors; use live file tools instead. Does not rebuild the index.", map[string]any{"query": str}, "query")
	add("git_branch", "Read live branch and HEAD through the configured Git MCP server.", map[string]any{})
	return result
}
func appendEvidence(op *Operation, ev Evidence) Evidence {
	ev.Number = len(op.Evidence) + 1
	ev.ID = fmt.Sprintf("e%d", ev.Number)
	op.Evidence = append(op.Evidence, ev)
	return ev
}
func toolJSON(value any) (string, error) { b, err := json.Marshal(value); return string(b), err }
func (e *Engine) executeTool(ctx context.Context, fs *ProjectFS, s *Session, op *Operation, tc ToolCall, checkpoint func() error) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var object map[string]json.RawMessage
	if err := json.Unmarshal([]byte(tc.Function.Arguments), &object); err != nil || object == nil {
		return "", fmt.Errorf("tool arguments must be a JSON object")
	}
	evidence := func(source, text string, start, end int) (string, error) {
		ev := appendEvidence(op, Evidence{Source: source, Text: text, Start: start, End: end})
		return toolJSON(ev)
	}
	switch tc.Function.Name {
	case "list_files", "search_files":
		var args struct {
			Query  string `json:"query"`
			Glob   string `json:"glob"`
			Offset int    `json:"offset"`
		}
		if err := decodeStrict(tc.Function.Arguments, &args); err != nil {
			return "", err
		}
		if args.Offset < 0 {
			return "", fmt.Errorf("offset must be nonnegative")
		}
		if tc.Function.Name == "search_files" && args.Query == "" {
			return "", fmt.Errorf("query is required")
		}
		if args.Glob != "" {
			if _, err := filepath.Match(args.Glob, ""); err != nil {
				return "", err
			}
		}
		paths, err := fs.Files(ctx)
		if err != nil {
			return "", err
		}
		rows := []string{}
		skipped := 0
		next := 0
		size := 0
		more := false
		add := func(row string) bool {
			index := next
			next++
			if index < args.Offset {
				return true
			}
			if len(rows) >= 100 || size+len(row) > maxToolBytes {
				more = true
				next--
				return false
			}
			rows = append(rows, row)
			size += len(row)
			return true
		}
	outer:
		for _, path := range paths {
			if err := ctx.Err(); err != nil {
				return "", err
			}
			if args.Glob != "" {
				match, _ := filepath.Match(args.Glob, path)
				if !match {
					continue
				}
			}
			if tc.Function.Name == "list_files" {
				if !add(path) {
					break
				}
				continue
			}
			content, _, err := fs.Read(path)
			if err != nil {
				skipped++
				continue
			}
			for i, line := range strings.Split(content, "\n") {
				if strings.Contains(line, args.Query) {
					if len(line) > 4096 {
						line = line[:4096] + " [line truncated]"
					}
					if !add(fmt.Sprintf("%s:%d:%s", path, i+1, line)) {
						break outer
					}
				}
			}
		}
		// Evidence is source text, not a JSON document embedded in a string.
		// Otherwise quoting a decoded search row loses an escaping layer and
		// fails exact-quote validation for tabs, quotes, and backslashes.
		ev := appendEvidence(op, Evidence{Source: "tool://" + tc.Function.Name, Text: strings.Join(rows, "\n")})
		return toolJSON(map[string]any{"evidence": ev, "next_offset": next, "has_more": more, "skipped_unreadable_or_nontext_files": skipped})
	case "read_file":
		var args struct {
			Path  string `json:"path"`
			Start int    `json:"start_line"`
			Count int    `json:"count"`
		}
		if err := decodeStrict(tc.Function.Arguments, &args); err != nil {
			return "", err
		}
		if args.Start == 0 {
			args.Start = 1
		}
		if args.Count == 0 {
			args.Count = 200
		}
		if args.Start < 1 || args.Count < 1 || args.Count > 500 {
			return "", fmt.Errorf("invalid line range")
		}
		content, path, err := fs.Read(args.Path)
		if err != nil {
			return "", err
		}
		lines := strings.SplitAfter(content, "\n")
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		if args.Start > len(lines)+1 {
			return "", fmt.Errorf("start_line exceeds file length")
		}
		end := args.Start - 1
		size := 0
		for end < len(lines) && end < args.Start-1+args.Count && size+len(lines[end]) <= maxToolBytes {
			size += len(lines[end])
			end++
		}
		if end == args.Start-1 && end < len(lines) {
			return "", fmt.Errorf("single line exceeds output limit")
		}
		text := strings.Join(lines[args.Start-1:end], "")
		ev := appendEvidence(op, Evidence{Source: path, Text: text, Start: args.Start, End: end})
		return toolJSON(map[string]any{"evidence": ev, "hash": hashText(content), "total_lines": len(lines), "has_more": end < len(lines), "next_line": end + 1})
	case "write_file":
		var args struct {
			Path     string  `json:"path"`
			Content  *string `json:"content"`
			Expected string  `json:"expected_hash"`
			Create   bool    `json:"create"`
		}
		if err := decodeStrict(tc.Function.Arguments, &args); err != nil {
			return "", err
		}
		if args.Content == nil {
			return "", fmt.Errorf("content is required")
		}
		if args.Create && args.Expected != "" {
			return "", fmt.Errorf("create cannot have expected_hash")
		}
		pos := -1
		change, err := fs.Write(ctx, args.Path, *args.Content, args.Expected, args.Create, func(c Change) error { pos = len(op.Changes); op.Changes = append(op.Changes, c); return checkpoint() })
		if pos >= 0 {
			if err != nil {
				change.Status = "failed"
			}
			op.Changes[pos] = change
			if saveErr := checkpoint(); saveErr != nil {
				return "", saveErr
			}
		}
		if err != nil {
			return "", err
		}
		return evidence(change.Path, fmt.Sprintf("Saved %s; before_hash=%s; after_hash=%s", change.Path, change.BeforeHash, change.AfterHash), 0, 0)
	case "rag_search":
		var args struct {
			Query string `json:"query"`
		}
		if err := decodeStrict(tc.Function.Arguments, &args); err != nil {
			return "", err
		}
		if strings.TrimSpace(args.Query) == "" {
			return "", fmt.Errorf("query is required")
		}
		raw, err := e.Index(ctx, e.Config, s.Root, "search", args.Query)
		if err != nil {
			return "", err
		}
		var candidates []Evidence
		if err = json.Unmarshal(raw, &candidates); err != nil {
			return "", err
		}
		// An index is untrusted storage: verify every selected source against live files.
		for _, ev := range candidates {
			text, _, err := fs.Read(ev.Source)
			if err != nil {
				return "", fmt.Errorf("invalid index source: %w", err)
			}
			lines := strings.SplitAfter(text, "\n")
			if ev.Start < 1 || ev.End < ev.Start || ev.End > len(lines) || strings.Join(lines[ev.Start-1:ev.End], "") != ev.Text {
				return "", fmt.Errorf("stale index source; reindex or use file tools")
			}
		}
		selected, err := SelectEvidence(ctx, candidates, args.Query, e.Complete, &op.Calls)
		if err != nil {
			return "", err
		}
		for i, ev := range selected {
			selected[i] = appendEvidence(op, ev)
		}
		return toolJSON(selected)
	case "git_branch":
		if err := decodeStrict(tc.Function.Arguments, &struct{}{}); err != nil {
			return "", err
		}
		snapshot, err := e.Fetch(ctx, e.Config, s.Root)
		if err != nil {
			op.Warning = "Live Git context unavailable: " + err.Error()
			return "", err
		}
		op.Git = &snapshot
		body, _ := toolJSON(snapshot)
		return evidence("mcp://git/git_branch", body, 0, 0)
	default:
		return "", fmt.Errorf("unknown tool %q", tc.Function.Name)
	}
}
