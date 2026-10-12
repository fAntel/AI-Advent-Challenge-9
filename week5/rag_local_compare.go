package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// Repeated trials do not share preparations: each is an independent pipeline run.
type RAGTrial struct {
	Pass        int               `json:"pass"`
	QuestionID  string            `json:"question_id"`
	StartedAt   time.Time         `json:"started_at"`
	Duration    time.Duration     `json:"duration"`
	Completed   bool              `json:"completed"`
	Preparation *RAGPreparation   `json:"preparation,omitempty"`
	Answer      *ComparisonAnswer `json:"answer,omitempty"`
	Error       string            `json:"error,omitempty"`
}
type RepeatedRAGRun struct {
	Version       string       `json:"version"`
	SuiteSHA256   string       `json:"suite_sha256"`
	IndexSHA256   string       `json:"index_sha256"`
	Settings      Settings     `json:"settings"`
	Ollama        OllamaConfig `json:"ollama"`
	ContextWindow int          `json:"context_window"`
	Passes        int          `json:"passes"`
	Trials        []RAGTrial   `json:"trials"`
}

func RunRepeatedRAG(ctx context.Context, provider Provider, cfg Config, settings Settings, suitePath, output string, passes int) error {
	if passes < 1 || passes > 10 {
		return fmt.Errorf("runs must be 1..10")
	}
	data, err := os.ReadFile(suitePath)
	if err != nil {
		return err
	}
	var suite ComparisonSuite
	if err = json.Unmarshal(data, &suite); err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, q := range suite.Questions {
		if q.ID == "" || strings.TrimSpace(q.Query) == "" || seen[q.ID] {
			return fmt.Errorf("invalid or duplicate suite question")
		}
		seen[q.ID] = true
	}
	if len(seen) == 0 {
		return fmt.Errorf("empty suite")
	}
	s := normalizeRAG(settings)
	s.RAG = true
	s.Approach = "none"
	s.ContextStrategy = "full"
	s.Compression = false
	s.Roles = nil
	if err = ValidateSettings(s); err != nil {
		return err
	}
	_, db, err := cfg.RAG.Paths()
	if err != nil {
		return err
	}
	fp, err := IndexFingerprint(db)
	if err != nil {
		return err
	}
	run := RepeatedRAGRun{Version: "local-grounded-v2", SuiteSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), IndexSHA256: fp, Settings: s, Ollama: cfg.Ollama, ContextWindow: cfg.Agent.ContextWindowTokens, Passes: passes, Trials: []RAGTrial{}}
	if saved, e := os.ReadFile(output); e == nil {
		var old RepeatedRAGRun
		if err = json.Unmarshal(saved, &old); err != nil {
			return err
		}
		metadata := old
		metadata.Trials = []RAGTrial{}
		a, _ := json.Marshal(metadata)
		b, _ := json.Marshal(run)
		if string(a) != string(b) {
			return fmt.Errorf("comparison inputs changed; choose a new output path")
		}
		run = old
	} else if !os.IsNotExist(e) {
		return e
	}
	positions := map[string]int{}
	for i, t := range run.Trials {
		key := fmt.Sprintf("%d/%s", t.Pass, t.QuestionID)
		if _, ok := positions[key]; ok || !seen[t.QuestionID] || t.Pass < 1 || t.Pass > passes {
			return fmt.Errorf("invalid saved trial")
		}
		if t.Completed && t.Error == "" && (t.Answer == nil || t.Answer.Result == nil) {
			return fmt.Errorf("invalid completed trial")
		}
		positions[key] = i
	}
	save := func() error { return SaveComparison(output, run) }
	for pass := 1; pass <= passes; pass++ {
		for _, q := range suite.Questions {
			if err = ctx.Err(); err != nil {
				return err
			}
			current, e := IndexFingerprint(db)
			if e != nil || current != fp {
				return fmt.Errorf("comparison index changed")
			}
			key := fmt.Sprintf("%d/%s", pass, q.ID)
			i, ok := positions[key]
			if ok && run.Trials[i].Completed {
				continue
			}
			if !ok {
				i = len(run.Trials)
				positions[key] = i
				run.Trials = append(run.Trials, RAGTrial{Pass: pass, QuestionID: q.ID, StartedAt: time.Now().UTC()})
			}
			trial := &run.Trials[i]
			started := time.Now()
			if err = save(); err != nil {
				return err
			}
			trialCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Daemon.RequestTimeout))
			prep, e := PrepareRAG(trialCtx, provider, cfg.RAG, s, q.Query, trial.Preparation, func(p *RAGPreparation) error {
				if p.Retrieved && p.Fingerprint != fp {
					return fmt.Errorf("comparison index changed")
				}
				trial.Preparation = clonePreparation(p)
				return save()
			})
			if e == nil {
				var evidence *Evidence
				evidence, e = SelectRAG(prep, s)
				if e == nil {
					record, answerErr := completeComparisonAnswer(trialCtx, provider, q, "grounded", s, evidence, ComparisonMessages(q.Query, evidence))
					trial.Answer = &record
					e = answerErr
				}
			}
			cancel()
			trial.Duration += time.Since(started)
			// Cancellation leaves an unfinished checkpoint; failures are terminal measured results.
			if ctx.Err() != nil {
				if err = save(); err != nil {
					return err
				}
				return ctx.Err()
			}
			trial.Completed = true
			if e != nil {
				trial.Error = e.Error()
			}
			if err = save(); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Saved trial %s: %s\n", key, func() string {
				if trial.Error != "" {
					return trial.Error
				}
				return trial.Answer.Result.Status
			}())
		}
	}
	failures := 0
	for _, t := range run.Trials {
		if t.Error != "" {
			failures++
		}
	}
	if failures > 0 {
		return fmt.Errorf("%d/%d trials failed; all results saved (rerun will not retry failures)", failures, len(run.Trials))
	}
	return nil
}
