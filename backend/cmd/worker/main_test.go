package main

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/config"
	aimodservice "github.com/bookhaven/backend/internal/modules/aimoderation/service"
	catalogservice "github.com/bookhaven/backend/internal/modules/catalog/service"
	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
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
	// errFor fails Register for the given cron spec.
	errFor string
}

func (f *fakeRegistrar) Register(cronspec string, task *asynq.Task, opts ...asynq.Option) (string, error) {
	if f.errFor != "" && cronspec == f.errFor {
		return "", errors.New("invalid cron spec")
	}
	f.regs = append(f.regs, registration{cronspec: cronspec, task: task, opts: opts})
	return "entry-1", nil
}

func TestRegisterSchedules_SimilarityDaily(t *testing.T) {
	f := &fakeRegistrar{}
	if err := registerSchedules(f, &config.Config{}); err != nil {
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

func TestRegisterSchedules_HarvestTickWhenEnabled(t *testing.T) {
	f := &fakeRegistrar{}
	cfg := &config.Config{HarvestScheduleEnabled: true, HarvestScheduleCron: "0 4 * * *", HarvestScheduleBatch: 10}
	if err := registerSchedules(f, cfg); err != nil {
		t.Fatalf("registerSchedules: %v", err)
	}
	if len(f.regs) != 2 {
		t.Fatalf("expected 2 registrations, got %d", len(f.regs))
	}
	if f.regs[0].task.Type() != catalogservice.TaskTypeRecomputeSimilarities {
		t.Errorf("first registration = %q, want the similarity recompute", f.regs[0].task.Type())
	}
	r := f.regs[1]
	if r.cronspec != "0 4 * * *" || r.task.Type() != harvestservice.TaskTypeHarvestScheduleTick {
		t.Errorf("tick registration = %q %q", r.cronspec, r.task.Type())
	}
	var queue string
	for _, o := range r.opts {
		if o.Type() == asynq.QueueOpt {
			queue = o.Value().(string)
		}
	}
	if queue != "harvest_schedule" {
		t.Errorf("tick queue = %q, want harvest_schedule", queue)
	}
}

func TestRegisterSchedules_HarvestTickDisabled(t *testing.T) {
	f := &fakeRegistrar{}
	cfg := &config.Config{HarvestScheduleEnabled: false, HarvestScheduleCron: "0 4 * * *", HarvestScheduleBatch: 10}
	if err := registerSchedules(f, cfg); err != nil {
		t.Fatalf("registerSchedules: %v", err)
	}
	if len(f.regs) != 1 || f.regs[0].task.Type() != catalogservice.TaskTypeRecomputeSimilarities {
		t.Fatalf("expected only the similarity schedule, got %+v", f.regs)
	}
}

func TestRegisterSchedules_InvalidHarvestCronFails(t *testing.T) {
	f := &fakeRegistrar{errFor: "not a cron"}
	cfg := &config.Config{HarvestScheduleEnabled: true, HarvestScheduleCron: "not a cron", HarvestScheduleBatch: 10}
	if err := registerSchedules(f, cfg); err == nil {
		t.Fatal("expected an error for an invalid harvest cron spec")
	}
}

func TestHarvestServerConfig_SerializesHarvestJobs(t *testing.T) {
	c := harvestServerConfig(nil)
	if c.Concurrency != 1 {
		t.Errorf("Concurrency = %d, want 1 (#191)", c.Concurrency)
	}
	if !reflect.DeepEqual(c.Queues, map[string]int{"harvest": 1}) {
		t.Errorf("Queues = %v, want map[harvest:1]", c.Queues)
	}
}

func TestScheduleServerConfig(t *testing.T) {
	c := scheduleServerConfig(nil)
	if c.Concurrency != 1 {
		t.Errorf("Concurrency = %d, want 1", c.Concurrency)
	}
	if !reflect.DeepEqual(c.Queues, map[string]int{"harvest_schedule": 1}) {
		t.Errorf("Queues = %v, want map[harvest_schedule:1]", c.Queues)
	}
}

func TestRegisterSchedules_AIModerationSweepWhenActive(t *testing.T) {
	f := &fakeRegistrar{}
	cfg := &config.Config{AIModerationEnabled: true, AnthropicAPIKey: "sk-test", AIModerationCron: "*/30 * * * *", AIModerationBatch: 50}
	if err := registerSchedules(f, cfg); err != nil {
		t.Fatalf("registerSchedules: %v", err)
	}
	if len(f.regs) != 2 {
		t.Fatalf("expected 2 registrations, got %d", len(f.regs))
	}
	r := f.regs[1]
	if r.cronspec != "*/30 * * * *" || r.task.Type() != aimodservice.TaskTypeSweep {
		t.Errorf("sweep registration = %q %q", r.cronspec, r.task.Type())
	}
	if string(r.task.Payload()) != `{"max":50,"trigger":"scheduled"}` {
		t.Errorf("sweep payload = %s", r.task.Payload())
	}
	var queue string
	for _, o := range r.opts {
		if o.Type() == asynq.QueueOpt {
			queue = o.Value().(string)
		}
	}
	if queue != "ai_moderation" {
		t.Errorf("sweep queue = %q, want ai_moderation", queue)
	}
}

func TestRegisterSchedules_AIModerationInactive(t *testing.T) {
	for name, cfg := range map[string]*config.Config{
		"flag off":   {AnthropicAPIKey: "sk-test", AIModerationCron: "*/30 * * * *", AIModerationBatch: 50},
		"key absent": {AIModerationEnabled: true, AIModerationCron: "*/30 * * * *", AIModerationBatch: 50},
	} {
		f := &fakeRegistrar{}
		if err := registerSchedules(f, cfg); err != nil {
			t.Fatalf("%s: registerSchedules: %v", name, err)
		}
		if len(f.regs) != 1 || f.regs[0].task.Type() != catalogservice.TaskTypeRecomputeSimilarities {
			t.Fatalf("%s: expected only the similarity schedule, got %+v", name, f.regs)
		}
	}
}

func TestRegisterSchedules_InvalidAIModerationCronFails(t *testing.T) {
	f := &fakeRegistrar{errFor: "bad"}
	cfg := &config.Config{AIModerationEnabled: true, AnthropicAPIKey: "sk-test", AIModerationCron: "bad", AIModerationBatch: 50}
	if err := registerSchedules(f, cfg); err == nil {
		t.Fatal("expected an error for an invalid AI moderation cron spec")
	}
}

func TestAIModerationServerConfig(t *testing.T) {
	c := aiModerationServerConfig(nil)
	if c.Concurrency != 1 {
		t.Errorf("Concurrency = %d, want 1", c.Concurrency)
	}
	if !reflect.DeepEqual(c.Queues, map[string]int{"ai_moderation": 1}) {
		t.Errorf("Queues = %v, want map[ai_moderation:1]", c.Queues)
	}
}

func TestAICuratorServerConfig(t *testing.T) {
	c := aiCuratorServerConfig(nil)
	if c.Concurrency != 1 {
		t.Errorf("Concurrency = %d, want 1", c.Concurrency)
	}
	if !reflect.DeepEqual(c.Queues, map[string]int{"ai_curator": 1}) {
		t.Errorf("Queues = %v, want map[ai_curator:1]", c.Queues)
	}
}
