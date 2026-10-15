package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	curatedservice "github.com/bookhaven/backend/internal/modules/curated/service"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type taskHarness struct {
	repo   *fakeRepo
	lookup *fakeLookup
	model  *scriptedLLM
	final  bool
	sleeps []time.Duration
	clock  time.Time
	h      *Handlers
	col    uuid.UUID
}

func newTaskHarness(steps ...step) *taskHarness {
	hs := &taskHarness{repo: newFakeRepo(), lookup: newFakeLookup(), model: &scriptedLLM{steps: steps}, clock: now0, col: uuid.New()}
	hs.lookup.infos[hs.col] = &curatedservice.ManualInfo{Title: "Фантастика", Description: "про космос", GenreName: "Фантастика", Size: 0}
	hs.h = NewHandlers(hs.repo, hs.lookup, hs.model, "claude-sonnet-5-5", nil,
		WithHandlerClock(func() time.Time { return hs.clock }),
		WithSleep(func(ctx context.Context, d time.Duration) error { hs.sleeps = append(hs.sleeps, d); return nil }),
		WithFinalAttempt(func(context.Context) bool { return hs.final }))
	return hs
}

func (hs *taskHarness) draft(count int) uuid.UUID {
	return seed(hs.repo, hs.col, domain.StatusQueued, count, now0.Add(-time.Minute), now0.Add(-time.Minute))
}

func (hs *taskHarness) run(t *testing.T, id uuid.UUID) error {
	t.Helper()
	task, err := NewDraftTask(id)
	if err != nil {
		t.Fatal(err)
	}
	return hs.h.HandleDraft(context.Background(), task)
}

func ok(r llm.StructuredResponse) step { return step{resp: r} }

func TestDraftTaskAndOptions(t *testing.T) {
	id := uuid.New()
	task, _ := NewDraftTask(id)
	var p DraftPayload
	if task.Type() != "ai_collections:draft" || json.Unmarshal(task.Payload(), &p) != nil || p.DraftID != id.String() {
		t.Fatalf("task = %s %s", task.Type(), task.Payload())
	}
	opts := map[asynq.OptionType]any{}
	for _, o := range DraftOptions() {
		opts[o.Type()] = o.Value()
	}
	if QueueName != "ai_collections" || opts[asynq.QueueOpt] != "ai_collections" || opts[asynq.MaxRetryOpt] != 2 || opts[asynq.TimeoutOpt] != 30*time.Minute {
		t.Errorf("options = %v", opts)
	}
}

func TestHandleDraft_SingleChunk(t *testing.T) {
	hs := newTaskHarness(ok(books(80, "Книга")), ok(books(0, "")))
	id := hs.draft(120)
	if err := hs.run(t, id); err != nil {
		t.Fatalf("HandleDraft: %v", err)
	}
	d := hs.repo.drafts[id]
	// The model knew only 80: the second request answers nothing new, so
	// generation stops early and the draft is done without an error code.
	if d.Status != domain.StatusDone || d.ErrorCode != nil {
		t.Fatalf("draft = %+v", d)
	}
	if len(hs.repo.proposals[id]) != 80 {
		t.Fatalf("proposals = %d", len(hs.repo.proposals[id]))
	}
	first := hs.model.requests[0]
	if first.System != SystemPrompt || first.MaxTokens != 16000 || first.Schema == nil ||
		!strings.Contains(first.User, "Места: с 1 по 100.") || !strings.Contains(first.User, `"genre":"Фантастика"`) {
		t.Errorf("first request = %+v", first)
	}
	if len(hs.repo.purgedAt) != 1 || !hs.repo.purgedAt[0].Equal(now0.AddDate(0, 0, -30)) {
		t.Errorf("purge = %v", hs.repo.purgedAt)
	}
	if d.Model != "claude-sonnet-5-5" || d.InputTokens != 2000 || d.OutputTokens != 1000 {
		t.Errorf("model/tokens = %+v", d)
	}
}

func TestHandleDraft_ThreeChunksRankedAndDeduped(t *testing.T) {
	// Chunk 2 repeats the last 5 books of chunk 1 and chunk 3 asks for
	// fewer than the model returns.
	c1 := books(100, "Книга")
	c2 := booksFrom(96, 100, "Книга") // 96..195: 96..100 are repeats
	c3 := booksFrom(196, 100, "Книга")
	hs := newTaskHarness(ok(c1), ok(c2), ok(c3))
	id := hs.draft(250)
	if err := hs.run(t, id); err != nil {
		t.Fatalf("HandleDraft: %v", err)
	}
	d := hs.repo.drafts[id]
	if d.Status != domain.StatusDone || d.ErrorCode != nil {
		t.Fatalf("draft = %+v", d)
	}
	ps := hs.repo.proposals[id]
	if len(ps) != 250 {
		t.Fatalf("proposals = %d", len(ps))
	}
	seen := map[string]bool{}
	for i, p := range ps {
		if p.Rank != i+1 {
			t.Fatalf("rank %d at index %d", p.Rank, i)
		}
		if seen[p.Title] {
			t.Fatalf("repeat %q", p.Title)
		}
		seen[p.Title] = true
	}
	if len(hs.model.requests) != 3 {
		t.Fatalf("model calls = %d", len(hs.model.requests))
	}
	if u := hs.model.requests[1].User; !strings.Contains(u, "Места: с 101 по 200.") || !strings.Contains(u, "Уже в списке (не повторять):\nКнига 1 — Автор 1\n") {
		t.Errorf("second chunk message: %s", u[len(u)-200:])
	}
	if u := hs.model.requests[2].User; !strings.Contains(u, "Места: с 196 по 250.") {
		t.Errorf("third chunk range: %s", u)
	}
	if d.InputTokens != 3000 || d.OutputTokens != 1500 {
		t.Errorf("tokens summed over chunks: %+v", d)
	}
}

func TestHandleDraft_NeverStoresMoreThanAsked(t *testing.T) {
	hs := newTaskHarness(ok(books(100, "Книга")))
	id := hs.draft(30)
	if err := hs.run(t, id); err != nil {
		t.Fatal(err)
	}
	if n := len(hs.repo.proposals[id]); n != 30 {
		t.Fatalf("proposals = %d, want 30", n)
	}
}

func TestHandleDraft_EmptyChunkStopsEarly(t *testing.T) {
	hs := newTaskHarness(ok(books(100, "Книга")), ok(books(0, "")))
	id := hs.draft(400)
	if err := hs.run(t, id); err != nil {
		t.Fatal(err)
	}
	d := hs.repo.drafts[id]
	if d.Status != domain.StatusDone || d.ErrorCode != nil || len(hs.repo.proposals[id]) != 100 || len(hs.model.requests) != 2 {
		t.Fatalf("draft = %+v proposals=%d calls=%d", d, len(hs.repo.proposals[id]), len(hs.model.requests))
	}
	// An all-repeats chunk stops too.
	hs = newTaskHarness(ok(books(100, "Книга")), ok(books(100, "Книга")))
	id = hs.draft(400)
	if err := hs.run(t, id); err != nil {
		t.Fatal(err)
	}
	if len(hs.repo.proposals[id]) != 100 || len(hs.model.requests) != 2 || hs.repo.drafts[id].Status != domain.StatusDone {
		t.Fatalf("all-repeat chunk: proposals=%d calls=%d", len(hs.repo.proposals[id]), len(hs.model.requests))
	}
}

func TestHandleDraft_TransientErrorResumesWithoutRepeatingStoredChunks(t *testing.T) {
	hs := newTaskHarness(ok(books(100, "Книга")), step{err: llm.ErrTransient}, ok(booksFrom(101, 50, "Книга")))
	id := hs.draft(150)

	if err := hs.run(t, id); err == nil || errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected a retryable error, got %v", err)
	}
	d := hs.repo.drafts[id]
	if d.Status != domain.StatusRunning || len(hs.repo.proposals[id]) != 100 {
		t.Fatalf("after the transient error: %+v proposals=%d", d, len(hs.repo.proposals[id]))
	}

	if err := hs.run(t, id); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if hs.repo.drafts[id].Status != domain.StatusDone || len(hs.repo.proposals[id]) != 150 {
		t.Fatalf("after the retry: %+v proposals=%d", hs.repo.drafts[id], len(hs.repo.proposals[id]))
	}
	// 1 (chunk 1) + 1 (failed chunk 2) + 1 (resumed chunk 2): chunk 1 is not asked again.
	if len(hs.model.requests) != 3 {
		t.Fatalf("model calls = %d", len(hs.model.requests))
	}
	resumed := hs.model.requests[2].User
	if !strings.Contains(resumed, "Места: с 101 по 150.") || !strings.Contains(resumed, "Книга 100 — Автор 100") {
		t.Errorf("resumed request: %s", resumed)
	}
}

func TestHandleDraft_FinalAttemptTransientKeepsStoredChunks(t *testing.T) {
	hs := newTaskHarness(ok(books(100, "Книга")), step{err: llm.ErrTransient})
	hs.final = true
	id := hs.draft(200)
	err := hs.run(t, id)
	if !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("expected SkipRetry, got %v", err)
	}
	d := hs.repo.drafts[id]
	if d.Status != domain.StatusDone || d.ErrorCode == nil || *d.ErrorCode != domain.ErrorModelError || len(hs.repo.proposals[id]) != 100 {
		t.Fatalf("draft = %+v", d)
	}
}

func TestHandleDraft_FirstChunkFailuresEndFailed(t *testing.T) {
	cases := []struct {
		name string
		step step
		code string
	}{
		{"refusal", ok(stop(llm.StopReasonRefusal)), domain.ErrorModelRefusal},
		{"truncated", ok(stop(llm.StopReasonMaxTokens)), domain.ErrorModelTruncated},
		{"invalid", ok(raw(`{"nope":1}`)), domain.ErrorInvalidOutput},
		{"other error", step{err: errors.New("boom")}, domain.ErrorModelError},
	}
	for _, tc := range cases {
		hs := newTaskHarness(tc.step)
		id := hs.draft(50)
		err := hs.run(t, id)
		d := hs.repo.drafts[id]
		if !errors.Is(err, asynq.SkipRetry) || d.Status != domain.StatusFailed || d.ErrorCode == nil || *d.ErrorCode != tc.code || len(hs.repo.proposals[id]) != 0 {
			t.Errorf("%s: err=%v draft=%+v", tc.name, err, d)
		}
	}
}

func TestHandleDraft_RefusalOnSecondChunkEndsDoneWithCode(t *testing.T) {
	hs := newTaskHarness(ok(books(100, "Книга")), ok(stop(llm.StopReasonRefusal)))
	id := hs.draft(200)
	if err := hs.run(t, id); !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("err = %v", err)
	}
	d := hs.repo.drafts[id]
	if d.Status != domain.StatusDone || d.ErrorCode == nil || *d.ErrorCode != domain.ErrorModelRefusal || len(hs.repo.proposals[id]) != 100 {
		t.Fatalf("draft = %+v proposals=%d", d, len(hs.repo.proposals[id]))
	}
	if d.InputTokens != 1100 {
		t.Errorf("the refused chunk still counts its tokens: %+v", d)
	}
}

func TestHandleDraft_NoOps(t *testing.T) {
	hs := newTaskHarness()
	// Missing draft (its collection was deleted).
	if err := hs.run(t, uuid.New()); err != nil {
		t.Errorf("missing draft: %v", err)
	}
	for _, st := range []string{domain.StatusDone, domain.StatusFailed} {
		id := seed(hs.repo, hs.col, st, 10, now0, now0)
		if err := hs.run(t, id); err != nil || len(hs.model.requests) != 0 || hs.repo.drafts[id].Status != st {
			t.Errorf("%s draft: err=%v calls=%d", st, err, len(hs.model.requests))
		}
	}
	// A bad payload is not retried.
	if err := hs.h.HandleDraft(context.Background(), asynq.NewTask(TaskTypeDraft, []byte("{"))); !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("bad payload: %v", err)
	}
	if err := hs.h.HandleDraft(context.Background(), asynq.NewTask(TaskTypeDraft, []byte(`{"draft_id":"x"}`))); !errors.Is(err, asynq.SkipRetry) {
		t.Errorf("bad id: %v", err)
	}
}

func TestHandleDraft_CollectionGoneFailsWithoutModelCall(t *testing.T) {
	hs := newTaskHarness()
	id := seed(hs.repo, uuid.New(), domain.StatusQueued, 10, now0, now0) // unknown collection
	err := hs.run(t, id)
	d := hs.repo.drafts[id]
	if !errors.Is(err, asynq.SkipRetry) || d.Status != domain.StatusFailed || *d.ErrorCode != domain.ErrorModelError || len(hs.model.requests) != 0 {
		t.Fatalf("err=%v draft=%+v", err, d)
	}
}

func TestHandleDraft_PacesModelCalls(t *testing.T) {
	hs := newTaskHarness(ok(books(100, "Книга")), ok(booksFrom(101, 100, "Книга")))
	id := hs.draft(200)
	if err := hs.run(t, id); err != nil {
		t.Fatal(err)
	}
	// The clock never advances: the second call waits the full interval.
	if len(hs.sleeps) != 1 || hs.sleeps[0] != minCallInterval {
		t.Fatalf("sleeps = %v", hs.sleeps)
	}
}
