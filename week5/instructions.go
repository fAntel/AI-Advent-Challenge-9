package agent

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveInstructions snapshots broad-to-specific instruction files. The
// returned content is persisted in the session, so later file edits cannot
// change an existing session's behavior.
func ResolveInstructions(cfg Config, projectPath, sessionPrompt, sessionPromptPath string) ([]InstructionSource, error) {
	paths := []string{filepath.Join(cfg.ConfigDir, "AGENTS.md"), filepath.Join(cfg.ConfigDir, "profiles", cfg.Profile, "AGENTS.md")}
	if projectPath != "" {
		paths = append(paths, filepath.Join(projectPath, "AGENTS.md"))
	}
	result := []InstructionSource{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read instructions %q: %w", path, err)
		}
		if strings.TrimSpace(string(data)) != "" {
			result = append(result, InstructionSource{Path: path, Content: string(data)})
		}
	}
	if strings.TrimSpace(sessionPrompt) != "" {
		path := sessionPromptPath
		if path == "" {
			path = "<cli:--system-prompt>"
		}
		result = append(result, InstructionSource{Path: path, Content: sessionPrompt})
	}
	return result, nil
}
