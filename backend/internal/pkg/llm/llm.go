// Package llm is a small, provider-neutral client for structured (JSON
// schema constrained) calls to a large language model. The only
// implementation is Anthropic (NewAnthropic); callers depend on Client so
// tests can use a fake and never reach the network.
package llm

import (
	"context"
	"errors"
)

// StructuredRequest is one single-turn call whose answer must follow
// Schema.
type StructuredRequest struct {
	// System holds the stable instructions. It is sent first, marked for
	// prompt caching.
	System string
	// User holds the per-call data (the only user message; no assistant
	// prefill is ever sent).
	User string
	// Schema is the JSON schema the answer must follow.
	Schema map[string]any
	// MaxTokens caps the answer; <= 0 uses DefaultMaxTokens.
	MaxTokens int
}

// StructuredResponse is the raw outcome of a call. The caller validates
// JSON itself: StopReason must be checked first (a refusal or a truncated
// answer carries no usable JSON).
type StructuredResponse struct {
	// JSON is the text of the first text block ("" if none).
	JSON []byte
	// StopReason is the provider's stop reason: end_turn, max_tokens,
	// refusal, ...
	StopReason string
	// StopDetails is the refusal explanation when present.
	StopDetails string
	// Model is the model id reported by the provider.
	Model                                      string
	InputTokens, OutputTokens, CacheReadTokens int
}

// Stop reasons the callers act on.
const (
	StopReasonEndTurn   = "end_turn"
	StopReasonMaxTokens = "max_tokens"
	StopReasonRefusal   = "refusal"
)

// DefaultMaxTokens is used when StructuredRequest.MaxTokens is not set.
const DefaultMaxTokens = 1024

// Client makes structured calls.
type Client interface {
	Structured(ctx context.Context, req StructuredRequest) (StructuredResponse, error)
}

// ErrTransient wraps failures worth retrying later: rate limiting (429),
// server errors (5xx), overload (529), network errors and context
// cancellation/deadline. Any other error (e.g. a 400 caused by a bad
// request) is returned unwrapped.
var ErrTransient = errors.New("llm: transient failure")
