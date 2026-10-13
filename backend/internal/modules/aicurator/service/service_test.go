package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

var now0 = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func fixedClock() time.Time { return now0 }

type fakeFinder struct {
	found map[string]catalogDomain.Author
	err   error
	calls [][]string
}

func (f *fakeFinder) FindAuthorsByWikidataQIDs(ctx context.Context, qids []string) (map[string]catalogDomain.Author, error) {
	f.calls = append(f.calls, qids)
	return f.found, f.err
}

func seed(repo *fakeRepo, user uuid.UUID, status string, created, updated time.Time) uuid.UUID {
	u := user
	req := &domain.Request{Prompt: "p", MaxNames: 20, Status: status, PromptVersion: 1, CreatedBy: &u, CreatedAt: created, UpdatedAt: updated}
	_ = repo.CreateRequest(context.Background(), req)
	return req.ID
}

func intp(n int) *int { return &n }

func TestCreateRequest_CheckOrder(t *testing.T) {
	ctx := context.Background()
	user := uuid.New()

	// 503 before anything else, even for an invalid prompt.
	off := NewService(newFakeRepo(), nil, false, "m", WithClock(fixedClock))
	if _, err := off.CreateRequest(ctx, user, "", intp(99)); !errors.Is(err, ErrDisabled) {
		t.Errorf("disabled: %v", err)
	}

	repo := newFakeRepo()
	svc := NewService(repo, nil, true, "m", WithClock(fixedClock))
	// 400 before 409.
	active := seed(repo, user, domain.StatusRunning, now0.Add(-time.Minute), now0.Add(-time.Minute))
	for name, prompt := range map[string]string{"empty": "   ", "too long": strings.Repeat("я", 501)} {
		if _, err := svc.CreateRequest(ctx, user, prompt, nil); !errors.Is(err, ErrInvalidPrompt) {
			t.Errorf("%s: %v", name, err)
		}
	}
	for _, n := range []int{0, 31, -1} {
		if _, err := svc.CreateRequest(ctx, user, "ok", intp(n)); !errors.Is(err, ErrInvalidMaxNames) {
			t.Errorf("max_names %d: %v", n, err)
		}
	}
	// 409 before 429.
	for i := 0; i < domain.DailyLimit; i++ {
		seed(repo, user, domain.StatusFailed, now0.Add(-time.Duration(i+1)*time.Minute), now0)
	}
	_, err := svc.CreateRequest(ctx, user, "ok", nil)
	var activeErr *ActiveRequestError
	if !errors.As(err, &activeErr) || activeErr.RequestID != active {
		t.Fatalf("active: %v", err)
	}
	// Once the active request goes stale, the daily limit applies.
	repo.requests[active].UpdatedAt = now0.Add(-2 * time.Hour)
	if _, err := svc.CreateRequest(ctx, user, "ok", nil); !errors.Is(err, ErrDailyLimit) {
		t.Fatalf("daily limit: %v", err)
	}
	if len(repo.requests) != domain.DailyLimit+1 {
		t.Errorf("nothing must be stored on rejection, have %d", len(repo.requests))
	}
}

func TestCreateRequest_Stored(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, nil, true, "m", WithClock(fixedClock))
	user := uuid.New()
	// 500 runes exactly is allowed; a request older than 24 h does not count.
	for i := 0; i < domain.DailyLimit; i++ {
		seed(repo, user, domain.StatusDone, now0.Add(-25*time.Hour), now0.Add(-25*time.Hour))
	}
	prompt := "  " + strings.Repeat("я", 500) + "  "
	req, err := svc.CreateRequest(context.Background(), user, prompt, nil)
	if err != nil {
		t.Fatalf("CreateRequest: %v", err)
	}
	if req.Status != domain.StatusQueued || req.MaxNames != domain.DefaultNames || req.CreatedBy == nil || *req.CreatedBy != user ||
		req.Prompt != strings.Repeat("я", 500) || req.PromptVersion != PromptVersion || !req.CreatedAt.Equal(now0) {
		t.Errorf("request = %+v", req)
	}

	other, err := svc.CreateRequest(context.Background(), uuid.New(), "ok", intp(30))
	if err != nil || other.MaxNames != 30 {
		t.Errorf("max_names 30: %+v %v", other, err)
	}

	if err := svc.MarkEnqueueFailed(context.Background(), req.ID); err != nil {
		t.Fatal(err)
	}
	if got := repo.requests[req.ID]; got.Status != domain.StatusFailed || *got.ErrorCode != domain.ErrorEnqueueFailed {
		t.Errorf("enqueue failed: %+v", got)
	}
}

func TestStatus(t *testing.T) {
	repo := newFakeRepo()
	user := uuid.New()
	seed(repo, user, domain.StatusDone, now0.Add(-time.Hour), now0.Add(-time.Hour))
	seed(repo, user, domain.StatusFailed, now0.Add(-30*time.Hour), now0.Add(-30*time.Hour))
	active := seed(repo, user, domain.StatusQueued, now0.Add(-time.Minute), now0.Add(-time.Minute))
	svc := NewService(repo, nil, true, "claude-haiku-5-5", WithClock(fixedClock))
	st, err := svc.Status(context.Background(), user)
	if err != nil || !st.Enabled || st.Model != "claude-haiku-5-5" || st.UsedToday != 2 || st.ActiveRequestID == nil || *st.ActiveRequestID != active {
		t.Fatalf("status = %+v (%v)", st, err)
	}
	st, _ = svc.Status(context.Background(), uuid.New())
	if st.UsedToday != 0 || st.ActiveRequestID != nil {
		t.Errorf("other user = %+v", st)
	}
}

func TestReadModel_TimeoutProjection(t *testing.T) {
	repo := newFakeRepo()
	user := uuid.New()
	stale := seed(repo, user, domain.StatusRunning, now0.Add(-3*time.Hour), now0.Add(-2*time.Hour))
	fresh := seed(repo, user, domain.StatusRunning, now0.Add(-time.Hour), now0.Add(-time.Minute))
	svc := NewService(repo, nil, true, "m", WithClock(fixedClock))

	items, err := svc.ListRecent(context.Background())
	if err != nil || len(items) != 2 {
		t.Fatalf("items = %+v (%v)", items, err)
	}
	for _, it := range items {
		switch it.ID {
		case stale:
			if it.Status != domain.StatusFailed || it.ErrorCode == nil || *it.ErrorCode != domain.ErrorTimeout {
				t.Errorf("stale = %+v", it)
			}
		case fresh:
			if it.Status != domain.StatusRunning || it.ErrorCode != nil {
				t.Errorf("fresh = %+v", it)
			}
		}
	}
	req, _, err := svc.GetRequest(context.Background(), stale)
	if err != nil || req.Status != domain.StatusFailed {
		t.Errorf("detail = %+v (%v)", req, err)
	}
	// Projection only: nothing written.
	if repo.requests[stale].Status != domain.StatusRunning {
		t.Error("projection must not write")
	}
}

func TestGetRequest_Existing(t *testing.T) {
	repo := newFakeRepo()
	user := uuid.New()
	id := seed(repo, user, domain.StatusDone, now0, now0)
	_ = repo.SaveModelResult(context.Background(), id, "m", 0, 0, []domain.NewCandidate{{Name: "А"}, {Name: "Б"}, {Name: "В"}}, now0)
	cands := repo.cands[id]
	_ = repo.SaveCandidateResolution(context.Background(), id, cands[0].ID, domain.MatchMatched, []domain.Option{{QID: "Q1"}}, now0)
	_ = repo.SaveCandidateResolution(context.Background(), id, cands[1].ID, domain.MatchAmbiguous, []domain.Option{{QID: "Q1"}, {QID: "Q2"}}, now0)

	author := catalogDomain.Author{ID: uuid.New(), Name: "А", Slug: "a", Status: "pending"}
	finder := &fakeFinder{found: map[string]catalogDomain.Author{"Q1": author}}
	svc := NewService(repo, finder, true, "m", WithClock(fixedClock))
	req, existing, err := svc.GetRequest(context.Background(), id)
	if err != nil || len(req.Candidates) != 3 {
		t.Fatalf("req = %+v (%v)", req, err)
	}
	if len(finder.calls) != 1 || len(finder.calls[0]) != 2 {
		t.Errorf("finder calls = %v", finder.calls)
	}
	if existing["Q1"].ID != author.ID {
		t.Errorf("existing = %v", existing)
	}

	failing := NewService(repo, &fakeFinder{err: errors.New("db down")}, true, "m", WithClock(fixedClock))
	if _, existing, err := failing.GetRequest(context.Background(), id); err != nil || existing != nil {
		t.Errorf("finder error: existing=%v err=%v", existing, err)
	}

	if _, _, err := svc.GetRequest(context.Background(), uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown: %v", err)
	}
}
