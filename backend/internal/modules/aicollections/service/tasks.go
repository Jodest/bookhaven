package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// Task type and queue. The queue is served by its own asynq.Server with
// Concurrency 1, so one draft is generated at a time.
const (
	TaskTypeDraft = "ai_collections:draft"
	QueueName     = "ai_collections"
)

// minCallInterval is the minimum pause between two model calls.
const minCallInterval = 500 * time.Millisecond

// DraftPayload is the payload of TaskTypeDraft.
type DraftPayload struct {
	DraftID string `json:"draft_id"`
}

// NewDraftTask builds the task that generates one draft.
func NewDraftTask(id uuid.UUID) (*asynq.Task, error) {
	payload, err := json.Marshal(DraftPayload{DraftID: id.String()})
	if err != nil {
		return nil, fmt.Errorf("aicollections: marshal draft payload: %w", err)
	}
	return asynq.NewTask(TaskTypeDraft, payload), nil
}

// DraftOptions: two retries (a retry resumes after the stored chunks and
// never repeats their model calls) and a 30 min timeout (up to 10 chunks).
func DraftOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(2),
		asynq.Timeout(30 * time.Minute),
	}
}

// attemptFunc reports whether this is the task final attempt.
type attemptFunc func(ctx context.Context) bool

// asynqFinalAttempt is true when asynq will not retry the task again. A
// context without asynq metadata counts as final (no retry to wait for).
func asynqFinalAttempt(ctx context.Context) bool {
	retried, ok1 := asynq.GetRetryCount(ctx)
	maxRetry, ok2 := asynq.GetMaxRetry(ctx)
	if !ok1 || !ok2 {
		return true
	}
	return retried >= maxRetry
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

// Handlers runs the draft task in the worker.
type Handlers struct {
	repo     Repository
	lookup   CollectionLookup
	model    llm.Client
	modelID  string
	now      func() time.Time
	sleep    func(ctx context.Context, d time.Duration) error
	final    attemptFunc
	log      *zap.Logger
	lastCall time.Time
}

// HandlerOption configures Handlers.
type HandlerOption func(*Handlers)

// WithHandlerClock overrides the clock (tests).
func WithHandlerClock(now func() time.Time) HandlerOption {
	return func(h *Handlers) { h.now = now }
}

// WithSleep overrides the pacing sleep (tests).
func WithSleep(sleep func(ctx context.Context, d time.Duration) error) HandlerOption {
	return func(h *Handlers) { h.sleep = sleep }
}

// WithFinalAttempt overrides the final-attempt check (tests).
func WithFinalAttempt(final func(ctx context.Context) bool) HandlerOption {
	return func(h *Handlers) { h.final = final }
}

// NewHandlers builds the task handlers. modelID is the configured model
// (stored when the provider reports none). A nil logger is replaced by a
// no-op logger.
func NewHandlers(repo Repository, lookup CollectionLookup, model llm.Client, modelID string, log *zap.Logger, opts ...HandlerOption) *Handlers {
	if log == nil {
		log = zap.NewNop()
	}
	h := &Handlers{repo: repo, lookup: lookup, model: model, modelID: modelID, now: time.Now, sleep: sleepCtx, final: asynqFinalAttempt, log: log}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *Handlers) clock() time.Time { return h.now().UTC() }

// HandleDraft generates one draft:
//
//  1. purges drafts older than RetentionDays (best effort);
//  2. loads the draft; a missing (its collection was deleted), done or
//     failed draft is a no-op;
//  3. marks it running and loads the collection context;
//  4. asks the model for chunks of at most ChunkSize books, continuing the
//     ranking after the stored proposals and excluding them; repeats are
//     dropped; a chunk without a new book ends generation early;
//  5. marks the draft done.
//
// A transient model failure on a non-final attempt is returned (asynq
// retries and resumes after the stored chunks). On the final attempt, or for
// any other failure, the draft ends done with error_code when anything is
// stored, else failed with the code (SkipRetry). The brief and the model
// answer are never logged.
func (h *Handlers) HandleDraft(ctx context.Context, t *asynq.Task) error {
	var p DraftPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("aicollections: decode draft payload: %v: %w", err, asynq.SkipRetry)
	}
	id, err := uuid.Parse(p.DraftID)
	if err != nil {
		return fmt.Errorf("aicollections: bad draft id %q: %w", p.DraftID, asynq.SkipRetry)
	}

	h.purge(ctx)

	d, err := h.repo.GetDraft(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("aicollections: load draft %s: %w", id, err)
	}
	if d.Status == domain.StatusDone || d.Status == domain.StatusFailed {
		return nil
	}
	if err := h.repo.MarkRunning(ctx, id, h.clock()); err != nil {
		return fmt.Errorf("aicollections: %w", err)
	}

	info, err := h.lookup.ManualInfo(ctx, d.CollectionID)
	if err != nil {
		h.log.Error("AI collection draft: collection unavailable",
			zap.String("draft_id", id.String()), zap.Error(err))
		if mErr := h.repo.MarkFailed(ctx, id, domain.ErrorModelError, h.clock()); mErr != nil {
			return fmt.Errorf("aicollections: %w", mErr)
		}
		return fmt.Errorf("aicollections: collection of draft %s: %v: %w", id, err, asynq.SkipRetry)
	}
	cc := CollectionContext{Title: info.Title, Description: info.Description, Genre: info.GenreName}

	stored, err := h.repo.ListProposals(ctx, id)
	if err != nil {
		return fmt.Errorf("aicollections: %w", err)
	}
	seen := make(map[string]struct{}, len(stored))
	lines := make([]string, 0, len(stored))
	for _, sp := range stored {
		seen[DedupKey(sp.Title, sp.Author)] = struct{}{}
		lines = append(lines, proposalLine(sp.Title, sp.Author))
	}

	for chunk := 0; len(lines) < d.Count; chunk++ {
		want := d.Count - len(lines)
		if want > domain.ChunkSize {
			want = domain.ChunkSize
		}
		from := len(lines) + 1

		if err := h.pace(ctx); err != nil {
			return fmt.Errorf("aicollections: %w", err)
		}
		resp, err := h.model.Structured(ctx, llm.StructuredRequest{
			System:    SystemPrompt,
			User:      UserMessage(cc, d.Brief, from, from+want-1, lines),
			Schema:    OutputSchema(),
			MaxTokens: draftMaxTokens,
		})
		h.lastCall = h.now()
		if err != nil {
			transient := errors.Is(err, llm.ErrTransient)
			if transient && !h.final(ctx) {
				h.log.Warn("AI collection draft: transient model failure, will retry",
					zap.String("draft_id", id.String()), zap.Int("chunk", chunk), zap.Error(err))
				return err
			}
			h.log.Error("AI collection draft: model call failed",
				zap.String("draft_id", id.String()), zap.Int("chunk", chunk), zap.Bool("transient", transient), zap.Error(err))
			return h.fail(ctx, id, len(lines), domain.ErrorModelError)
		}

		props, code := ParseOutput(resp)
		h.log.Info("AI collection draft model answer",
			zap.String("draft_id", id.String()),
			zap.Int("chunk", chunk),
			zap.String("stop_reason", resp.StopReason),
			zap.Int("input_tokens", resp.InputTokens),
			zap.Int("output_tokens", resp.OutputTokens),
			zap.Int("proposals", len(props)),
			zap.String("error_code", code))
		model := resp.Model
		if model == "" {
			model = h.modelID
		}
		if code != "" {
			// The unusable answer still cost tokens.
			if err := h.repo.AppendProposals(ctx, id, from, nil, model, resp.InputTokens, resp.OutputTokens, h.clock()); err != nil {
				return fmt.Errorf("aicollections: %w", err)
			}
			return h.fail(ctx, id, len(lines), code)
		}

		fresh := make([]domain.NewProposal, 0, len(props))
		for _, np := range props {
			if len(fresh) >= want {
				break
			}
			key := DedupKey(np.Title, np.Author)
			if _, dup := seen[key]; dup {
				continue
			}
			seen[key] = struct{}{}
			fresh = append(fresh, np)
		}
		if err := h.repo.AppendProposals(ctx, id, from, fresh, model, resp.InputTokens, resp.OutputTokens, h.clock()); err != nil {
			return fmt.Errorf("aicollections: %w", err)
		}
		if len(fresh) == 0 {
			break
		}
		for _, np := range fresh {
			lines = append(lines, proposalLine(np.Title, np.Author))
		}
	}

	if err := h.repo.MarkDone(ctx, id, nil, h.clock()); err != nil {
		return fmt.Errorf("aicollections: %w", err)
	}
	h.log.Info("AI collection draft finished",
		zap.String("draft_id", id.String()), zap.Int("proposals", len(lines)))
	return nil
}

// fail ends a draft after an error: done with the error code when anything
// is stored, else failed with it. It returns SkipRetry.
func (h *Handlers) fail(ctx context.Context, id uuid.UUID, stored int, code string) error {
	var err error
	if stored > 0 {
		err = h.repo.MarkDone(ctx, id, &code, h.clock())
	} else {
		err = h.repo.MarkFailed(ctx, id, code, h.clock())
	}
	if err != nil {
		return fmt.Errorf("aicollections: %w", err)
	}
	return fmt.Errorf("aicollections: draft %s ended with %s: %w", id, code, asynq.SkipRetry)
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

func proposalLine(title, author string) string {
	if author == "" {
		return title
	}
	return title + " — " + author
}

// purge deletes drafts older than RetentionDays; a failure is logged, never
// fatal.
func (h *Handlers) purge(ctx context.Context) {
	n, err := h.repo.PurgeOlderThan(ctx, h.clock().AddDate(0, 0, -domain.RetentionDays))
	if err != nil {
		h.log.Warn("AI collection draft: purge of old drafts failed", zap.Error(err))
		return
	}
	if n > 0 {
		h.log.Info("AI collection draft: purged old drafts", zap.Int64("deleted", n))
	}
}
