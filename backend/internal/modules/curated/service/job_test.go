package service

import (
	"context"
	"errors"
	"testing"

	"github.com/hibiken/asynq"
)

type stubRecomputer struct{ err error }

func (s stubRecomputer) RecomputeAll(context.Context) error { return s.err }

func TestRecomputeAutoJob_HandleTask(t *testing.T) {
	if err := NewRecomputeAutoJob(stubRecomputer{}).HandleTask(context.Background(), NewRecomputeAutoTask()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	boom := errors.New("boom")
	if err := NewRecomputeAutoJob(stubRecomputer{err: boom}).HandleTask(context.Background(), NewRecomputeAutoTask()); !errors.Is(err, boom) {
		t.Fatalf("listing failure must be returned, got %v", err)
	}
}

func TestRecomputeAutoOptions(t *testing.T) {
	var queue string
	var retry int
	for _, o := range RecomputeAutoOptions() {
		switch o.Type() {
		case asynq.QueueOpt:
			queue = o.Value().(string)
		case asynq.MaxRetryOpt:
			retry = o.Value().(int)
		}
	}
	if queue != QueueName || retry != 1 {
		t.Fatalf("queue=%q retry=%d", queue, retry)
	}
}
