package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// Task types and queue. The queue is served by its own asynq.Server with
// Concurrency 1, so all model calls are serialized.
const (
	TaskTypeGenerate = "ai_descriptions:generate"
	TaskTypeSweep    = "ai_descriptions:sweep"
	QueueName        = "ai_descriptions"
)

// minCallInterval is the minimum pause between two model calls.
const minCallInterval = 500 * time.Millisecond

// GeneratePayload is the payload of TaskTypeGenerate. QueuedAt lets a
// retry skip books already generated since the task was queued.
type GeneratePayload struct {
	BookIDs  []string  `json:"book_ids"`
	QueuedAt time.Time `json:"queued_at"`
}

// SweepPayload is the payload of TaskTypeSweep.
type SweepPayload struct {
	Max int `json:"max"`
}

// NewGenerateTask builds the generation of the given books.
func NewGenerateTask(ids []uuid.UUID, queuedAt time.Time) (*asynq.Task, error) {
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		strs = append(strs, id.String())
	}
	payload, err := json.Marshal(GeneratePayload{BookIDs: strs, QueuedAt: queuedAt.UTC()})
	if err != nil {
		return nil, fmt.Errorf("aidescriptions: marshal generate payload: %w", err)
	}
	return asynq.NewTask(TaskTypeGenerate, payload), nil
}

// GenerateOptions: two retries (a retry skips books already generated
// after queued_at, so it never pays twice) and a 15 min timeout.
func GenerateOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(2),
		asynq.Timeout(15 * time.Minute),
	}
}

// NewSweepTask builds an all_missing run over at most max books.
func NewSweepTask(max int) (*asynq.Task, error) {
	payload, err := json.Marshal(SweepPayload{Max: max})
	if err != nil {
		return nil, fmt.Errorf("aidescriptions: marshal sweep payload: %w", err)
	}
	return asynq.NewTask(TaskTypeSweep, payload), nil
}

// SweepOptions: no retries (the admin queues the next run), a 1 h timeout
// and Unique(2h): a second run while one is queued is rejected
// (asynq.ErrDuplicateTask → 409).
func SweepOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(time.Hour),
		asynq.Unique(2 * time.Hour),
	}
}

// errModelCall marks a failed model call: nothing was stored for the book
// (errors.Is(err, llm.ErrTransient) tells whether a retry may help).
var errModelCall = errors.New("aidescriptions: model call failed")

// attemptFunc reports whether this is the task's final attempt.
type attemptFunc func(ctx context.Context) bool

// asynqFinalAttempt is true when asynq will not retry the task again. A
// context without asynq metadata counts as final.
func asynqFinalAttempt(ctx context.Context) bool {
	retried, ok1 := asynq.GetRetryCount(ctx)
	maxRetry, ok2 := asynq.GetMaxRetry(ctx)
	if !ok1 || !ok2 {
		return true
	}
	return retried >= maxRetry
}

// Handlers runs the AI description tasks in the worker, one book at a
// time with at least minCallInterval between model calls.
type Handlers struct {
	repo    Repository
	books   BookLoader
	client  llm.Client
	modelID string
	log     *zap.Logger

	now      func() time.Time
	sleep    func(ctx context.Context, d time.Duration) error
	final    attemptFunc
	lastCall time.Time
}

// HandlerOption configures Handlers.
type HandlerOption func(*Handlers)

// WithHandlerClock overrides the clock and the pause (tests).
func WithHandlerClock(now func() time.Time, sleep func(ctx context.Context, d time.Duration) error) HandlerOption {
	return func(h *Handlers) {
		h.now = now
		h.sleep = sleep
	}
}

// WithFinalAttempt overrides the final-attempt check (tests).
func WithFinalAttempt(final func(ctx context.Context) bool) HandlerOption {
	return func(h *Handlers) { h.final = final }
}

// NewHandlers builds the task handlers. modelID is the configured model
// (stored when the provider reports none). A nil logger is replaced by a
// no-op logger.
func NewHandlers(repo Repository, books BookLoader, client llm.Client, modelID string, log *zap.Logger, opts ...HandlerOption) *Handlers {
	if log == nil {
		log = zap.NewNop()
	}
	h := &Handlers{
		repo: repo, books: books, client: client, modelID: modelID, log: log,
		now: time.Now, sleep: sleepCtx, final: asynqFinalAttempt,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
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

func (h *Handlers) clock() time.Time { return h.now().UTC() }

// HandleGenerate (re)generates every listed book that is still eligible,
// whatever its current AI description, skipping books whose record was
// generated after queued_at (done by an earlier attempt). A transient
// model failure on a non-final attempt is returned (asynq retries); on the
// final attempt, or for any other model failure, it is logged and retries
// are skipped. Nothing is stored for the failed book.
func (h *Handlers) HandleGenerate(ctx context.Context, t *asynq.Task) error {
	var p GeneratePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("aidescriptions: decode generate payload: %v: %w", err, asynq.SkipRetry)
	}
	queuedAt := p.QueuedAt
	for _, raw := range p.BookIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		if _, err := h.generateOne(ctx, id, &queuedAt); err != nil {
			if !errors.Is(err, errModelCall) {
				return fmt.Errorf("aidescriptions generate: book %s: %w", id, err)
			}
			transient := errors.Is(err, llm.ErrTransient)
			if transient && !h.final(ctx) {
				h.log.Warn("AI description: transient model failure, will retry",
					zap.String("book_id", id.String()), zap.Error(err))
				return err
			}
			h.logModelFailure("AI description generation stopped", id, err)
			return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
		}
	}
	return nil
}

// HandleSweep generates up to payload.max published books with a blank
// own description and no AI description record, oldest first. It stops at
// the first failed model call and returns nil after logging it (nothing is
// stored for that book); a database failure is returned.
func (h *Handlers) HandleSweep(ctx context.Context, t *asynq.Task) error {
	var p SweepPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("aidescriptions: decode sweep payload: %v: %w", err, asynq.SkipRetry)
	}
	max := p.Max
	if max <= 0 {
		return nil
	}
	if max > domain.MaxBulk {
		max = domain.MaxBulk
	}
	ids, err := h.repo.ListMissingCandidateIDs(ctx, max)
	if err != nil {
		return fmt.Errorf("aidescriptions sweep: %w", err)
	}
	stored, skipped := 0, 0
	for _, id := range ids {
		ok, err := h.generateOne(ctx, id, nil)
		if err != nil {
			if errors.Is(err, errModelCall) {
				h.logModelFailure("AI description sweep stopped", id, err)
				break
			}
			return fmt.Errorf("aidescriptions sweep: book %s: %w", id, err)
		}
		if ok {
			stored++
		} else {
			skipped++
		}
	}
	h.log.Info("AI description sweep finished",
		zap.Int("candidates", len(ids)), zap.Int("stored", stored), zap.Int("skipped", skipped))
	return nil
}

// eligible: own description blank and status pending or published.
func eligible(b catalogDomain.Book) bool {
	if b.Description != nil && strings.TrimSpace(*b.Description) != "" {
		return false
	}
	return b.Status == catalogDomain.BookStatusPending || b.Status == catalogDomain.BookStatusPublished
}

// generateOne loads the book right before the call and generates its AI
// description. stored is false when the book was skipped (unknown,
// ineligible, or generated after *queuedAt). A failed model call stores
// nothing and returns an error wrapping errModelCall.
func (h *Handlers) generateOne(ctx context.Context, id uuid.UUID, queuedAt *time.Time) (stored bool, err error) {
	books, err := h.books.LoadBooksWithRelations(ctx, []uuid.UUID{id})
	if err != nil {
		return false, err
	}
	var book *catalogDomain.Book
	for i := range books {
		if books[i].ID == id {
			book = &books[i]
			break
		}
	}
	if book == nil || !eligible(*book) {
		return false, nil
	}
	if queuedAt != nil && !queuedAt.IsZero() {
		cur, err := h.repo.Get(ctx, id)
		if err != nil {
			return false, err
		}
		if cur != nil && cur.GeneratedAt.After(*queuedAt) {
			return false, nil
		}
	}

	in := InputOf(*book)
	user, err := UserMessage(in)
	if err != nil {
		return false, err
	}
	if err := h.pace(ctx); err != nil {
		return false, fmt.Errorf("%w: %w: %v", errModelCall, llm.ErrTransient, err)
	}
	h.lastCall = h.now()
	resp, err := h.client.Structured(ctx, llm.StructuredRequest{
		System:    SystemPrompt,
		User:      user,
		Schema:    OutputSchema(),
		MaxTokens: generateMaxTokens,
	})
	if err != nil {
		return false, fmt.Errorf("%w: %w", errModelCall, err)
	}

	out := ValidateResponse(resp)
	model := resp.Model
	if model == "" {
		model = h.modelID
	}
	var code *string
	if out.ErrorCode != "" {
		c := out.ErrorCode
		code = &c
	}
	if err := h.repo.Upsert(ctx, domain.Description{
		BookID:        id,
		Status:        out.Status,
		Text:          out.Text,
		ErrorCode:     code,
		Model:         model,
		PromptVersion: PromptVersion,
		InputHash:     InputHash(in),
		InputTokens:   resp.InputTokens,
		OutputTokens:  resp.OutputTokens,
		GeneratedAt:   h.clock(),
	}); err != nil {
		return false, err
	}
	// Never log the prompt or the text: id, status, tokens, stop reason.
	h.log.Info("AI description stored",
		zap.String("book_id", id.String()),
		zap.String("status", out.Status),
		zap.String("error_code", out.ErrorCode),
		zap.String("stop_reason", resp.StopReason),
		zap.Int("input_tokens", resp.InputTokens),
		zap.Int("output_tokens", resp.OutputTokens))
	return true, nil
}

// pace waits until minCallInterval has passed since the previous call.
func (h *Handlers) pace(ctx context.Context) error {
	if h.lastCall.IsZero() {
		return nil
	}
	if wait := minCallInterval - h.now().Sub(h.lastCall); wait > 0 {
		return h.sleep(ctx, wait)
	}
	return nil
}

func (h *Handlers) logModelFailure(msg string, id uuid.UUID, err error) {
	fields := []zap.Field{zap.String("book_id", id.String()), zap.Error(err)}
	if errors.Is(err, llm.ErrTransient) {
		h.log.Warn(msg+": transient model failure", fields...)
		return
	}
	// A non-transient failure (e.g. a 400) is a config or code bug.
	h.log.Error(msg+": model call failed", fields...)
}
