package agent

import (
	"context"
	"errors"
)

// CompletionRequest and CompletionResponse are provider-neutral core values.
// Provider adapters own wire payloads, credentials, validation, and pricing.
type CompletionRequest struct {
	Messages []Message
	Settings Settings
}
type CompletionResponse struct {
	Answer      string
	Usage       Usage
	Metrics     Metrics
	Call        CallMetrics
	Diagnostics string
}
type Completion = CompletionResponse

type Provider interface {
	Complete(context.Context, CompletionRequest) (CompletionResponse, error)
}
type Completer interface {
	Complete(context.Context, []Message, Settings) (CompletionResponse, error)
} // Week 1 compatibility

var ErrContextWindowExceeded = errors.New("model context window exceeded")

type contextKey string

const sessionContextKey contextKey = "session"
const operationContextKey contextKey = "operation"
