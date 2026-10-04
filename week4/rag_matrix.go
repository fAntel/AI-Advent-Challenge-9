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

type RAGMode struct {
	Name    string `json:"name"`
	Rewrite bool   `json:"rewrite"`
	Rerank  bool   `json:"rerank"`
	Filter  bool   `json:"filter"`
}

func EnhancedModes() []RAGMode {
	modes := []RAGMode{}
	for _, rewrite := range []bool{false, true} {
		for _, base := range []RAGMode{{Name: "rag"}, {Name: "rag-rerank", Rerank: true}, {Name: "rag-filter", Filter: true}, {Name: "rag-rerank-filter", Rerank: true, Filter: true}} {
			base.Rewrite = rewrite
			if rewrite {
				base.Name += "-rewrite"
			}
			modes = append(modes, base)
		}
	}
	return modes
}

type EnhancedComparisonRun struct {
	Version      string                     `json:"version"`
	SuiteSHA256  string                     `json:"suite_sha256"`
	IndexSHA256  string                     `json:"index_sha256"`
	Settings     Settings                   `json:"settings"`
	Modes        []RAGMode                  `json:"modes"`
	StartedAt    time.Time                  `json:"started_at"`
	Preparations map[string]*RAGPreparation `json:"preparations"`
	Answers      []ComparisonAnswer         `json:"answers"`
}

func RunEnhancedComparison(ctx context.Context, provider Provider, cfg RAGConfig, settings Settings, suitePath, output string) error {
	data, err := os.ReadFile(suitePath)
	if err != nil {
		return err
	}
	var suite ComparisonSuite
	if err = json.Unmarshal(data, &suite); err != nil {
		return err
	}
	questions := map[string]bool{}
	for _, q := range suite.Questions {
		if q.ID == "" || strings.TrimSpace(q.Query) == "" || questions[q.ID] {
			return fmt.Errorf("invalid or duplicate suite question")
		}
		questions[q.ID] = true
	}
	if len(questions) == 0 {
		return fmt.Errorf("empty comparison suite")
	}
	s := normalizeRAG(settings)
	s.RAG = true
	s.RAGRewrite = false
	s.RAGRerank = false
	s.RAGFilter = false
	s.Temperature = nil
	s.Approach = "none"
	s.ContextStrategy = "full"
	s.Compression = false
	s.Roles = nil
	if err = ValidateSettings(s); err != nil {
		return err
	}
	_, db, err := cfg.Paths()
	if err != nil {
		return err
	}
	fp, err := IndexFingerprint(db)
	if err != nil {
		return err
	}
	run := EnhancedComparisonRun{Version: RAGPreparationVersion, SuiteSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), IndexSHA256: fp, Settings: s, Modes: EnhancedModes(), StartedAt: time.Now().UTC(), Preparations: map[string]*RAGPreparation{}, Answers: []ComparisonAnswer{}}
	if saved, e := os.ReadFile(output); e == nil {
		var old EnhancedComparisonRun
		if err = json.Unmarshal(saved, &old); err != nil {
			return err
		}
		oldSettings, _ := json.Marshal(old.Settings)
		newSettings, _ := json.Marshal(s)
		oldModes, _ := json.Marshal(old.Modes)
		newModes, _ := json.Marshal(run.Modes)
		if old.Version != run.Version || old.SuiteSHA256 != run.SuiteSHA256 || old.IndexSHA256 != fp || string(oldSettings) != string(newSettings) || string(oldModes) != string(newModes) {
			return fmt.Errorf("comparison inputs changed; choose a new output path")
		}
		if old.Preparations == nil {
			return fmt.Errorf("invalid saved preparations")
		}
		run = old
	} else if !os.IsNotExist(e) {
		return e
	}
	modeSet := map[string]bool{}
	for _, m := range run.Modes {
		modeSet[m.Name] = true
	}
	done := map[string]bool{}
	for _, a := range run.Answers {
		key := a.QuestionID + "/" + a.Mode
		if done[key] || !questions[a.QuestionID] || !modeSet[a.Mode] || strings.TrimSpace(a.Answer) == "" || a.Evidence == nil || a.Evidence.Fingerprint != fp {
			return fmt.Errorf("invalid saved answer %s", key)
		}
		done[key] = true
	}
	save := func() error { return SaveComparison(output, run) }
	for i, q := range suite.Questions {
		for offset := 0; offset < len(run.Modes); offset++ {
			mode := run.Modes[(i+offset)%len(run.Modes)]
			key := q.ID + "/" + mode.Name
			if done[key] {
				continue
			}
			current, err := IndexFingerprint(db)
			if err != nil || current != fp {
				return fmt.Errorf("comparison index changed")
			}
			modeSettings := s
			modeSettings.RAGRewrite = mode.Rewrite
			modeSettings.RAGRerank = mode.Rerank
			modeSettings.RAGFilter = mode.Filter
			preparationKey := fmt.Sprintf("%s/rewrite=%t", q.ID, mode.Rewrite)
			prep, err := PrepareRAG(ctx, provider, cfg, modeSettings, q.Query, run.Preparations[preparationKey], func(p *RAGPreparation) error {
				if p.Retrieved && p.Fingerprint != fp {
					return fmt.Errorf("comparison index changed")
				}
				run.Preparations[preparationKey] = clonePreparation(p)
				return save()
			})
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			evidence, err := SelectRAG(prep, modeSettings)
			if err != nil {
				return err
			}
			// Baseline modes must not inherit scoring calls made for another mode.
			if !mode.Rerank && !mode.Filter {
				evidence.Preparation.Scores = map[string]int{}
				calls := []RAGCall{}
				for _, c := range evidence.Preparation.Calls {
					if c.Purpose != "rag_score" {
						calls = append(calls, c)
					}
				}
				evidence.Preparation.Calls = calls
			}
			messages := ComparisonMessages(q.Query, evidence)
			result, err := provider.Complete(ctx, CompletionRequest{Messages: messages, Settings: modeSettings})
			if err != nil {
				return fmt.Errorf("%s: %w", key, err)
			}
			if strings.TrimSpace(result.Answer) == "" {
				return fmt.Errorf("%s: empty answer", key)
			}
			run.Answers = append(run.Answers, ComparisonAnswer{QuestionID: q.ID, Mode: mode.Name, Messages: messages, Settings: modeSettings, Evidence: evidence, Answer: result.Answer, Call: result.Call, ReviewStatus: "unreviewed"})
			if err = save(); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Saved %s (%d/%d)\n", key, len(run.Answers), len(run.Modes)*len(suite.Questions))
		}
	}
	return nil
}
