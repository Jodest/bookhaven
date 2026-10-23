package main

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/bookhaven/backend/internal/pkg/errtrack"
	"github.com/hibiken/asynq"
)

func TestIsFinalFailure(t *testing.T) {
	tests := []struct {
		name           string
		retried, maxRe int
		err            error
		want           bool
	}{
		{"first attempt, retries left", 0, 3, errors.New("x"), false},
		{"intermediate", 2, 3, errors.New("x"), false},
		{"retry count reached max", 3, 3, errors.New("x"), true},
		{"max retry zero", 0, 0, errors.New("x"), true},
		{"skip retry at retry 0", 0, 3, fmt.Errorf("bad payload: %w", asynq.SkipRetry), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := isFinalFailure(tc.retried, tc.maxRe, tc.err); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

type recordingReporter struct {
	mu    sync.Mutex
	tasks []errtrack.TaskFailure
}

func (r *recordingReporter) CaptureHTTP(context.Context, errtrack.HTTPFailure) {}
func (r *recordingReporter) CapturePanic(context.Context, any, string, string) {}
func (r *recordingReporter) CaptureTaskFailure(_ context.Context, f errtrack.TaskFailure) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tasks = append(r.tasks, f)
}
func (r *recordingReporter) snapshot() []errtrack.TaskFailure {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]errtrack.TaskFailure(nil), r.tasks...)
}

func TestWithErrorHandler_SetsHandlerOnEveryConfig(t *testing.T) {
	h := taskErrorHandler(&recordingReporter{})
	l := zapAsynqLogger{}
	cfgs := map[string]asynq.Config{
		"harvest":     harvestServerConfig(l),
		"authPurge":   authPurgeServerConfig(l),
		"schedule":    scheduleServerConfig(l),
		"curated":     curatedServerConfig(l),
		"aiModerate":  aiModerationServerConfig(l),
		"aiCurator":   aiCuratorServerConfig(l),
		"covers":      coversServerConfig(l),
		"aiDescr":     aiDescriptionsServerConfig(l),
		"aiColl":      aiCollectionsServerConfig(l),
		"aiGenres":    aiGenresServerConfig(l),
		"inlineEmail": {Concurrency: 4},
	}
	for name, c := range cfgs {
		if got := withErrorHandler(c, h); got.ErrorHandler == nil {
			t.Errorf("%s: ErrorHandler is nil", name)
		}
	}
}

// End to end through a real asynq server on miniredis: only the final failure
// of a retried task is reported, with type and queue and no payload.
func TestTaskErrorHandler_EndToEnd(t *testing.T) {
	mr := miniredis.RunT(t)
	rep := &recordingReporter{}
	cfg := withErrorHandler(asynq.Config{
		Queues:         map[string]int{"q": 1},
		Concurrency:    1,
		RetryDelayFunc: func(int, error, *asynq.Task) time.Duration { return 10 * time.Millisecond },
		// The retry set is polled every 5s by default; two retries would
		// otherwise take ~10s and race the deadline below.
		DelayedTaskCheckInterval: 50 * time.Millisecond,
	}, taskErrorHandler(rep))
	srv := asynq.NewServer(asynq.RedisClientOpt{Addr: mr.Addr()}, cfg)

	var mu sync.Mutex
	attempts := 0
	mux := asynq.NewServeMux()
	mux.HandleFunc("t:fail", func(context.Context, *asynq.Task) error {
		mu.Lock()
		defer mu.Unlock()
		attempts++
		return errors.New("always fails")
	})
	if err := srv.Start(mux); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer srv.Shutdown()

	client := asynq.NewClient(asynq.RedisClientOpt{Addr: mr.Addr()})
	defer client.Close()
	if _, err := client.Enqueue(asynq.NewTask("t:fail", []byte("secret payload a@b.com")), asynq.Queue("q"), asynq.MaxRetry(2)); err != nil {
		t.Fatalf("enqueue: %v", err)
	}

	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) && len(rep.snapshot()) == 0 {
		time.Sleep(20 * time.Millisecond)
	}
	time.Sleep(200 * time.Millisecond) // no further reports expected
	got := rep.snapshot()
	if len(got) != 1 {
		t.Fatalf("want exactly 1 report, got %d", len(got))
	}
	mu.Lock()
	defer mu.Unlock()
	if attempts != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
	f := got[0]
	if f.Type != "t:fail" || f.Queue != "q" || f.Retried != 2 || f.MaxRetry != 2 {
		t.Errorf("report = %+v", f)
	}
}
