package service

import (
	"context"
	"errors"
	"testing"
	"time"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type svcFixture struct {
	t    *testing.T
	svc  *Service
	repo Repository
	cat  *fakeCatalog
	db   *gorm.DB
	user uuid.UUID
	run  uuid.UUID
}

func newSvcFixture(t *testing.T, active bool) *svcFixture {
	t.Helper()
	repo, db := newRepo(t)
	cat := newFakeCatalog()
	svc := NewService(repo, cat, cat, active, "cfg-model", nil)
	svc.now = func() time.Time { return now0 }
	return &svcFixture{t: t, svc: svc, repo: repo, cat: cat, db: db, user: insertUser(t, db), run: queuedRun(t, repo, domain.TriggerManual)}
}

// suggest stores a merge suggestion (target first) and returns its id.
func (f *svcFixture) suggest(kind string, target uuid.UUID, sources ...uuid.UUID) uuid.UUID {
	f.t.Helper()
	genres := []domain.SuggestionGenre{{GenreID: target, Role: domain.RoleTarget, NameSnapshot: f.cat.genres[target].Name}}
	for _, s := range sources {
		genres = append(genres, domain.SuggestionGenre{GenreID: s, Role: domain.RoleSource, NameSnapshot: f.cat.genres[s].Name})
	}
	return f.store(domain.NewSuggestion{
		Kind: kind, ReasonCodes: []string{"spelling"}, Reason: "r", Confidence: 0.9, Model: "m", PromptVersion: 1, Genres: genres,
	})
}

func (f *svcFixture) suggestJunk(g uuid.UUID) uuid.UUID {
	f.t.Helper()
	return f.store(domain.NewSuggestion{
		Kind: domain.KindJunk, ReasonCodes: []string{"date"}, Reason: "r", Confidence: 0.9, Model: "m", PromptVersion: 1,
		Genres: []domain.SuggestionGenre{{GenreID: g, Role: domain.RoleSource, NameSnapshot: f.cat.genres[g].Name}},
	})
}

func (f *svcFixture) store(n domain.NewSuggestion) uuid.UUID {
	f.t.Helper()
	created, err := f.repo.InsertSuggestions(context.Background(), []domain.NewSuggestion{n}, f.run, now0)
	if err != nil || created != 1 {
		f.t.Fatalf("store suggestion: %d %v", created, err)
	}
	items, _, _ := f.repo.ListSuggestions(context.Background(), listOpen())
	for _, it := range items {
		if it.Genres[0].GenreID == n.Genres[0].GenreID && len(it.Genres) == len(n.Genres) {
			return it.ID
		}
	}
	f.t.Fatal("stored suggestion not found")
	return uuid.Nil
}

func (f *svcFixture) status(id uuid.UUID) string {
	f.t.Helper()
	s, err := f.repo.GetSuggestion(context.Background(), id)
	if err != nil {
		f.t.Fatal(err)
	}
	return s.Status
}

func TestApply_MergeSuggestedTarget(t *testing.T) {
	f := newSvcFixture(t, true)
	fantasy := f.cat.add("Фэнтези", "published", 100)
	fentezi := f.cat.add("Фентези", "pending", 3)
	eng := f.cat.add("Fantasy", "pending", 2)
	id := f.suggest(domain.KindDuplicate, fantasy, fentezi, eng)

	res, err := f.svc.Apply(context.Background(), f.user, id, nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(f.cat.mergeCalls) != 1 || f.cat.mergeTarget[0] != fantasy ||
		len(f.cat.mergeCalls[0]) != 2 || f.cat.mergeCalls[0][0] != fentezi || f.cat.mergeCalls[0][1] != eng {
		t.Fatalf("merge calls: %v -> %v", f.cat.mergeCalls, f.cat.mergeTarget)
	}
	if res.Merge == nil || res.Merge.SourcesMerged != 2 || res.StatusReport != nil {
		t.Fatalf("result: %+v", res)
	}
	got, _ := f.repo.GetSuggestion(context.Background(), id)
	if got.Status != domain.StatusApplied || got.DecidedBy == nil || *got.DecidedBy != f.user ||
		got.AppliedTargetID == nil || *got.AppliedTargetID != fantasy || got.DecidedAt == nil {
		t.Fatalf("suggestion: %+v", got)
	}
	if res.View.AppliedTarget == nil || res.View.AppliedTarget.ID != fantasy {
		t.Fatalf("view: %+v", res.View)
	}
	// An applied suggestion cannot be applied or dismissed again.
	if _, err := f.svc.Apply(context.Background(), f.user, id, nil); !errors.Is(err, ErrNotOpen) {
		t.Fatalf("second apply: %v", err)
	}
	if _, err := f.svc.Dismiss(context.Background(), f.user, id); !errors.Is(err, ErrNotOpen) {
		t.Fatalf("dismiss applied: %v", err)
	}
	if len(f.cat.mergeCalls) != 1 {
		t.Fatal("merge must run once")
	}
}

func TestApply_TargetChosenInsideAndOutsideTheGroup(t *testing.T) {
	f := newSvcFixture(t, true)
	fantasy := f.cat.add("Фэнтези", "published", 100)
	fentezi := f.cat.add("Фентези", "pending", 3)
	eng := f.cat.add("Fantasy", "pending", 2)
	id := f.suggest(domain.KindDuplicate, fantasy, fentezi, eng)

	// Inside the group: the other two are the sources, in suggestion order.
	if _, err := f.svc.Apply(context.Background(), f.user, id, &eng); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if f.cat.mergeTarget[0] != eng || len(f.cat.mergeCalls[0]) != 2 ||
		f.cat.mergeCalls[0][0] != fantasy || f.cat.mergeCalls[0][1] != fentezi {
		t.Fatalf("inside: %v -> %v", f.cat.mergeCalls[0], f.cat.mergeTarget[0])
	}
	got, _ := f.repo.GetSuggestion(context.Background(), id)
	if *got.AppliedTargetID != eng {
		t.Fatalf("applied target: %v", got.AppliedTargetID)
	}

	// Outside the group: every genre of the suggestion is a source.
	f2 := newSvcFixture(t, true)
	a := f2.cat.add("Фэнтези", "published", 100)
	b := f2.cat.add("Фентези", "pending", 3)
	scifi := f2.cat.add("Фантастика", "published", 500)
	id2 := f2.suggest(domain.KindDuplicate, a, b)
	if _, err := f2.svc.Apply(context.Background(), f2.user, id2, &scifi); err != nil {
		t.Fatalf("apply outside: %v", err)
	}
	if f2.cat.mergeTarget[0] != scifi || len(f2.cat.mergeCalls[0]) != 2 || f2.cat.mergeCalls[0][0] != a || f2.cat.mergeCalls[0][1] != b {
		t.Fatalf("outside: %v -> %v", f2.cat.mergeCalls[0], f2.cat.mergeTarget[0])
	}
}

func TestApply_MissingChosenTargetIs404AndChangesNothing(t *testing.T) {
	f := newSvcFixture(t, true)
	a := f.cat.add("Фэнтези", "published", 100)
	b := f.cat.add("Фентези", "pending", 3)
	id := f.suggest(domain.KindDuplicate, a, b)
	missing := uuid.New()
	_, err := f.svc.Apply(context.Background(), f.user, id, &missing)
	if !errors.Is(err, ErrGenreNotFound) {
		t.Fatalf("expected genre not found, got %v", err)
	}
	if f.status(id) != domain.StatusOpen || f.cat.genres[b].MergedIntoID != nil {
		t.Fatal("nothing may change")
	}
}

func TestApply_StaleSuggestionIsRefusedMarkedAndNeverMerged(t *testing.T) {
	f := newSvcFixture(t, true)
	a := f.cat.add("Фэнтези", "published", 100)
	b := f.cat.add("Фентези", "pending", 3)
	c := f.cat.add("Fantasy", "pending", 2)
	id := f.suggest(domain.KindDuplicate, a, b, c)
	f.cat.genres[c].Status = "rejected" // rejected by hand after the analysis

	if _, err := f.svc.Apply(context.Background(), f.user, id, nil); !errors.Is(err, ErrStale) {
		t.Fatalf("expected stale, got %v", err)
	}
	if len(f.cat.mergeCalls) != 0 {
		t.Fatal("a stale suggestion must never reach the merge")
	}
	if f.status(id) != domain.StatusStale {
		t.Fatalf("status = %s", f.status(id))
	}

	// A deleted genre is stale too.
	f2 := newSvcFixture(t, true)
	x := f2.cat.add("Икс", "published", 1)
	y := f2.cat.add("Игрек", "pending", 1)
	id2 := f2.suggest(domain.KindDuplicate, x, y)
	delete(f2.cat.genres, y)
	if _, err := f2.svc.Apply(context.Background(), f2.user, id2, nil); !errors.Is(err, ErrStale) {
		t.Fatalf("deleted genre: %v", err)
	}
}

func TestApply_JunkRejectsThroughBulkStatus(t *testing.T) {
	f := newSvcFixture(t, true)
	g := f.cat.add("Бестселлер", "published", 7)
	id := f.suggestJunk(g)

	other := uuid.New()
	if _, err := f.svc.Apply(context.Background(), f.user, id, &other); err == nil {
		t.Fatal("target_id on a junk suggestion must be refused")
	} else {
		var in *InputError
		if !errors.As(err, &in) {
			t.Fatalf("expected an input error, got %v", err)
		}
	}
	res, err := f.svc.Apply(context.Background(), f.user, id, nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if f.cat.genres[g].Status != "rejected" || f.cat.statusCalls != 1 || len(f.cat.mergeCalls) != 0 {
		t.Fatalf("junk apply: status=%s calls=%d merges=%d", f.cat.genres[g].Status, f.cat.statusCalls, len(f.cat.mergeCalls))
	}
	if res.StatusReport == nil || res.StatusReport.Updated != 1 || res.Merge != nil {
		t.Fatalf("result: %+v", res)
	}
	got, _ := f.repo.GetSuggestion(context.Background(), id)
	if got.Status != domain.StatusApplied || got.AppliedTargetID != nil {
		t.Fatalf("suggestion: %+v", got)
	}
}

func TestApply_MergeInputErrorsAreInputErrors(t *testing.T) {
	f := newSvcFixture(t, true)
	a := f.cat.add("Фэнтези", "published", 100)
	b := f.cat.add("Фентези", "pending", 3)
	id := f.suggest(domain.KindDuplicate, a, b)
	f.cat.mergeErr = adminservice.ErrGenreMergeTooLarge
	_, err := f.svc.Apply(context.Background(), f.user, id, nil)
	var in *InputError
	if !errors.As(err, &in) {
		t.Fatalf("expected an input error, got %v", err)
	}
	if f.status(id) != domain.StatusOpen {
		t.Fatal("the suggestion stays open")
	}
}

func TestApply_UnknownSuggestionIs404(t *testing.T) {
	f := newSvcFixture(t, true)
	if _, err := f.svc.Apply(context.Background(), f.user, uuid.New(), nil); !errors.Is(err, ErrNotFound) {
		t.Fatalf("got %v", err)
	}
	if _, err := f.svc.Dismiss(context.Background(), f.user, uuid.New()); !errors.Is(err, ErrNotFound) {
		t.Fatalf("dismiss: %v", err)
	}
}

func TestApplyBulk_OverlapMarksSecondStale(t *testing.T) {
	f := newSvcFixture(t, true)
	a := f.cat.add("Фэнтези", "published", 100)
	b := f.cat.add("Фентези", "pending", 3)
	c := f.cat.add("Фантастика", "published", 500)
	d := f.cat.add("Фантастика (рус.)", "pending", 1)
	// Two open suggestions that share b are possible when one was stored
	// before the other became stale; build the overlap directly.
	first := f.suggest(domain.KindDuplicate, a, b)
	second := f.store(domain.NewSuggestion{
		Kind: domain.KindDuplicate, ReasonCodes: []string{"spelling"}, Reason: "r", Confidence: 0.8, Model: "m", PromptVersion: 1,
		Genres: []domain.SuggestionGenre{
			{GenreID: c, Role: domain.RoleTarget, NameSnapshot: "x"},
			{GenreID: d, Role: domain.RoleSource, NameSnapshot: "y"},
		},
	})
	// Make the second one name b too (after the first was stored).
	if err := f.db.Exec("INSERT INTO genre_ai_suggestion_genres (suggestion_id, genre_id, role, position, name_snapshot) VALUES (?, ?, 'source', 2, 'z')", second, b).Error; err != nil {
		t.Fatal(err)
	}
	unknown := uuid.New()
	rep := f.svc.ApplyBulk(context.Background(), f.user, []ApplyItem{{ID: first}, {ID: second}, {ID: unknown}})
	if rep.Processed != 3 || rep.Applied != 1 || rep.Stale != 1 || len(rep.Errors) != 1 || rep.Errors[0].SuggestionID != unknown.String() ||
		rep.Errors[0].Message != "suggestion not found" {
		t.Fatalf("report: %+v", rep)
	}
	if len(f.cat.mergeCalls) != 1 {
		t.Fatalf("only the first merge may run, got %d", len(f.cat.mergeCalls))
	}
	if f.status(first) != domain.StatusApplied || f.status(second) != domain.StatusStale {
		t.Fatalf("statuses: %s %s", f.status(first), f.status(second))
	}
}

func TestDismiss_SingleAndBulk(t *testing.T) {
	f := newSvcFixture(t, true)
	a := f.cat.add("Фэнтези", "published", 100)
	b := f.cat.add("Фентези", "pending", 3)
	g := f.cat.add("Бестселлер", "pending", 7)
	merge := f.suggest(domain.KindDuplicate, a, b)
	junk := f.suggestJunk(g)

	view, err := f.svc.Dismiss(context.Background(), f.user, merge)
	if err != nil || view.Suggestion.Status != domain.StatusDismissed || view.Suggestion.DecidedBy == nil {
		t.Fatalf("dismiss: %+v %v", view, err)
	}
	if f.cat.writes() != 0 {
		t.Fatal("dismissing changes no genre")
	}
	rep := f.svc.DismissBulk(context.Background(), f.user, []uuid.UUID{junk, merge, uuid.New()})
	if rep.Processed != 3 || rep.Dismissed != 1 || len(rep.Errors) != 2 ||
		rep.Errors[0].Message != "suggestion is not open" || rep.Errors[1].Message != "suggestion not found" {
		t.Fatalf("bulk dismiss: %+v", rep)
	}
	// A stale open suggestion can be dismissed.
	h := f.cat.add("Икс", "pending", 1)
	staleID := f.suggestJunk(h)
	f.cat.genres[h].Status = "rejected"
	if _, err := f.svc.Dismiss(context.Background(), f.user, staleID); err != nil {
		t.Fatalf("dismiss stale: %v", err)
	}
}

func TestList_ComputesStaleAndCurrentGenreState(t *testing.T) {
	f := newSvcFixture(t, true)
	a := f.cat.add("Фэнтези", "published", 100)
	b := f.cat.add("Фентези", "pending", 3)
	id := f.suggest(domain.KindDuplicate, a, b)
	f.cat.genres[a].Name = "Фэнтези!" // renamed since

	items, total, err := f.svc.List(context.Background(), ListFilter{})
	if err != nil || total != 1 || len(items) != 1 {
		t.Fatalf("list: %d %v", total, err)
	}
	v := items[0]
	if v.Stale || v.Target == nil || v.Target.Name != "Фэнтези!" || len(v.Genres) != 2 || !v.Genres[0].Exists || v.Genres[0].BooksTotal != 100 {
		t.Fatalf("view: %+v", v)
	}

	// Merged by hand: still listed as open, but stale.
	tid := a
	f.cat.genres[b].MergedIntoID = &tid
	f.cat.genres[b].Status = "rejected"
	items, _, _ = f.svc.List(context.Background(), ListFilter{})
	if len(items) != 1 || !items[0].Stale || !items[0].Genres[1].Merged || items[0].Suggestion.Status != domain.StatusOpen {
		t.Fatalf("stale view: %+v", items[0])
	}
	if f.status(id) != domain.StatusOpen {
		t.Fatal("reading must not write")
	}

	// A deleted genre shows its stored name.
	delete(f.cat.genres, b)
	items, _, _ = f.svc.List(context.Background(), ListFilter{})
	if g := items[0].Genres[1]; g.Exists || g.Name != "Фентези" || len(g.SampleBooks) != 0 {
		t.Fatalf("deleted genre: %+v", g)
	}

	for _, bad := range []ListFilter{{Status: "weird"}, {Kind: "other"}} {
		if _, _, err := f.svc.List(context.Background(), bad); !errors.Is(err, ErrInvalidFilter) {
			t.Fatalf("filter %+v: %v", bad, err)
		}
	}
}

func TestStartAnalysis_Rules(t *testing.T) {
	off := newSvcFixture(t, false)
	if _, err := off.svc.StartAnalysis(context.Background(), off.user); !errors.Is(err, ErrDisabled) {
		t.Fatalf("disabled: %v", err)
	}

	f := newSvcFixture(t, true)
	// The fixture's own run is queued and fresh: a second one is refused.
	if _, err := f.svc.StartAnalysis(context.Background(), f.user); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("already running: %v", err)
	}
	// A run older than 2 hours is interrupted first.
	if err := f.db.Exec("UPDATE genre_ai_runs SET created_at = ?", now0.Add(-3*time.Hour)).Error; err != nil {
		t.Fatal(err)
	}
	run, err := f.svc.StartAnalysis(context.Background(), f.user)
	if err != nil || run.Status != domain.RunQueued || run.Trigger != domain.TriggerManual || run.CreatedBy == nil {
		t.Fatalf("start: %+v %v", run, err)
	}
	old, _ := f.repo.GetRun(context.Background(), f.run)
	if old.Status != domain.RunFailed || *old.ErrorCode != domain.ErrorInterrupted {
		t.Fatalf("old run: %+v", old)
	}
	if err := f.svc.MarkRunEnqueueFailed(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	got, _ := f.repo.GetRun(context.Background(), run.ID)
	if got.Status != domain.RunFailed || *got.ErrorCode != domain.ErrorEnqueueFailed {
		t.Fatalf("enqueue failed: %+v", got)
	}
}

func TestStatus(t *testing.T) {
	f := newSvcFixture(t, true)
	st, err := f.svc.Status(context.Background())
	if err != nil || !st.Enabled || st.Model != "cfg-model" || st.PromptVersion != PromptVersion || st.OpenCount != 0 ||
		st.LastRun == nil || st.LastRun.ID != f.run {
		t.Fatalf("status: %+v %v", st, err)
	}
	// A queued run older than two hours reads as interrupted without a write.
	f.svc.now = func() time.Time { return now0.Add(3 * time.Hour) }
	st, _ = f.svc.Status(context.Background())
	if st.LastRun.Status != domain.RunFailed || *st.LastRun.ErrorCode != domain.ErrorInterrupted {
		t.Fatalf("projected run: %+v", st.LastRun)
	}
	if stored, _ := f.repo.GetRun(context.Background(), f.run); stored.Status != domain.RunQueued {
		t.Fatal("status must not write")
	}
}
