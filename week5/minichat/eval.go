package minichat

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Dialog struct {
	Name      string   `json:"name"`
	Questions []string `json:"questions"`
}
type Suite struct {
	Dialogs []Dialog `json:"dialogs"`
}
type Evaluation struct {
	Version     int        `json:"version"`
	SuiteSHA256 string     `json:"suite_sha256"`
	Settings    Settings   `json:"settings"`
	Sessions    []*Session `json:"sessions"`
}

func Evaluate(ctx context.Context, e *Engine, settings Settings, suitePath, output string, progress io.Writer) error {
	data, err := os.ReadFile(suitePath)
	if err != nil {
		return err
	}
	var suite Suite
	if err = json.Unmarshal(data, &suite); err != nil {
		return err
	}
	if len(suite.Dialogs) != 2 {
		return errors.New("evaluation requires two dialogs")
	}
	for _, d := range suite.Dialogs {
		if len(d.Questions) < 10 || len(d.Questions) > 15 {
			return errors.New("each dialog must contain 10–15 questions")
		}
	}
	sum := sha256.Sum256(data)
	run := Evaluation{Version: 1, SuiteSHA256: hex.EncodeToString(sum[:]), Settings: settings, Sessions: []*Session{}}
	if old, readErr := os.ReadFile(output); readErr == nil {
		if err = json.Unmarshal(old, &run); err != nil {
			return err
		}
		if run.Version != 1 || run.SuiteSHA256 != hex.EncodeToString(sum[:]) || jsonText(run.Settings) != jsonText(settings) {
			return errors.New("evaluation inputs changed; choose a new output")
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	for i, d := range suite.Dialogs {
		if len(run.Sessions) <= i {
			s, err := e.Store.Create(settings)
			if err != nil {
				return err
			}
			run.Sessions = append(run.Sessions, s)
			if err = saveEvaluation(output, run); err != nil {
				return err
			}
		}
		id := run.Sessions[i].ID
		for j, q := range d.Questions {
			s, err := e.Store.Load(id)
			if err != nil {
				return err
			}
			if len(s.Turns) > j {
				t := s.Turns[j]
				if t.Question != q || t.State == "discarded" {
					return errors.New("evaluation session diverged from suite")
				}
				if t.State == "completed" {
					run.Sessions[i] = s
					continue
				}
				s, err = e.Retry(ctx, id)
			} else {
				s, err = e.Ask(ctx, id, q)
			}
			if s != nil {
				run.Sessions[i] = s
			}
			if saveErr := saveEvaluation(output, run); saveErr != nil {
				return saveErr
			}
			if err != nil {
				return fmt.Errorf("%s turn %d: %w (rerun eval to resume)", d.Name, j+1, err)
			}
			fmt.Fprintf(progress, "%s %d/%d: %s\n", d.Name, j+1, len(d.Questions), s.Turns[j].State)
		}
	}
	return saveEvaluation(output, run)
}
func saveEvaluation(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".mini-chat-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
