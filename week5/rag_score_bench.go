package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"time"
)

type ScoreBenchTrial struct {
	Pass        int             `json:"pass"`
	QuestionID  string          `json:"question_id"`
	Duration    time.Duration   `json:"duration"`
	Completed   bool            `json:"completed"`
	Preparation *RAGPreparation `json:"preparation,omitempty"`
	Evidence    *Evidence       `json:"evidence,omitempty"`
	Error       string          `json:"error,omitempty"`
	Budget      time.Duration   `json:"budget,omitempty"`
}
type ScoreBenchRun struct {
	Version        string                   `json:"version"`
	InputSHA256    string                   `json:"input_sha256"`
	Settings       Settings                 `json:"settings"`
	Scorer         ScorerConfig             `json:"scorer"`
	Ollama         OllamaConfig             `json:"ollama"`
	ContextWindow  int                      `json:"context_window"`
	Questions      []string                 `json:"questions"`
	Passes         int                      `json:"passes"`
	Warmup         *ScoreBenchTrial         `json:"warmup,omitempty"`
	Trials         []ScoreBenchTrial        `json:"trials"`
	BaselineSHA256 string                   `json:"baseline_sha256,omitempty"`
	TimeoutFactor  float64                  `json:"timeout_factor,omitempty"`
	Budgets        map[string]time.Duration `json:"budgets,omitempty"`
	WarmupTimeout  time.Duration            `json:"warmup_timeout,omitempty"`
}

type ScoreBenchOptions struct {
	Baseline      string
	TimeoutFactor float64
	WarmupTimeout time.Duration
}

// RunScoreBench replays the first recorded pass without retrieval or label input.
// Checkpointed batches survive interruptions; terminal failures are not retried.
func RunScoreBench(ctx context.Context, provider Provider, cfg Config, settings Settings, input, output, questionIDs string, passes int, options ...ScoreBenchOptions) error {
	cfg = cfg.scoringConfig()
	if settings.Provider != "ollama" {
		return fmt.Errorf("score-bench requires the local Ollama provider")
	}
	if passes < 1 || passes > 10 {
		return fmt.Errorf("runs must be 1..10")
	}
	if err := cfg.RAG.Scorer.Validate(); err != nil {
		return err
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	var source RepeatedRAGRun
	if err := json.Unmarshal(data, &source); err != nil {
		return err
	}
	requested := map[string]bool{}
	if questionIDs != "" {
		for _, id := range strings.Split(questionIDs, ",") {
			if requested[id] || id == "" {
				return fmt.Errorf("invalid question selection")
			}
			requested[id] = true
		}
	}
	fixtures := []RAGTrial{}
	seen := map[string]bool{}
	for _, t := range source.Trials {
		if t.Pass != 1 || (len(requested) > 0 && !requested[t.QuestionID]) {
			continue
		}
		if seen[t.QuestionID] || t.Preparation == nil || !t.Preparation.Retrieved || len(t.Preparation.Candidates) == 0 {
			return fmt.Errorf("invalid frozen candidates for %s", t.QuestionID)
		}
		seen[t.QuestionID] = true
		fixtures = append(fixtures, t)
	}
	if len(fixtures) == 0 || (len(requested) > 0 && len(seen) != len(requested)) {
		return fmt.Errorf("missing frozen questions")
	}
	settings = normalizeRAG(settings)
	settings.RAGRewrite = false
	settings.RAGRerank = true
	if err := validateRAG(settings); err != nil {
		return err
	}
	ids := []string{}
	for _, t := range fixtures {
		ids = append(ids, t.QuestionID)
	}
	run := ScoreBenchRun{Version: "score-bench-v1", InputSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), Settings: settings, Scorer: cfg.RAG.Scorer.Effective(), Ollama: cfg.Ollama, ContextWindow: cfg.Agent.ContextWindowTokens, Questions: ids, Passes: passes, Trials: []ScoreBenchTrial{}}
	if len(options) > 1 {
		return fmt.Errorf("only one benchmark options value is allowed")
	}
	if len(options) == 1 {
		o := options[0]
		if o.WarmupTimeout < 0 {
			return fmt.Errorf("warmup timeout must be nonnegative")
		}
		run.WarmupTimeout = o.WarmupTimeout
		if o.Baseline != "" {
			if math.IsNaN(o.TimeoutFactor) || math.IsInf(o.TimeoutFactor, 0) || o.TimeoutFactor <= 0 || o.TimeoutFactor > 1 {
				return fmt.Errorf("timeout factor must be in (0, 1]")
			}
			b, err := os.ReadFile(o.Baseline)
			if err != nil {
				return err
			}
			var baseline ScoreBenchRun
			if err := json.Unmarshal(b, &baseline); err != nil {
				return err
			}
			if baseline.InputSHA256 != run.InputSHA256 || baseline.Passes != 1 || !reflect.DeepEqual(baseline.Settings, run.Settings) {
				return fmt.Errorf("baseline must use the same frozen input/settings and one pass")
			}
			run.BaselineSHA256 = fmt.Sprintf("%x", sha256.Sum256(b))
			run.TimeoutFactor = o.TimeoutFactor
			run.Budgets = map[string]time.Duration{}
			for _, t := range baseline.Trials {
				if !seen[t.QuestionID] {
					continue
				}
				if !t.Completed || t.Error != "" || t.Duration <= 0 || run.Budgets[t.QuestionID] != 0 {
					return fmt.Errorf("invalid baseline trial: %s", t.QuestionID)
				}
				budget := time.Duration(float64(t.Duration) * o.TimeoutFactor)
				if budget <= 0 {
					return fmt.Errorf("baseline budget too small")
				}
				run.Budgets[t.QuestionID] = budget
			}
			if len(run.Budgets) != len(ids) {
				return fmt.Errorf("baseline is missing selected questions")
			}
		} else if o.TimeoutFactor != 0 {
			return fmt.Errorf("timeout factor requires a baseline")
		}
	}
	if saved, e := os.ReadFile(output); e == nil {
		var old ScoreBenchRun
		if e = json.Unmarshal(saved, &old); e != nil {
			return e
		}
		metadata := old
		metadata.Warmup = nil
		metadata.Trials = []ScoreBenchTrial{}
		if !reflect.DeepEqual(metadata, run) {
			return fmt.Errorf("benchmark inputs changed; choose a new output path")
		}
		run = old
	} else if !os.IsNotExist(e) {
		return e
	}
	save := func() error { return SaveComparison(output, run) }
	execute := func(t *ScoreBenchTrial, fixture RAGTrial) error {
		frozen := &Evidence{Chunks: fixture.Preparation.Candidates, Fingerprint: fixture.Preparation.Fingerprint}
		trialCtx := context.WithValue(ctx, frozenCandidatesKey, frozen)
		if t.Budget > 0 {
			var cancel context.CancelFunc
			trialCtx, cancel = context.WithTimeout(trialCtx, t.Budget-t.Duration)
			defer cancel()
		}
		started := time.Now()
		previousDuration := t.Duration
		p, e := PrepareRAG(trialCtx, provider, cfg.RAG, settings, fixture.Preparation.Query, t.Preparation, func(p *RAGPreparation) error {
			t.Preparation = clonePreparation(p)
			t.Duration = previousDuration + time.Since(started)
			return save()
		})
		t.Duration = previousDuration + time.Since(started)
		if trialCtx.Err() != nil {
			e = trialCtx.Err()
		}
		if e == nil {
			t.Preparation = p
			t.Evidence, e = SelectRAG(p, settings)
		}
		if ctx.Err() != nil {
			return save()
		}
		t.Completed = true
		if e != nil {
			t.Error = e.Error()
		}
		return save()
	}
	if run.Warmup == nil {
		run.Warmup = &ScoreBenchTrial{QuestionID: fixtures[0].QuestionID, Budget: run.WarmupTimeout}
	}
	if !run.Warmup.Completed {
		if err := execute(run.Warmup, fixtures[0]); err != nil {
			return err
		}
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if run.Warmup.Error != "" {
		return fmt.Errorf("warmup failed: %s", run.Warmup.Error)
	}
	positions := map[string]int{}
	for i, t := range run.Trials {
		key := fmt.Sprintf("%d/%s", t.Pass, t.QuestionID)
		if _, ok := positions[key]; ok || t.Pass < 1 || t.Pass > passes || !seen[t.QuestionID] {
			return fmt.Errorf("invalid saved benchmark trial")
		}
		positions[key] = i
	}
	failed := false
	for pass := 1; pass <= passes; pass++ {
		for _, fixture := range fixtures {
			key := fmt.Sprintf("%d/%s", pass, fixture.QuestionID)
			i, ok := positions[key]
			if !ok {
				i = len(run.Trials)
				run.Trials = append(run.Trials, ScoreBenchTrial{Pass: pass, QuestionID: fixture.QuestionID, Budget: run.Budgets[fixture.QuestionID]})
				positions[key] = i
			}
			t := &run.Trials[i]
			if !t.Completed {
				if err := execute(t, fixture); err != nil {
					return err
				}
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if t.Error != "" {
				failed = true
			}
		}
	}
	if failed {
		return fmt.Errorf("one or more scoring trials failed; see %s", output)
	}
	return nil
}
