package agent

import (
	"context"
	"fmt"
)

func (a *Agent) validateProvider(s Settings) error {
	// Empty provider identifiers from old DeepSeek sessions remain compatible.
	provider := s.Provider
	if provider == "" {
		provider = "deepseek"
	}
	expected := a.defaults.Provider
	if expected == "" {
		expected = "deepseek"
	}
	if provider != expected {
		return fmt.Errorf("session provider %q does not match daemon provider %q; select the matching profile", provider, expected)
	}
	return nil
}

func localBatchCount(messages []Message, desired int) int {
	count := 0
	for count+1 < len(messages) && count+2 <= desired {
		if messages[count].Role != "user" || messages[count+1].Role != "assistant" {
			break
		}
		count += 2
	}
	return count
}

func (a *Agent) fitLocalHistory(ctx context.Context, id, opID string, settings Settings, memory MemoryView, invariants map[string]string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for {
		r := a.sessions[id]
		if r == nil || r.session.Operation == nil || r.session.Operation.ID != opID {
			return ErrConflict
		}
		messages := contextRequestMessagesWithInvariantFeedback(r.session, invariants, memory, settings, false, false)
		messages = WithEvidence(messages, r.session.Operation.Evidence)
		err := checkLocalBudget(withLocalCapabilities(messages), a.contextWindowTokens, a.local.OutputTokens)
		if err == nil {
			return nil
		}
		if !settings.Compression || len(r.session.Messages) < 5 {
			return err
		}
		before := r.session.SummarizedMessages
		a.compressHistoryMode(ctx, id, opID, settings, r, true)
		r = a.sessions[id]
		if r == nil {
			return ErrConflict
		}
		if r.session.SummarizedMessages == before {
			return fmt.Errorf("%w; local compression could not reduce history: %s", err, r.session.Operation.Warning)
		}
		if err := a.store.Save(r.session); err != nil {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}
