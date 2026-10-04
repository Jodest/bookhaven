package main

import (
	"testing"
	"time"

	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	"github.com/hibiken/asynq"
)

type registration struct {
	cronspec string
	task     *asynq.Task
	opts     []asynq.Option
}

// fakeRegistrar is a hand-rolled scheduleRegistrar fake.
type fakeRegistrar struct {
	regs []registration
}

func (f *fakeRegistrar) Register(cronspec string, task *asynq.Task, opts ...asynq.Option) (string, error) {
	f.regs = append(f.regs, registration{cronspec: cronspec, task: task, opts: opts})
	return "entry-1", nil
}

func TestRegisterSchedules_SimilarityDaily(t *testing.T) {
	f := &fakeRegistrar{}
	if err := registerSchedules(f); err != nil {
		t.Fatalf("registerSchedules: %v", err)
	}
	if len(f.regs) != 1 {
		t.Fatalf("expected 1 registration, got %d", len(f.regs))
	}
	r := f.regs[0]
	if r.cronspec != "30 3 * * *" {
		t.Errorf("cronspec = %q, want 30 3 * * *", r.cronspec)
	}
	if r.task.Type() != "similarity:recompute" || r.task.Type() != catalogservice.TaskTypeRecomputeSimilarities {
		t.Errorf("task type = %q", r.task.Type())
	}
	var queue string
	var timeout time.Duration
	for _, o := range r.opts {
		switch o.Type() {
		case asynq.QueueOpt:
			queue = o.Value().(string)
		case asynq.TimeoutOpt:
			timeout = o.Value().(time.Duration)
		}
	}
	if queue != "similarity" || timeout != 30*time.Minute {
		t.Errorf("queue=%q timeout=%v, want similarity/30m", queue, timeout)
	}
}
