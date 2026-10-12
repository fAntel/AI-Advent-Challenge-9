package agent

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const ComparisonInstructions = "Answer the Kotlin question accurately and concisely, with a small example when useful. State version caveats and uncertainty. If reference data is supplied, cite supported claims using its numbered references."

type ComparisonQuestion struct {
	ID    string `json:"id"`
	Query string `json:"query"`
}
type ComparisonSuite struct {
	Questions []ComparisonQuestion `json:"questions"`
}
type ComparisonAnswer struct {
	Result        *RAGResult  `json:"result,omitempty"`
	RawAnswer     string      `json:"raw_answer,omitempty"`
	AnswerSkipped bool        `json:"answer_skipped,omitempty"`
	QuestionID    string      `json:"question_id"`
	Mode          string      `json:"mode"`
	Messages      []Message   `json:"messages"`
	Settings      Settings    `json:"settings"`
	Evidence      *Evidence   `json:"evidence,omitempty"`
	Answer        string      `json:"answer"`
	Call          CallMetrics `json:"call"`
	ReviewStatus  string      `json:"review_status"`
}
type ComparisonFailure struct {
	Record ComparisonAnswer `json:"record"`
	Error  string           `json:"error"`
}

type ComparisonRun struct {
	FailedAttempts []ComparisonFailure `json:"failed_attempts,omitempty"`
	Version        string              `json:"version"`
	SuiteSHA256    string              `json:"suite_sha256"`
	IndexSHA256    string              `json:"index_sha256"`
	Settings       Settings            `json:"settings"`
	StartedAt      time.Time           `json:"started_at"`
	Answers        []ComparisonAnswer  `json:"answers"`
}

func SaveComparison(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".comparison-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func ComparisonMessages(query string, e *Evidence) []Message {
	return WithEvidence([]Message{{Role: "system", Content: ComparisonInstructions}, {Role: "user", Content: query}}, e)
}
func RunComparison(ctx context.Context, p Provider, cfg RAGConfig, s Settings, suitePath, output string) error {
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
		if q.ID == "" || q.Query == "" || seen[q.ID] {
			return fmt.Errorf("invalid or duplicate suite question")
		}
		seen[q.ID] = true
	}
	if len(seen) == 0 {
		return fmt.Errorf("empty comparison suite")
	}
	s = normalizeRAG(s)
	s.RAG = false
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
	run := ComparisonRun{Version: RAGGroundingVersion, SuiteSHA256: fmt.Sprintf("%x", sha256.Sum256(data)), IndexSHA256: fp, Settings: s, StartedAt: time.Now().UTC(), Answers: []ComparisonAnswer{}}
	if saved, err := os.ReadFile(output); err == nil {
		var old ComparisonRun
		if err = json.Unmarshal(saved, &old); err != nil {
			return err
		}
		a, _ := json.Marshal(old.Settings)
		b, _ := json.Marshal(s)
		if old.Version != run.Version || old.SuiteSHA256 != run.SuiteSHA256 || old.IndexSHA256 != fp || string(a) != string(b) {
			return fmt.Errorf("comparison inputs changed; choose a new output path")
		}
		run = old
	} else if !os.IsNotExist(err) {
		return err
	}
	done := map[string]bool{}
	for _, a := range run.Answers {
		key := a.QuestionID + "/" + a.Mode
		if done[key] || a.Answer == "" || !seen[a.QuestionID] || (a.Mode != "plain" && a.Mode != "rag") {
			return fmt.Errorf("invalid saved answer %s", key)
		}
		done[key] = true
	}
	for i, q := range suite.Questions {
		modes := []string{"plain", "rag"}
		if i%2 == 1 {
			modes = []string{"rag", "plain"}
		}
		for _, mode := range modes {
			key := q.ID + "/" + mode
			if done[key] {
				continue
			}
			settings := s
			settings.RAG = mode == "rag"
			var evidence *Evidence
			if settings.RAG {
				evidence, err = Retrieve(ctx, cfg, settings, q.Query, p)
				if err != nil {
					return err
				}
				if evidence.Fingerprint != fp {
					return fmt.Errorf("comparison index changed")
				}
			}
			messages := ComparisonMessages(q.Query, evidence)
			record, err := completeComparisonAnswer(ctx, p, q, mode, settings, evidence, messages)
			if err != nil {
				run.FailedAttempts = append(run.FailedAttempts, ComparisonFailure{record, err.Error()})
				if saveErr := SaveComparison(output, run); saveErr != nil {
					return saveErr
				}
				return fmt.Errorf("%s: %w", key, err)
			}
			run.Answers = append(run.Answers, record)
			if err = SaveComparison(output, run); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Saved %s (%d/%d)\n", key, len(run.Answers), 2*len(suite.Questions))
		}
	}
	return nil
}

// Both comparison commands use exactly the same grounding validator as the daemon.
func completeComparisonAnswer(ctx context.Context, provider Provider, q ComparisonQuestion, mode string, settings Settings, evidence *Evidence, messages []Message) (ComparisonAnswer, error) {
	if settings.Provider == "ollama" {
		messages = withLocalCapabilities(messages)
	}
	record := ComparisonAnswer{QuestionID: q.ID, Mode: mode, Messages: messages, Settings: settings, Evidence: evidence, ReviewStatus: "unreviewed"}
	if evidence != nil && len(evidence.Chunks) == 0 {
		record.Result = unknownRAG()
		record.Answer = record.Result.Render()
		record.AnswerSkipped = true
		return record, nil
	}
	ctx = context.WithValue(ctx, ragEvidenceKey, evidence)
	response, err := provider.Complete(ctx, CompletionRequest{Messages: messages, Settings: settings})
	record.Call = response.Call
	record.RawAnswer = response.Answer
	if err != nil {
		return record, err
	}
	if response.Call.FinishReason == "length" {
		return record, fmt.Errorf("answer reached output token limit")
	}
	if strings.TrimSpace(response.Answer) == "" {
		return record, fmt.Errorf("empty answer")
	}
	if evidence != nil {
		record.Result, err = ValidateRAGAnswer(response.Answer, evidence)
		if err != nil {
			return record, err
		}
		record.Answer = record.Result.Render()
	} else {
		record.Answer = response.Answer
	}
	return record, nil
}
