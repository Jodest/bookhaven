package service

import (
	"context"
	"time"

	"github.com/hibiken/asynq"
)

// TaskTypeRecomputeSimilarities is the asynq task type of the daily
// similar-books recompute (empty payload).
const TaskTypeRecomputeSimilarities = "similarity:recompute"

// SimilarityQueue is the dedicated asynq queue of the recompute task,
// served by its own asynq.Server (concurrency 1) in cmd/worker so a long
// batch never blocks harvest or email.
const SimilarityQueue = "similarity"

// RecomputeTaskOptions are the fixed options of the recompute task. Unique
// rejects a duplicate enqueue (e.g. two schedulers) within 6 hours.
func RecomputeTaskOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(SimilarityQueue),
		asynq.MaxRetry(2),
		asynq.Timeout(30 * time.Minute),
		asynq.Unique(6 * time.Hour),
	}
}

// NewRecomputeSimilaritiesTask builds the recompute task.
func NewRecomputeSimilaritiesTask() *asynq.Task {
	return asynq.NewTask(TaskTypeRecomputeSimilarities, nil)
}

// HandleTask runs the recompute. The error is returned as-is so asynq
// retries it (up to MaxRetry).
func (j *SimilarityJob) HandleTask(ctx context.Context, _ *asynq.Task) error {
	_, err := j.Run(ctx)
	return err
}
