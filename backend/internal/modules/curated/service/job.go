package service

import (
	"context"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// TaskTypeRecomputeAuto is the asynq task the worker's scheduler enqueues to
// recompute the snapshots of every automatic collection.
const TaskTypeRecomputeAuto = "curated:recompute_auto"

// QueueName is the queue of the recompute task. It is served by its own small
// asynq.Server (Concurrency 1) so it never waits behind a harvest.
const QueueName = "curated"

// NewRecomputeAutoTask builds the (payload-less) recompute task.
func NewRecomputeAutoTask() *asynq.Task {
	return asynq.NewTask(TaskTypeRecomputeAuto, nil)
}

// RecomputeAutoOptions are the fixed options of the scheduled task: a single
// retry (the next night tries again anyway) and Unique as a defense in depth
// against duplicate enqueues.
func RecomputeAutoOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(1),
		asynq.Unique(time.Hour),
	}
}

// Recomputer is the part of *Service the job needs.
type Recomputer interface {
	RecomputeAll(ctx context.Context) error
}

// RecomputeAutoJob handles TaskTypeRecomputeAuto.
type RecomputeAutoJob struct {
	svc Recomputer
}

// NewRecomputeAutoJob builds the job handler.
func NewRecomputeAutoJob(svc Recomputer) *RecomputeAutoJob {
	return &RecomputeAutoJob{svc: svc}
}

// HandleTask recomputes every automatic collection. Per-collection failures
// are logged by RecomputeAll; only a listing failure is returned (and retried).
func (j *RecomputeAutoJob) HandleTask(ctx context.Context, _ *asynq.Task) error {
	if err := j.svc.RecomputeAll(ctx); err != nil {
		return fmt.Errorf("curated recompute: %w", err)
	}
	return nil
}
