package llm

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// anthropicMaxRetries is the SDK's own retry budget for one call (it backs
// off on 429/5xx/529 before the error reaches the caller).
const anthropicMaxRetries = 2

// Anthropic is the Client backed by the Anthropic Messages API.
type Anthropic struct {
	client anthropic.Client
	model  string
}

// NewAnthropic builds a Client for the exact model id (e.g.
// "claude-haiku-5-5"). opts are applied after the defaults, so tests can
// override the base URL or the retry budget. The API key is never logged.
func NewAnthropic(apiKey, model string, opts ...option.RequestOption) *Anthropic {
	all := append([]option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithMaxRetries(anthropicMaxRetries),
	}, opts...)
	return &Anthropic{client: anthropic.NewClient(all...), model: model}
}

// Structured sends one user message (no assistant prefill) with the system
// prompt as a cached block, low effort and a json_schema output format via
// output_config (structured outputs; not the deprecated output_format and
// not a forced tool_choice).
func (a *Anthropic) Structured(ctx context.Context, req StructuredRequest) (StructuredResponse, error) {
	maxTokens := req.MaxTokens
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: int64(maxTokens),
		System: []anthropic.TextBlockParam{{
			Text:         req.System,
			CacheControl: anthropic.NewCacheControlEphemeralParam(),
		}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(req.User)),
		},
		OutputConfig: anthropic.OutputConfigParam{
			Effort: anthropic.OutputConfigEffortLow,
			Format: anthropic.JSONOutputFormatParam{Schema: req.Schema},
		},
	}

	msg, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return StructuredResponse{}, classifyAnthropicError(err)
	}

	out := StructuredResponse{
		StopReason:      string(msg.StopReason),
		Model:           string(msg.Model),
		InputTokens:     int(msg.Usage.InputTokens),
		OutputTokens:    int(msg.Usage.OutputTokens),
		CacheReadTokens: int(msg.Usage.CacheReadInputTokens),
	}
	if out.StopReason == StopReasonRefusal {
		out.StopDetails = strings.TrimSpace(strings.Join(nonEmpty(string(msg.StopDetails.Category), msg.StopDetails.Explanation), ": "))
	}
	for _, block := range msg.Content {
		if block.Type == "text" {
			out.JSON = []byte(block.Text)
			break
		}
	}
	return out, nil
}

// classifyAnthropicError wraps transient failures in ErrTransient and
// returns any other API error (4xx other than 429) as-is.
func classifyAnthropicError(err error) error {
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		switch code := apiErr.StatusCode; {
		case code == http.StatusTooManyRequests, code >= 500:
			// 5xx covers 500/502/503/504 and Anthropic's 529 "overloaded".
			return fmt.Errorf("%w: anthropic status %d: %v", ErrTransient, code, err)
		default:
			return fmt.Errorf("llm: anthropic status %d: %w", code, err)
		}
	}
	// No HTTP response: network failure, context cancellation or deadline.
	return fmt.Errorf("%w: %v", ErrTransient, err)
}

func nonEmpty(values ...string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			out = append(out, v)
		}
	}
	return out
}
