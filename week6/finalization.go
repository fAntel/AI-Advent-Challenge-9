package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

type AnswerAttempt struct {
	Raw          string   `json:"raw"`
	ValidatedRaw string   `json:"validated_raw,omitempty"`
	Error        string   `json:"error,omitempty"`
	Issues       []string `json:"issues,omitempty"`
}

func needsFinalization(op Operation) bool {
	if op.FinalizationPending {
		return true
	}
	// Recognize previously saved validation failures without upgrading unrelated
	// tool/provider failures to answer-only retries.
	if op.Raw != "" && op.Error != "" {
		_, err := ValidateAnswer(op.Raw, op.Evidence)
		return err != nil && err.Error() == op.Error
	}
	return false
}

// This phase cannot call tools. A rejected answer is corrected against the same
// evidence, so validation recovery never repeats a successful write.
func (e *Engine) finalizeAnswer(ctx context.Context, op *Operation, draft string, checkpoint func() error) (Answer, error) {
	op.Draft = draft
	op.FinalizationPending = true
	if err := checkpoint(); err != nil {
		return Answer{}, err
	}
	payload, _ := json.Marshal(map[string]any{"question": op.Question, "draft": draft, "evidence": op.Evidence, "warning": op.Warning})
	messages := []Message{{"system", finalInstructions}, {"user", string(payload)}}
	var validationErr error
	for attempt := 0; attempt < 3; attempt++ {
		if err := ctx.Err(); err != nil {
			return Answer{}, err
		}
		encoded, _ := json.Marshal(messages)
		if len(encoded) > 240000 {
			return Answer{}, fmt.Errorf("final answer context exceeds 240000 bytes; saved changes remain; /retry retries only the final answer")
		}
		purpose := "answer"
		if attempt > 0 {
			purpose = "answer_repair"
		}
		raw, call, err := e.Complete(ctx, purpose, messages)
		op.Calls = append(op.Calls, call)
		op.Raw = raw
		record := AnswerAttempt{Raw: raw}
		if err != nil {
			record.Error = err.Error()
			op.AnswerAttempts = append(op.AnswerAttempts, record)
			if saveErr := checkpoint(); saveErr != nil {
				return Answer{}, saveErr
			}
			return Answer{}, err
		}
		validated := restoreMarkdownQuoteWrapping(raw, op.Evidence)
		if validated != raw {
			record.ValidatedRaw = validated
			op.Raw = validated
		}
		answer, err := ValidateAnswer(validated, op.Evidence)
		if err == nil && strings.Contains(answer.Answer, "```") {
			for _, ev := range op.Evidence {
				if ev.Source == "chart://weights" {
					err = &AnswerValidationError{Issues: []string{"Do not reproduce the chart or include code fences; the harness appends the exact chart automatically."}}
					break
				}
			}
		}
		validationErr = err
		if err != nil {
			record.Error = err.Error()
			record.Issues = validationIssues(err)
		}
		op.AnswerAttempts = append(op.AnswerAttempts, record)
		if err := checkpoint(); err != nil {
			return Answer{}, err
		}
		if validationErr == nil {
			op.FinalizationPending = false
			return answer, nil
		}
		// Replace the failed draft, rather than accumulating responses and consuming
		// the context budget. The original evidence is unchanged in messages[1].
		messages = messages[:2]
		feedback, _ := json.Marshal(map[string]any{"validation_error": validationErr.Error(), "validation_errors": validationIssues(validationErr), "instruction": "Correct the final JSON answer using the original evidence. Audit EVERY citation and quote, not only the first reported error. Each [n] requires at least one exact supporting quote from its corresponding evidence ID. Remove unsupported claims rather than fabricate quotes. Return the complete corrected JSON answer."})
		messages = append(messages, Message{"assistant", raw}, Message{"user", string(feedback)})
	}
	return Answer{}, fmt.Errorf("final answer validation failed after 3 attempts: %w; saved changes remain; /retry retries only the final answer", validationErr)
}

// Recover only Markdown line wrapping, then run the unchanged exact-substring
// validator. Preserve the original provider answer in AnswerAttempt.Raw.
func restoreMarkdownQuoteWrapping(raw string, evidence []Evidence) string {
	var answer Answer
	if decodeStrict(raw, &answer) != nil {
		return raw
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal([]byte(raw), &fields) != nil {
		return raw
	}
	for _, key := range []string{"status", "answer", "quotes", "clarification"} {
		if v, ok := fields[key]; !ok || string(v) == "null" {
			return raw
		}
	}
	changed := false
	for i, q := range answer.Quotes {
		for _, e := range evidence {
			if e.ID != q.ID || !strings.HasSuffix(strings.ToLower(e.Source), ".md") || strings.Contains(e.Text, q.Text) {
				continue
			}
			words := strings.Fields(q.Text)
			if len(words) < 2 {
				continue
			}
			for j := range words {
				words[j] = regexp.QuoteMeta(words[j])
			}
			match := regexp.MustCompile(strings.Join(words, `[\t\r\n ]+`)).FindString(e.Text)
			if match != "" {
				answer.Quotes[i].Text = match
				changed = true
			}
		}
	}
	if !changed {
		return raw
	}
	b, err := json.Marshal(answer)
	if err != nil {
		return raw
	}
	return string(b)
}

func validationIssues(err error) []string {
	var validation *AnswerValidationError
	if errors.As(err, &validation) {
		return validation.Issues
	}
	return []string{err.Error()}
}
