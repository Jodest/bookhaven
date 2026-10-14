package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// Task type and queue. The queue is served by its own asynq.Server with
// Concurrency 1, so one curation request runs at a time.
const (
	TaskTypeCurate = "ai_curator:curate"
	QueueName      = "ai_curator"
)

// CuratePayload is the payload of TaskTypeCurate.
type CuratePayload struct {
	RequestID string `json:"request_id"`
}

// NewCurateTask builds the task that processes one request.
func NewCurateTask(id uuid.UUID) (*asynq.Task, error) {
	payload, err := json.Marshal(CuratePayload{RequestID: id.String()})
	if err != nil {
		return nil, fmt.Errorf("aicurator: marshal curate payload: %w", err)
	}
	return asynq.NewTask(TaskTypeCurate, payload), nil
}

// CurateOptions: two retries (a retry never calls the model again once
// candidates are stored) and a 15 min timeout.
func CurateOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(2),
		asynq.Timeout(15 * time.Minute),
	}
}

// attemptFunc reports whether this is the task's final attempt.
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

// Handlers runs the curation task in the worker.
type Handlers struct {
	repo     Repository
	model    llm.Client
	modelID  string
	resolver *Resolver
	now      func() time.Time
	final    attemptFunc
	log      *zap.Logger
}

// HandlerOption configures Handlers.
type HandlerOption func(*Handlers)

// WithHandlerClock overrides the clock (tests).
func WithHandlerClock(now func() time.Time) HandlerOption {
	return func(h *Handlers) { h.now = now }
}

// WithFinalAttempt overrides the final-attempt check (tests).
func WithFinalAttempt(final func(ctx context.Context) bool) HandlerOption {
	return func(h *Handlers) { h.final = final }
}

// NewHandlers builds the task handlers. modelID is the configured model
// (stored when the provider reports none). A nil logger is replaced by a
// no-op logger.
func NewHandlers(repo Repository, model llm.Client, modelID string, resolver *Resolver, log *zap.Logger, opts ...HandlerOption) *Handlers {
	if log == nil {
		log = zap.NewNop()
	}
	h := &Handlers{repo: repo, model: model, modelID: modelID, resolver: resolver, now: time.Now, final: asynqFinalAttempt, log: log}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *Handlers) clock() time.Time { return h.now().UTC() }

// HandleCurate processes one request:
//
//  1. purges requests older than RetentionDays (best effort);
//  2. loads the request; a missing, done or failed request is a no-op;
//  3. marks it running;
//  4. when the model result is not stored yet, calls the model once: a
//     transient failure on a non-final attempt is returned (asynq
//     retries); on the final attempt, or for any other failure, the
//     request fails with model_error (SkipRetry); an unusable answer fails
//     with its code; otherwise the candidates are stored;
//  5. resolves every pending candidate in position order: a Wikidata
//     failure on a non-final attempt is returned (the retry resumes at
//     that candidate); on the final attempt it and every remaining
//     candidate become error;
//  6. marks the request done.
//
// The prompt text is never logged.
func (h *Handlers) HandleCurate(ctx context.Context, t *asynq.Task) error {
	var p CuratePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("aicurator: decode curate payload: %v: %w", err, asynq.SkipRetry)
	}
	id, err := uuid.Parse(p.RequestID)
	if err != nil {
		return fmt.Errorf("aicurator: bad request id %q: %w", p.RequestID, asynq.SkipRetry)
	}

	h.purge(ctx)

	req, err := h.repo.GetRequest(ctx, id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil
		}
		return fmt.Errorf("aicurator: load request %s: %w", id, err)
	}
	if req.Status == domain.StatusDone || req.Status == domain.StatusFailed {
		return nil
	}
	if err := h.repo.MarkRunning(ctx, id, h.clock()); err != nil {
		return fmt.Errorf("aicurator: %w", err)
	}

	if req.CandidatesTotal == 0 && req.Model == "" {
		done, err := h.callModel(ctx, req)
		if err != nil || done {
			return err
		}
	}

	if err := h.resolvePending(ctx, id); err != nil {
		return err
	}
	if err := h.repo.MarkDone(ctx, id, h.clock()); err != nil {
		return fmt.Errorf("aicurator: %w", err)
	}
	h.log.Info("AI curation finished", zap.String("request_id", id.String()))
	return nil
}

// callModel makes the request's single model call and stores the result.
// done is true when the request has ended (failed) and the handler must
// stop.
func (h *Handlers) callModel(ctx context.Context, req *domain.Request) (done bool, err error) {
	resp, err := h.model.Structured(ctx, llm.StructuredRequest{
		System:    SystemPrompt,
		User:      UserMessage(req.Prompt, req.MaxNames),
		Schema:    OutputSchema(),
		MaxTokens: curateMaxTokens,
	})
	if err != nil {
		transient := errors.Is(err, llm.ErrTransient)
		if transient && !h.final(ctx) {
			h.log.Warn("AI curation: transient model failure, will retry",
				zap.String("request_id", req.ID.String()), zap.Error(err))
			return true, err
		}
		h.log.Error("AI curation: model call failed",
			zap.String("request_id", req.ID.String()), zap.Bool("transient", transient), zap.Error(err))
		if mErr := h.repo.MarkFailed(ctx, req.ID, domain.ErrorModelError, h.clock()); mErr != nil {
			return true, fmt.Errorf("aicurator: %w", mErr)
		}
		return true, fmt.Errorf("aicurator: model call: %v: %w", err, asynq.SkipRetry)
	}

	cands, code := ParseOutput(resp, req.MaxNames)
	h.log.Info("AI curation model answer",
		zap.String("request_id", req.ID.String()),
		zap.String("stop_reason", resp.StopReason),
		zap.Int("input_tokens", resp.InputTokens),
		zap.Int("output_tokens", resp.OutputTokens),
		zap.Int("names", len(cands)),
		zap.String("error_code", code))
	if code != "" {
		if err := h.repo.MarkFailed(ctx, req.ID, code, h.clock()); err != nil {
			return true, fmt.Errorf("aicurator: %w", err)
		}
		return true, nil
	}
	model := resp.Model
	if model == "" {
		model = h.modelID
	}
	if err := h.repo.SaveModelResult(ctx, req.ID, model, resp.InputTokens, resp.OutputTokens, cands, h.clock()); err != nil {
		return true, fmt.Errorf("aicurator: %w", err)
	}
	return false, nil
}

// resolvePending resolves every pending candidate in position order.
func (h *Handlers) resolvePending(ctx context.Context, id uuid.UUID) error {
	pending, err := h.repo.ListPendingCandidates(ctx, id)
	if err != nil {
		return fmt.Errorf("aicurator: %w", err)
	}
	for i, c := range pending {
		status, options, err := h.resolver.Resolve(ctx, c.Name)
		if err != nil {
			if !h.final(ctx) {
				h.log.Warn("AI curation: Wikidata failure, will retry",
					zap.String("request_id", id.String()), zap.Int("position", c.Position), zap.Error(err))
				return fmt.Errorf("aicurator: resolve candidate %d: %w", c.Position, err)
			}
			h.log.Error("AI curation: Wikidata failure on the final attempt; remaining candidates marked error",
				zap.String("request_id", id.String()), zap.Int("remaining", len(pending)-i), zap.Error(err))
			for _, rest := range pending[i:] {
				if err := h.repo.SaveCandidateResolution(ctx, id, rest.ID, domain.MatchError, nil, h.clock()); err != nil {
					return fmt.Errorf("aicurator: %w", err)
				}
			}
			return nil
		}
		if err := h.repo.SaveCandidateResolution(ctx, id, c.ID, status, options, h.clock()); err != nil {
			return fmt.Errorf("aicurator: %w", err)
		}
	}
	return nil
}

// purge deletes requests older than RetentionDays; a failure is logged,
// never fatal.
func (h *Handlers) purge(ctx context.Context) {
	n, err := h.repo.PurgeOlderThan(ctx, h.clock().AddDate(0, 0, -domain.RetentionDays))
	if err != nil {
		h.log.Warn("AI curation: purge of old requests failed", zap.Error(err))
		return
	}
	if n > 0 {
		h.log.Info("AI curation: purged old requests", zap.Int64("deleted", n))
	}
}
