package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	curateddomain "github.com/bookhaven/backend/internal/modules/curated/domain"
	curatedservice "github.com/bookhaven/backend/internal/modules/curated/service"
	"github.com/google/uuid"
)

type svcHarness struct {
	repo    *fakeRepo
	lookup  *fakeLookup
	matcher *fakeMatcher
	svc     *Service
	col     uuid.UUID
	user    uuid.UUID
}

func newSvc(active bool, size int) *svcHarness {
	h := &svcHarness{repo: newFakeRepo(), lookup: newFakeLookup(), matcher: &fakeMatcher{}, col: uuid.New(), user: uuid.New()}
	h.lookup.infos[h.col] = &curatedservice.ManualInfo{Title: "Фантастика", Size: size}
	h.svc = NewService(h.repo, h.lookup, h.matcher, active, "claude-sonnet-5-5", WithClock(fixedClock))
	return h
}

func (h *svcHarness) create(brief string, count *int) (*domain.Draft, error) {
	return h.svc.CreateDraft(context.Background(), h.user, h.col.String(), brief, count)
}

func intp(n int) *int { return &n }

func TestCreateDraft_ErrorsInSpecOrder(t *testing.T) {
	// 503 wins over everything else, even an invalid body.
	h := newSvc(false, 10)
	if _, err := h.svc.CreateDraft(context.Background(), h.user, "bad", "", intp(0)); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled: %v", err)
	}
	if len(h.repo.drafts) != 0 {
		t.Fatal("a disabled feature must store nothing")
	}

	h = newSvc(true, 10)
	ctx := context.Background()
	// 400: collection id, then brief, then count; before the collection lookup.
	if _, err := h.svc.CreateDraft(ctx, h.user, "", "x", nil); !errors.Is(err, ErrInvalidCollectionID) {
		t.Errorf("missing collection id: %v", err)
	}
	if _, err := h.svc.CreateDraft(ctx, h.user, "nope", "x", nil); !errors.Is(err, ErrInvalidCollectionID) {
		t.Errorf("bad collection id: %v", err)
	}
	unknown := uuid.NewString()
	if _, err := h.svc.CreateDraft(ctx, h.user, unknown, "   ", intp(0)); !errors.Is(err, ErrInvalidBrief) {
		t.Errorf("empty brief: %v", err)
	}
	long := make([]rune, domain.MaxBriefRunes+1)
	for i := range long {
		long[i] = 'я'
	}
	if _, err := h.svc.CreateDraft(ctx, h.user, unknown, string(long), nil); !errors.Is(err, ErrInvalidBrief) {
		t.Errorf("long brief: %v", err)
	}
	for _, c := range []int{0, 1001, -3} {
		if _, err := h.svc.CreateDraft(ctx, h.user, unknown, "x", intp(c)); !errors.Is(err, ErrInvalidCount) {
			t.Errorf("count %d: %v", c, err)
		}
	}
	if h.lookup.calls != 0 {
		t.Errorf("validation must come before the collection lookup, calls = %d", h.lookup.calls)
	}

	// 404 / 422.
	if _, err := h.svc.CreateDraft(ctx, h.user, unknown, "x", nil); !errors.Is(err, ErrCollectionNotFound) {
		t.Errorf("unknown collection: %v", err)
	}
	auto := uuid.New()
	h.lookup.autos[auto] = true
	if _, err := h.svc.CreateDraft(ctx, h.user, auto.String(), "x", nil); !errors.Is(err, ErrCollectionAutomatic) {
		t.Errorf("automatic collection: %v", err)
	}

	// 409 beats 429.
	active := seed(h.repo, h.col, domain.StatusRunning, 50, now0.Add(-10*time.Minute), now0.Add(-time.Minute))
	for i := 0; i < domain.DailyLimit; i++ {
		d := h.repo.drafts[seed(h.repo, uuid.New(), domain.StatusDone, 5, now0.Add(-time.Hour), now0.Add(-time.Hour))]
		d.CreatedBy = &h.user
	}
	_, err := h.create("x", nil)
	var ae *ActiveDraftError
	if !errors.As(err, &ae) || ae.DraftID != active {
		t.Fatalf("active draft: %v", err)
	}

	// 429 once the active draft is gone.
	delete(h.repo.drafts, active)
	if _, err := h.create("x", nil); !errors.Is(err, ErrDailyLimit) {
		t.Fatalf("daily limit: %v", err)
	}
}

func TestCreateDraft_StoresQueuedDraftWithDefaultCount(t *testing.T) {
	for _, tc := range []struct{ size, want int }{{100, 120}, {1000, 1000}, {0, 1}, {7, 9}} {
		h := newSvc(true, tc.size)
		d, err := h.create("  Топ научной фантастики  ", nil)
		if err != nil {
			t.Fatalf("size %d: %v", tc.size, err)
		}
		if d.Status != domain.StatusQueued || d.Count != tc.want || d.Brief != "Топ научной фантастики" ||
			d.CollectionID != h.col || d.PromptVersion != PromptVersion || d.CreatedBy == nil || *d.CreatedBy != h.user {
			t.Errorf("size %d: draft = %+v", tc.size, d)
		}
	}
	h := newSvc(true, 100)
	d, err := h.create("x", intp(33))
	if err != nil || d.Count != 33 {
		t.Fatalf("explicit count: %+v (%v)", d, err)
	}
}

func TestCreateDraft_StaleActiveDraftDoesNotBlock(t *testing.T) {
	h := newSvc(true, 10)
	seed(h.repo, h.col, domain.StatusRunning, 50, now0.Add(-3*time.Hour), now0.Add(-2*time.Hour))
	if _, err := h.create("x", nil); err != nil {
		t.Fatalf("stale draft must not block: %v", err)
	}
}

func TestMarkEnqueueFailed(t *testing.T) {
	h := newSvc(true, 10)
	d, _ := h.create("x", nil)
	if err := h.svc.MarkEnqueueFailed(context.Background(), d.ID); err != nil {
		t.Fatal(err)
	}
	got := h.repo.drafts[d.ID]
	if got.Status != domain.StatusFailed || got.ErrorCode == nil || *got.ErrorCode != domain.ErrorEnqueueFailed {
		t.Errorf("draft = %+v", got)
	}
}

func TestStatusCountsOnlyTheCaller(t *testing.T) {
	h := newSvc(true, 10)
	h.create("a", nil)
	other := seed(h.repo, h.col, domain.StatusDone, 5, now0.Add(-time.Hour), now0.Add(-time.Hour))
	_ = other
	st, err := h.svc.Status(context.Background(), h.user)
	if err != nil || !st.Enabled || st.UsedToday != 1 || st.Model != "claude-sonnet-5-5" {
		t.Fatalf("status = %+v (%v)", st, err)
	}
	off := newSvc(false, 1)
	if st, _ := off.svc.Status(context.Background(), off.user); st.Enabled {
		t.Error("status must work while disabled and report enabled=false")
	}
}

func TestTimeoutProjection(t *testing.T) {
	h := newSvc(true, 10)
	stale := seed(h.repo, h.col, domain.StatusRunning, 50, now0.Add(-3*time.Hour), now0.Add(-2*time.Hour))
	d, rep, err := h.svc.GetDraft(context.Background(), stale)
	if err != nil || rep != nil || d.Status != domain.StatusFailed || d.ErrorCode == nil || *d.ErrorCode != domain.ErrorTimeout {
		t.Fatalf("detail = %+v %v %v", d, rep, err)
	}
	if h.repo.drafts[stale].Status != domain.StatusRunning {
		t.Error("the projection must not write")
	}
	items, err := h.svc.ListDrafts(context.Background(), h.col)
	if err != nil || len(items) != 1 || items[0].Status != domain.StatusFailed || *items[0].ErrorCode != domain.ErrorTimeout {
		t.Fatalf("list = %+v (%v)", items, err)
	}
}

func TestGetDraft_ReportOnlyWhenDone(t *testing.T) {
	h := newSvc(true, 10)
	running := seed(h.repo, h.col, domain.StatusRunning, 50, now0, now0)
	_, rep, err := h.svc.GetDraft(context.Background(), running)
	if err != nil || rep != nil || h.matcher.calls != 0 {
		t.Fatalf("running: report=%v err=%v calls=%d", rep, err, h.matcher.calls)
	}

	done := seed(h.repo, h.col, domain.StatusDone, 2, now0, now0)
	h.repo.proposals[done] = []domain.Proposal{
		{DraftID: done, Rank: 1, Title: "Гиперион", Author: "Дэн Симмонс", OriginalTitle: "Hyperion", Note: "эпос"},
		{DraftID: done, Rank: 2, Title: "Дюна", Author: "Фрэнк Герберт"},
	}
	h.matcher.preview = &curatedservice.PastePreview{BooksCount: 3}
	d, rep, err := h.svc.GetDraft(context.Background(), done)
	if err != nil || rep == nil || d.Status != domain.StatusDone || rep.Preview.BooksCount != 3 {
		t.Fatalf("done: %+v %+v %v", d, rep, err)
	}
	if len(h.matcher.got) != 2 || h.matcher.got[0] != (curatedservice.Proposal{Rank: 1, Title: "Гиперион", Author: "Дэн Симмонс", OriginalTitle: "Hyperion"}) {
		t.Errorf("matcher input = %+v", h.matcher.got)
	}
	if rep.Proposals[1].Note != "эпос" {
		t.Errorf("proposals by rank = %+v", rep.Proposals)
	}

	if _, _, err := h.svc.GetDraft(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
	h.matcher.err = curateddomain.ErrCollectionNotFound
	if _, _, err := h.svc.GetDraft(context.Background(), done); !errors.Is(err, ErrNotFound) {
		t.Errorf("collection gone: %v", err)
	}
}

func TestDomainDefaultCount(t *testing.T) {
	for in, want := range map[int]int{0: 1, 1: 2, 5: 6, 100: 120, 833: 1000, 5000: 1000} {
		if got := domain.DefaultCount(in); got != want {
			t.Errorf("DefaultCount(%d) = %d, want %d", in, got, want)
		}
	}
}
