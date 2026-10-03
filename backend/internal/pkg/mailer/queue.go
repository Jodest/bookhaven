package mailer

import (
	"context"
	"fmt"

	"github.com/hibiken/asynq"
)

// Enqueuer is what consuming services (api process) depend on.
type Enqueuer interface {
	Enqueue(ctx context.Context, msg Message) error
}

// TaskEnqueuer is the subset of *asynq.Client Queue depends on.
type TaskEnqueuer interface {
	EnqueueContext(ctx context.Context, task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Queue implements Enqueuer on top of an asynq client.
type Queue struct {
	client TaskEnqueuer
}

// NewQueue builds a Queue; client is typically the process's shared
// *asynq.Client.
func NewQueue(client TaskEnqueuer) *Queue {
	return &Queue{client: client}
}

var _ Enqueuer = (*Queue)(nil)

// Enqueue validates msg and enqueues an email:send task.
func (q *Queue) Enqueue(ctx context.Context, msg Message) error {
	task, err := NewSendEmailTask(msg)
	if err != nil {
		return err
	}
	if _, err := q.client.EnqueueContext(ctx, task); err != nil {
		return fmt.Errorf("mailer: enqueue: %w", err)
	}
	return nil
}
