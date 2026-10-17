package service

import (
	"context"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

func newHandlers(t *testing.T, client *scriptedLLM, cat *fakeCatalog) (*Handlers, Repository) {
	t.Helper()
	repo, _ := newRepo(t)
	an := newTestAnalyzer(client, repo, cat, 300)
	h := NewHandlers(an, repo, cat, "cfg-model", nil)
	h.now = func() time.Time { return now0 }
	return h, repo
}

func TestTaskDefinitions(t *testing.T) {
	task, err := NewAnalyzeTask(uuid.New())
	if err != nil || task.Type() != "ai_genres:analyze" {
		t.Fatalf("analyze task: %v %v", task, err)
	}
	if NewScheduledTask().Type() != "ai_genres:scheduled" {
		t.Fatal("scheduled task type")
	}
	for _, opts := range [][]asynq.Option{AnalyzeOptions(), ScheduledOptions()} {
		var queue string
		var retry = -1
		for _, o := range opts {
			switch o.Type() {
			case asynq.QueueOpt:
				queue = o.Value().(string)
			case asynq.MaxRetryOpt:
				retry = o.Value().(int)
			}
		}
		if queue != "ai_genres" || retry != 0 {
			t.Fatalf("options: queue=%q retry=%d", queue, retry)
		}
	}
}

func TestScheduled_SkipsWhenNothingChanged(t *testing.T) {
	cat := newFakeCatalog()
	cat.add("Фэнтези", "published", 10)
	cat.add("Фентези", "pending", 1)
	client := &scriptedLLM{responses: []scripted{jsonResp(answer()), jsonResp(answer()), jsonResp(answer())}}
	h, repo := newHandlers(t, client, cat)
	ctx := context.Background()

	// First tick: nothing analyzed yet, so a scheduled run is made.
	if err := h.HandleScheduled(ctx, NewScheduledTask()); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("first tick must analyze, requests=%d", len(client.requests))
	}
	run, _ := repo.LatestRun(ctx)
	if run.Trigger != domain.TriggerScheduled || run.Status != domain.RunDone {
		t.Fatalf("run: %+v", run)
	}

	// Second tick, same genres: no model call, no new run.
	if err := h.HandleScheduled(ctx, NewScheduledTask()); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("an unchanged catalog must not be analyzed again, requests=%d", len(client.requests))
	}
	if again, _ := repo.LatestRun(ctx); again.ID != run.ID {
		t.Fatal("no run may be created")
	}

	// A harvested genre changes the fingerprint.
	cat.add("Новый жанр", "pending", 0)
	if err := h.HandleScheduled(ctx, NewScheduledTask()); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("a new genre must trigger an analysis, requests=%d", len(client.requests))
	}

	// A rename does too.
	for _, g := range cat.genres {
		if g.Name == "Фентези" {
			g.Name = "Фэнтези (рус.)"
		}
	}
	if err := h.HandleScheduled(ctx, NewScheduledTask()); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 3 {
		t.Fatalf("a rename must trigger an analysis, requests=%d", len(client.requests))
	}
}

func TestScheduled_SkipsWhileARunIsActive(t *testing.T) {
	cat := newFakeCatalog()
	cat.add("Фэнтези", "published", 10)
	client := &scriptedLLM{}
	h, repo := newHandlers(t, client, cat)
	queuedRun(t, repo, domain.TriggerManual)
	if err := h.HandleScheduled(context.Background(), NewScheduledTask()); err != nil || len(client.requests) != 0 {
		t.Fatalf("an active run must block the schedule: %v", err)
	}
}

func TestScheduled_FailedRunsDoNotCountAsAnalyzed(t *testing.T) {
	cat := newFakeCatalog()
	cat.add("Фэнтези", "published", 10)
	cat.add("Фентези", "pending", 1)
	client := &scriptedLLM{responses: []scripted{
		{resp: jsonResp(answer()).resp, err: nil},
	}}
	// The first run fails with a transient error; the next tick must retry.
	failing := &scriptedLLM{responses: []scripted{{err: errTransient()}}}
	h, repo := newHandlers(t, failing, cat)
	ctx := context.Background()
	if err := h.HandleScheduled(ctx, NewScheduledTask()); err != nil {
		t.Fatal(err)
	}
	if run, _ := repo.LatestRun(ctx); run.Status != domain.RunFailed {
		t.Fatalf("run: %+v", run)
	}
	h.analyzer.client = client
	if err := h.HandleScheduled(ctx, NewScheduledTask()); err != nil {
		t.Fatal(err)
	}
	if len(client.requests) != 1 {
		t.Fatalf("a failed run must be retried by the next tick, requests=%d", len(client.requests))
	}
}

func TestHandleAnalyze_RunsTheNamedRun(t *testing.T) {
	cat := newFakeCatalog()
	cat.add("Фэнтези", "published", 10)
	cat.add("Фентези", "pending", 1)
	client := &scriptedLLM{responses: []scripted{jsonResp(answer())}}
	h, repo := newHandlers(t, client, cat)
	runID := queuedRun(t, repo, domain.TriggerManual)
	task, _ := NewAnalyzeTask(runID)
	if err := h.HandleAnalyze(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	run, _ := repo.GetRun(context.Background(), runID)
	if run.Status != domain.RunDone || len(client.requests) != 1 {
		t.Fatalf("run: %+v", run)
	}
	if err := h.HandleAnalyze(context.Background(), asynq.NewTask(TaskTypeAnalyze, []byte("{bad"))); err == nil {
		t.Fatal("a bad payload must fail")
	}
}
