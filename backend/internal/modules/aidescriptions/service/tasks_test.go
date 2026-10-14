package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/llm"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

var t0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

// fakeClock advances only when the handler sleeps; sleeps are recorded.
type fakeClock struct {
	now    time.Time
	sleeps []time.Duration
}

func (c *fakeClock) Now() time.Time { return c.now }
func (c *fakeClock) Sleep(ctx context.Context, d time.Duration) error {
	c.sleeps = append(c.sleeps, d)
	c.now = c.now.Add(d)
	return nil
}

func newHandlers(repo *fakeRepo, books *fakeBooks, client *fakeLLM, final bool) (*Handlers, *fakeClock) {
	clk := &fakeClock{now: t0}
	h := NewHandlers(repo, books, client, "claude-sonnet-5-5", nil,
		WithHandlerClock(clk.Now, clk.Sleep),
		WithFinalAttempt(func(context.Context) bool { return final }))
	return h, clk
}

func generateTask(t *testing.T, queuedAt time.Time, ids ...uuid.UUID) *asynq.Task {
	t.Helper()
	task, err := NewGenerateTask(ids, queuedAt)
	if err != nil {
		t.Fatal(err)
	}
	return task
}

func TestHandleGenerate_StoresDraftAndSkipsIneligible(t *testing.T) {
	ok := book(catalogDomain.BookStatusPublished, nil)
	pending := book(catalogDomain.BookStatusPending, strp("  "))
	own := book(catalogDomain.BookStatusPublished, strp("Своё описание"))
	rejected := book(catalogDomain.BookStatusRejected, nil)
	deleted := uuid.New()
	repo := newFakeRepo()
	client := &fakeLLM{}
	h, _ := newHandlers(repo, booksOf(ok, pending, own, rejected), client, false)

	if err := h.HandleGenerate(context.Background(), generateTask(t, t0.Add(-time.Minute), ok.ID, own.ID, rejected.ID, deleted, pending.ID)); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(client.requests) != 2 {
		t.Fatalf("model calls = %d, want 2 (ineligible books skipped)", len(client.requests))
	}
	if len(repo.upserts) != 2 || repo.upserts[0].BookID != ok.ID || repo.upserts[1].BookID != pending.ID {
		t.Fatalf("upserts = %+v", repo.upserts)
	}
	d := repo.upserts[0]
	if d.Status != domain.StatusDraft || d.Text != validText || d.Model != "claude-sonnet-5-5" || d.PromptVersion != PromptVersion ||
		d.InputHash != InputHash(InputOf(ok)) || d.InputTokens != 1050 || d.OutputTokens != 600 || d.ErrorCode != nil {
		t.Errorf("stored = %+v", d)
	}
	req := client.requests[0]
	if req.System != SystemPrompt || req.MaxTokens != generateMaxTokens || req.Schema == nil {
		t.Errorf("request = %+v", req)
	}
}

func TestHandleGenerate_UnknownAndFailedStored(t *testing.T) {
	a := book(catalogDomain.BookStatusPublished, nil)
	b := book(catalogDomain.BookStatusPublished, nil)
	repo := newFakeRepo()
	client := &fakeLLM{resps: []llm.StructuredResponse{
		answer(`{"known":false,"description":""}`),
		{StopReason: llm.StopReasonRefusal},
	}}
	h, _ := newHandlers(repo, booksOf(a, b), client, false)
	if err := h.HandleGenerate(context.Background(), generateTask(t, t0, a.ID, b.ID)); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if repo.upserts[0].Status != domain.StatusUnknown || repo.upserts[0].Text != "" {
		t.Errorf("unknown = %+v", repo.upserts[0])
	}
	if repo.upserts[1].Status != domain.StatusFailed || repo.upserts[1].ErrorCode == nil || *repo.upserts[1].ErrorCode != domain.ErrorModelRefusal {
		t.Errorf("failed = %+v", repo.upserts[1])
	}
	// The refusal carries no model id: the configured one is stored.
	if repo.upserts[1].Model != "claude-sonnet-5-5" {
		t.Errorf("model = %q", repo.upserts[1].Model)
	}
}

func TestHandleGenerate_RetrySkipsBooksDoneAfterQueuedAt(t *testing.T) {
	done := book(catalogDomain.BookStatusPublished, nil)
	old := book(catalogDomain.BookStatusPublished, nil)
	repo := newFakeRepo()
	queuedAt := t0.Add(-10 * time.Minute)
	repo.records[done.ID] = domain.Item{Description: domain.Description{BookID: done.ID, Status: domain.StatusDraft, GeneratedAt: t0.Add(-5 * time.Minute)}}
	repo.records[old.ID] = domain.Item{Description: domain.Description{BookID: old.ID, Status: domain.StatusApproved, GeneratedAt: t0.Add(-24 * time.Hour)}}
	client := &fakeLLM{}
	h, _ := newHandlers(repo, booksOf(done, old), client, false)

	if err := h.HandleGenerate(context.Background(), generateTask(t, queuedAt, done.ID, old.ID)); err != nil {
		t.Fatalf("generate: %v", err)
	}
	if len(client.requests) != 1 || len(repo.upserts) != 1 || repo.upserts[0].BookID != old.ID {
		t.Fatalf("calls=%d upserts=%+v: only the book generated before queued_at is regenerated", len(client.requests), repo.upserts)
	}
}

func TestHandleGenerate_TransientRetriesThenSkipRetry(t *testing.T) {
	a := book(catalogDomain.BookStatusPublished, nil)
	b := book(catalogDomain.BookStatusPublished, nil)
	transient := fmt.Errorf("%w: 529 overloaded", llm.ErrTransient)

	// Non-final attempt: the error is returned for a retry, nothing stored
	// for the failed book.
	repo := newFakeRepo()
	client := &fakeLLM{errs: []error{nil, transient}}
	h, _ := newHandlers(repo, booksOf(a, b), client, false)
	err := h.HandleGenerate(context.Background(), generateTask(t, t0.Add(-time.Minute), a.ID, b.ID))
	if err == nil || !errors.Is(err, llm.ErrTransient) || errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("non-final transient: %v", err)
	}
	if len(repo.upserts) != 1 || repo.upserts[0].BookID != a.ID {
		t.Fatalf("upserts = %+v", repo.upserts)
	}

	// Final attempt: SkipRetry.
	repo = newFakeRepo()
	client = &fakeLLM{errs: []error{transient}}
	h, _ = newHandlers(repo, booksOf(a), client, true)
	if err := h.HandleGenerate(context.Background(), generateTask(t, t0, a.ID)); !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("final transient: %v", err)
	}
	if len(repo.upserts) != 0 {
		t.Errorf("nothing must be stored")
	}

	// Non-transient: SkipRetry even on a non-final attempt.
	repo = newFakeRepo()
	client = &fakeLLM{errs: []error{errors.New("400 bad request")}}
	h, _ = newHandlers(repo, booksOf(a), client, false)
	if err := h.HandleGenerate(context.Background(), generateTask(t, t0, a.ID)); !errors.Is(err, asynq.SkipRetry) {
		t.Fatalf("non-transient: %v", err)
	}
	if len(repo.upserts) != 0 {
		t.Errorf("nothing must be stored")
	}
}

func TestHandleSweep_CandidatesStopAndPacing(t *testing.T) {
	a := book(catalogDomain.BookStatusPublished, nil)
	b := book(catalogDomain.BookStatusPublished, nil)
	c := book(catalogDomain.BookStatusPublished, nil)
	repo := newFakeRepo()
	repo.missing = []uuid.UUID{a.ID, b.ID, c.ID}
	transient := fmt.Errorf("%w: 429", llm.ErrTransient)
	client := &fakeLLM{errs: []error{nil, nil, transient}}
	h, clk := newHandlers(repo, booksOf(a, b, c), client, false)

	task, _ := NewSweepTask(domain.MaxBulk)
	if err := h.HandleSweep(context.Background(), task); err != nil {
		t.Fatalf("sweep must return nil after a transient stop: %v", err)
	}
	if len(client.requests) != 3 || len(repo.upserts) != 2 {
		t.Fatalf("calls=%d upserts=%d", len(client.requests), len(repo.upserts))
	}
	// No pause before the first call, then minCallInterval before each
	// following one (the fake clock does not move between calls).
	if len(clk.sleeps) != 2 || clk.sleeps[0] != minCallInterval || clk.sleeps[1] != minCallInterval {
		t.Errorf("sleeps = %v", clk.sleeps)
	}

	repo = newFakeRepo()
	repo.missing = []uuid.UUID{a.ID, b.ID, c.ID}
	client = &fakeLLM{}
	h, _ = newHandlers(repo, booksOf(a, b, c), client, false)
	task, _ = NewSweepTask(2)
	if err := h.HandleSweep(context.Background(), task); err != nil || len(repo.upserts) != 2 {
		t.Fatalf("max 2: err=%v upserts=%d", err, len(repo.upserts))
	}
}

func TestTaskOptions(t *testing.T) {
	if task, err := NewSweepTask(200); err != nil || task.Type() != TaskTypeSweep || string(task.Payload()) != `{"max":200}` {
		t.Errorf("sweep task = %v %v", task, err)
	}
	if len(SweepOptions()) != 4 || len(GenerateOptions()) != 3 {
		t.Errorf("options changed")
	}
}
