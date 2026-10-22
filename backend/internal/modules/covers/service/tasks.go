package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// Task types and queue. The queue is served by its own asynq.Server with
// Concurrency 1, so downloads are serialized.
const (
	TaskTypeMirror = "covers:mirror"
	TaskTypeSweep  = "covers:sweep"
	QueueName      = "covers"
)

// MirrorPayload is the payload of TaskTypeMirror.
type MirrorPayload struct {
	BookID string `json:"book_id"`
	Reason string `json:"reason"`
}

// NewMirrorTask builds the copy of one book's cover.
func NewMirrorTask(bookID uuid.UUID, reason string) (*asynq.Task, error) {
	payload, err := json.Marshal(MirrorPayload{BookID: bookID.String(), Reason: reason})
	if err != nil {
		return nil, fmt.Errorf("covers: marshal mirror payload: %w", err)
	}
	return asynq.NewTask(TaskTypeMirror, payload), nil
}

// MirrorOptions: retries are owned by the database (backoff columns), not
// asynq, so MaxRetry is 0.
func MirrorOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(2 * time.Minute),
		asynq.Unique(10 * time.Minute),
	}
}

// NewSweepTask builds a sweep run (also registered as a cron entry).
func NewSweepTask() *asynq.Task { return asynq.NewTask(TaskTypeSweep, nil) }

// SweepOptions: the sweep runs its batch inline and is not retried.
func SweepOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(30 * time.Minute),
		asynq.Unique(30 * time.Minute),
	}
}

// Enqueuer queues mirror tasks. It satisfies the admin service's
// CoverEnqueuer interface.
type Enqueuer struct {
	client *asynq.Client
}

func NewEnqueuer(client *asynq.Client) *Enqueuer { return &Enqueuer{client: client} }

// EnqueueMirror queues a copy of the book's cover. A duplicate of a task
// already queued is not an error.
func (e *Enqueuer) EnqueueMirror(ctx context.Context, bookID uuid.UUID, reason string) error {
	task, err := NewMirrorTask(bookID, reason)
	if err != nil {
		return err
	}
	if _, err := e.client.EnqueueContext(ctx, task, MirrorOptions()...); err != nil && !errors.Is(err, asynq.ErrDuplicateTask) {
		return err
	}
	return nil
}

// Handlers adapts the service to asynq.
type Handlers struct {
	svc   *Service
	batch int
	log   *zap.Logger
}

func NewHandlers(svc *Service, sweepBatch int, log *zap.Logger) *Handlers {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handlers{svc: svc, batch: sweepBatch, log: log}
}

// HandleMirror runs one copy attempt. Failures are stored on the book and
// never returned, so asynq does not retry.
func (h *Handlers) HandleMirror(ctx context.Context, t *asynq.Task) error {
	var p MirrorPayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("covers: decode mirror payload: %v: %w", err, asynq.SkipRetry)
	}
	id, err := uuid.Parse(p.BookID)
	if err != nil {
		return fmt.Errorf("covers: bad book id: %v: %w", err, asynq.SkipRetry)
	}
	if err := h.svc.MirrorOne(ctx, id, p.Reason); err != nil {
		h.log.Error("cover mirror failed", zap.String("book_id", p.BookID), zap.Error(err))
	}
	return nil
}

// HandleSweep runs one sweep.
func (h *Handlers) HandleSweep(ctx context.Context, _ *asynq.Task) error {
	if _, err := h.svc.Sweep(ctx, h.batch); err != nil {
		h.log.Error("cover sweep failed", zap.Error(err))
	}
	return nil
}
