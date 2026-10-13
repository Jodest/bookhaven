package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// Task types and queue. The queue is served by its own asynq.Server with
// Concurrency 1, so all model calls are serialized.
const (
	TaskTypeSweep       = "ai_moderation:sweep"
	TaskTypeReviewBooks = "ai_moderation:review_books"
	QueueName           = "ai_moderation"
)

// Sweep triggers.
const (
	TriggerScheduled = "scheduled"
	TriggerManual    = "manual"
)

// ManualSweepMax caps the admin's «all unreviewed» sweep.
const ManualSweepMax = 1000

// MaxReviewBookIDs caps one review_books task (the HTTP limit).
const MaxReviewBookIDs = 100

// SweepPayload is the payload of TaskTypeSweep.
type SweepPayload struct {
	Max     int    `json:"max"`
	Trigger string `json:"trigger"`
}

// ReviewBooksPayload is the payload of TaskTypeReviewBooks.
type ReviewBooksPayload struct {
	BookIDs []string `json:"book_ids"`
}

// NewSweepTask builds a sweep over at most max candidates.
func NewSweepTask(max int, trigger string) (*asynq.Task, error) {
	payload, err := json.Marshal(SweepPayload{Max: max, Trigger: trigger})
	if err != nil {
		return nil, fmt.Errorf("aimoderation: marshal sweep payload: %w", err)
	}
	return asynq.NewTask(TaskTypeSweep, payload), nil
}

// ScheduledSweepOptions: no retries (the next tick continues), a 30 min
// timeout and Unique(1h) as a defense in depth (single worker replica).
func ScheduledSweepOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(30 * time.Minute),
		asynq.Unique(time.Hour),
	}
}

// ManualSweepOptions: like the scheduled sweep, with room for
// ManualSweepMax books; a second manual sweep while one is queued is
// rejected by Unique (asynq.ErrDuplicateTask).
func ManualSweepOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(2 * time.Hour),
		asynq.Unique(2 * time.Hour),
	}
}

// NewReviewBooksTask builds a forced check of the given books.
func NewReviewBooksTask(ids []uuid.UUID) (*asynq.Task, error) {
	strs := make([]string, 0, len(ids))
	for _, id := range ids {
		strs = append(strs, id.String())
	}
	payload, err := json.Marshal(ReviewBooksPayload{BookIDs: strs})
	if err != nil {
		return nil, fmt.Errorf("aimoderation: marshal review payload: %w", err)
	}
	return asynq.NewTask(TaskTypeReviewBooks, payload), nil
}

// ReviewBooksOptions: two retries of the whole (small) batch on a
// transient failure; recomputing already-done books is harmless.
func ReviewBooksOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(2),
		asynq.Timeout(15 * time.Minute),
	}
}

// Handlers runs the AI moderation tasks.
type Handlers struct {
	reviewer *Reviewer
	books    BookSource
	log      *zap.Logger
}

// NewHandlers builds the task handlers. A nil logger is replaced by a
// no-op logger.
func NewHandlers(reviewer *Reviewer, books BookSource, log *zap.Logger) *Handlers {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handlers{reviewer: reviewer, books: books, log: log}
}

// HandleSweep checks up to payload.max candidates (no verdict first, then
// oldest first). It stops at the first failed model call and returns nil
// after logging it, so nothing is stored for that book and the next run
// continues; a database failure is returned.
func (h *Handlers) HandleSweep(ctx context.Context, t *asynq.Task) error {
	var p SweepPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("aimoderation: decode sweep payload: %v: %w", err, asynq.SkipRetry)
	}
	if p.Max <= 0 {
		return nil
	}
	ids, err := h.books.ListAIReviewCandidates(ctx, p.Max, PromptVersion)
	if err != nil {
		return fmt.Errorf("aimoderation sweep: %w", err)
	}
	stored, skipped := 0, 0
	for _, id := range ids {
		res, err := h.reviewer.ReviewBook(ctx, id)
		if err != nil {
			if errors.Is(err, ErrModelCall) {
				h.logModelFailure("AI sweep stopped", id, err)
				break
			}
			return fmt.Errorf("aimoderation sweep: book %s: %w", id, err)
		}
		if res == ReviewStored {
			stored++
		} else {
			skipped++
		}
	}
	h.log.Info("AI sweep finished",
		zap.String("trigger", p.Trigger),
		zap.Int("candidates", len(ids)),
		zap.Int("stored", stored),
		zap.Int("skipped", skipped))
	return nil
}

// HandleReviewBooks recomputes the verdict of every listed book that is
// still pending, even a fresh one. A transient model failure is returned
// so asynq retries the batch; any other model failure skips retries.
func (h *Handlers) HandleReviewBooks(ctx context.Context, t *asynq.Task) error {
	var p ReviewBooksPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("aimoderation: decode review payload: %v: %w", err, asynq.SkipRetry)
	}
	for _, raw := range p.BookIDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			continue
		}
		if _, err := h.reviewer.ReviewBook(ctx, id); err != nil {
			if errors.Is(err, ErrModelCall) {
				h.logModelFailure("AI review stopped", id, err)
				if errors.Is(err, llm.ErrTransient) {
					return err
				}
				return fmt.Errorf("%w: %w", err, asynq.SkipRetry)
			}
			return fmt.Errorf("aimoderation review: book %s: %w", id, err)
		}
	}
	return nil
}

func (h *Handlers) logModelFailure(msg string, id uuid.UUID, err error) {
	fields := []zap.Field{zap.String("book_id", id.String()), zap.Error(err)}
	if errors.Is(err, llm.ErrTransient) {
		h.log.Warn(msg+": transient model failure", fields...)
		return
	}
	// A non-transient failure (e.g. a 400 from a bad request) is a config
	// or code bug: nothing is stored, so no junk verdicts are written.
	h.log.Error(msg+": model call failed", fields...)
}
