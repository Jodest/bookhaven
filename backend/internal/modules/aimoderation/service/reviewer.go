package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// minCallInterval is the minimum pause between two model calls.
const minCallInterval = 500 * time.Millisecond

// maxOutputTokens caps one answer (JSON plus low-effort thinking).
const maxOutputTokens = 1024

// BookSource is the subset of *adminservice.Service the reviewer and the
// task handlers depend on.
type BookSource interface {
	BookReviewInputs(ctx context.Context, ids []uuid.UUID) ([]adminservice.BookReviewTarget, error)
	SaveBookAIVerdict(ctx context.Context, v adminservice.BookAIVerdictRecord) error
	ListAIReviewCandidates(ctx context.Context, max, promptVersion int) ([]uuid.UUID, error)
}

// ErrModelCall marks a failed model call: nothing was stored for the book
// and the run should stop (errors.Is(err, llm.ErrTransient) tells whether
// a retry later may help).
var ErrModelCall = errors.New("aimoderation: model call failed")

// ReviewResult tells what happened to one book.
type ReviewResult int

const (
	// ReviewStored: a verdict was computed and stored.
	ReviewStored ReviewResult = iota
	// ReviewSkipped: the book is unknown or no longer pending.
	ReviewSkipped
)

// Reviewer computes and stores verdicts one book at a time, with at least
// minCallInterval between model calls.
type Reviewer struct {
	client llm.Client
	books  BookSource
	model  string
	log    *zap.Logger

	now      func() time.Time
	sleep    func(ctx context.Context, d time.Duration) error
	lastCall time.Time
}

// NewReviewer builds a Reviewer for the configured model id. A nil logger
// is replaced by a no-op logger.
func NewReviewer(client llm.Client, books BookSource, model string, log *zap.Logger) *Reviewer {
	if log == nil {
		log = zap.NewNop()
	}
	return &Reviewer{client: client, books: books, model: model, log: log, now: time.Now, sleep: sleepCtx}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// ReviewBook (re)computes the verdict of one book, whatever its current
// verdict. The book is loaded right before the call, so a book published
// or rejected meanwhile is skipped. A failed model call stores nothing and
// returns an error wrapping ErrModelCall (and llm.ErrTransient when
// transient). A refusal, a truncated or an invalid answer is stored as a
// needs_human verdict (ValidateResponse).
func (r *Reviewer) ReviewBook(ctx context.Context, id uuid.UUID) (ReviewResult, error) {
	targets, err := r.books.BookReviewInputs(ctx, []uuid.UUID{id})
	if err != nil {
		return ReviewSkipped, err
	}
	if len(targets) == 0 || targets[0].Status != catalogDomain.BookStatusPending {
		return ReviewSkipped, nil
	}
	target := targets[0]

	user, err := UserMessage(target.Input)
	if err != nil {
		return ReviewSkipped, err
	}
	if err := r.pace(ctx); err != nil {
		return ReviewSkipped, fmt.Errorf("%w: %w: %v", ErrModelCall, llm.ErrTransient, err)
	}
	r.lastCall = r.now()
	resp, err := r.client.Structured(ctx, llm.StructuredRequest{
		System:    SystemPrompt,
		User:      user,
		Schema:    OutputSchema(),
		MaxTokens: maxOutputTokens,
	})
	if err != nil {
		return ReviewSkipped, fmt.Errorf("%w: %w", ErrModelCall, err)
	}

	out := ValidateResponse(resp)
	model := resp.Model
	if model == "" {
		model = r.model
	}
	if err := r.books.SaveBookAIVerdict(ctx, adminservice.BookAIVerdictRecord{
		BookID:        target.BookID,
		Verdict:       out.Verdict,
		ReasonCodes:   out.ReasonCodes,
		Reason:        out.Reason,
		Confidence:    out.Confidence,
		Model:         model,
		PromptVersion: PromptVersion,
		InputHash:     target.InputHash,
		InputTokens:   resp.InputTokens,
		OutputTokens:  resp.OutputTokens,
	}); err != nil {
		return ReviewSkipped, err
	}
	// Never log the prompt or the book text: id, verdict, tokens, stop.
	r.log.Info("AI verdict stored",
		zap.String("book_id", target.BookID.String()),
		zap.String("verdict", out.Verdict),
		zap.Strings("reason_codes", out.ReasonCodes),
		zap.String("stop_reason", resp.StopReason),
		zap.Int("input_tokens", resp.InputTokens),
		zap.Int("output_tokens", resp.OutputTokens),
		zap.Int("cache_read_tokens", resp.CacheReadTokens))
	return ReviewStored, nil
}

// pace waits until minCallInterval has passed since the previous call.
func (r *Reviewer) pace(ctx context.Context) error {
	if r.lastCall.IsZero() {
		return nil
	}
	if wait := minCallInterval - r.now().Sub(r.lastCall); wait > 0 {
		return r.sleep(ctx, wait)
	}
	return nil
}
