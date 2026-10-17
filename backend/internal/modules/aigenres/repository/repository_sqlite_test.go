package repository

import (
	"context"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// schemaDDL recreates the tables with SQLite-native types (the production
// schema in migration 000032 is Postgres-only). The partial unique index that
// backs the dismissal memory is kept: SQLite supports partial indexes.
const schemaDDL = `
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE
);
CREATE TABLE genre_ai_runs (
	id TEXT PRIMARY KEY,
	trigger TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'queued',
	error_code TEXT,
	model TEXT NOT NULL DEFAULT '',
	prompt_version INTEGER NOT NULL,
	fingerprint TEXT NOT NULL DEFAULT '',
	batches_total INTEGER NOT NULL DEFAULT 0,
	batches_done INTEGER NOT NULL DEFAULT 0,
	suggestions_created INTEGER NOT NULL DEFAULT 0,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
	created_at DATETIME NOT NULL,
	started_at DATETIME,
	finished_at DATETIME
);
CREATE TABLE genre_ai_suggestions (
	id TEXT PRIMARY KEY,
	run_id TEXT REFERENCES genre_ai_runs(id) ON DELETE SET NULL,
	kind TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'open',
	set_key TEXT NOT NULL,
	reason_codes TEXT NOT NULL DEFAULT '[]',
	reason TEXT NOT NULL DEFAULT '',
	confidence REAL NOT NULL,
	model TEXT NOT NULL,
	prompt_version INTEGER NOT NULL,
	applied_target_id TEXT,
	decided_by TEXT REFERENCES users(id) ON DELETE SET NULL,
	decided_at DATETIME,
	created_at DATETIME NOT NULL
);
CREATE UNIQUE INDEX uq_genre_ai_suggestions_set_key_live ON genre_ai_suggestions (set_key) WHERE status IN ('open','dismissed');
CREATE TABLE genre_ai_suggestion_genres (
	suggestion_id TEXT NOT NULL REFERENCES genre_ai_suggestions(id) ON DELETE CASCADE,
	genre_id TEXT NOT NULL,
	role TEXT NOT NULL,
	position INTEGER NOT NULL,
	name_snapshot TEXT NOT NULL,
	PRIMARY KEY (suggestion_id, genre_id)
);
`

var base = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

type fixture struct {
	t    *testing.T
	repo *GormRepository
	db   *gorm.DB
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec("PRAGMA foreign_keys = ON").Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(schemaDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &fixture{t: t, repo: NewGormRepository(db), db: db}
}

func (f *fixture) user() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO users (id, email) VALUES (?, ?)", id, id.String()+"@x.test").Error; err != nil {
		f.t.Fatalf("insert user: %v", err)
	}
	return id
}

func (f *fixture) run(trigger string, at time.Time) uuid.UUID {
	f.t.Helper()
	run := &domain.Run{Trigger: trigger, Status: domain.RunQueued, PromptVersion: domain.PromptVersion, CreatedAt: at}
	if err := f.repo.CreateRun(context.Background(), run); err != nil {
		f.t.Fatalf("create run: %v", err)
	}
	return run.ID
}

// merge builds a merge suggestion: target first, then sources.
func merge(kind string, target uuid.UUID, sources ...uuid.UUID) domain.NewSuggestion {
	genres := []domain.SuggestionGenre{{GenreID: target, Role: domain.RoleTarget, NameSnapshot: "target"}}
	for _, s := range sources {
		genres = append(genres, domain.SuggestionGenre{GenreID: s, Role: domain.RoleSource, NameSnapshot: "source"})
	}
	return domain.NewSuggestion{
		Kind: kind, ReasonCodes: []string{domain.ReasonSpelling}, Reason: "r", Confidence: 0.9,
		Model: "m", PromptVersion: domain.PromptVersion, Genres: genres,
	}
}

func junk(g uuid.UUID) domain.NewSuggestion {
	return domain.NewSuggestion{
		Kind: domain.KindJunk, ReasonCodes: []string{domain.ReasonDate}, Reason: "r", Confidence: 0.8,
		Model: "m", PromptVersion: domain.PromptVersion,
		Genres: []domain.SuggestionGenre{{GenreID: g, Role: domain.RoleSource, NameSnapshot: "junk"}},
	}
}

func TestInsertSuggestions_StoresAndReadsBack(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	runID := f.run(domain.TriggerManual, base)
	target, a, b := uuid.New(), uuid.New(), uuid.New()

	n, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{merge(domain.KindDuplicate, target, a, b), junk(uuid.New())}, runID, base)
	if err != nil || n != 2 {
		t.Fatalf("insert: n=%d err=%v", n, err)
	}
	items, total, err := f.repo.ListSuggestions(ctx, ListFilter{Status: domain.StatusOpen, Limit: 20})
	if err != nil || total != 2 || len(items) != 2 {
		t.Fatalf("list: %d/%d err=%v", len(items), total, err)
	}
	// Highest confidence first.
	if items[0].Kind != domain.KindDuplicate || items[0].Confidence != 0.9 {
		t.Fatalf("unexpected order: %+v", items[0])
	}
	got := items[0]
	if len(got.Genres) != 3 || got.Genres[0].GenreID != target || got.Genres[0].Role != domain.RoleTarget ||
		got.Genres[1].GenreID != a || got.Genres[2].GenreID != b {
		t.Fatalf("genres: %+v", got.Genres)
	}
	if got.Target() == nil || got.Target().GenreID != target || len(got.ReasonCodes) != 1 || got.ReasonCodes[0] != "spelling" {
		t.Fatalf("target/codes: %+v", got)
	}
	if got.RunID == nil || *got.RunID != runID || got.Status != domain.StatusOpen {
		t.Fatalf("run/status: %+v", got)
	}

	only, total, err := f.repo.ListSuggestions(ctx, ListFilter{Status: domain.StatusOpen, Kind: domain.KindJunk, Limit: 20})
	if err != nil || total != 1 || len(only) != 1 || only[0].Kind != domain.KindJunk {
		t.Fatalf("kind filter: %+v err=%v", only, err)
	}
	open, err := f.repo.CountOpen(ctx)
	if err != nil || open != 2 {
		t.Fatalf("count open: %d err=%v", open, err)
	}
}

func TestInsertSuggestions_DismissalMemoryAndOverlap(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	runID := f.run(domain.TriggerManual, base)
	user := f.user()
	fantasy, fentezi, eng := uuid.New(), uuid.New(), uuid.New()

	if n, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{merge(domain.KindDuplicate, fantasy, fentezi)}, runID, base); err != nil || n != 1 {
		t.Fatalf("first insert: %d %v", n, err)
	}
	items, _, _ := f.repo.ListSuggestions(ctx, ListFilter{Status: domain.StatusOpen, Limit: 20})
	id := items[0].ID

	// The same set while open is skipped, even with another target.
	if n, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{merge(domain.KindDuplicate, fentezi, fantasy)}, runID, base); err != nil || n != 0 {
		t.Fatalf("same set while open: %d %v", n, err)
	}
	// A different set that shares an open genre is skipped (overlap).
	if n, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{merge(domain.KindDuplicate, fantasy, fentezi, eng)}, runID, base); err != nil || n != 0 {
		t.Fatalf("overlap with open: %d %v", n, err)
	}

	changed, err := f.repo.MarkDismissed(ctx, id, user, base.Add(time.Hour))
	if err != nil || !changed {
		t.Fatalf("dismiss: %v %v", changed, err)
	}
	// Dismissed: the same set never comes back, in any target order.
	if n, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{merge(domain.KindDuplicate, fentezi, fantasy)}, runID, base); err != nil || n != 0 {
		t.Fatalf("same set after dismissal: %d %v", n, err)
	}
	// A different set is stored: the overlap rule only looks at open ones.
	if n, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{merge(domain.KindDuplicate, fantasy, fentezi, eng)}, runID, base); err != nil || n != 1 {
		t.Fatalf("different set after dismissal: %d %v", n, err)
	}
	// The same genre as junk is another action, so another set.
	if n, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{junk(uuid.New())}, runID, base); err != nil || n != 1 {
		t.Fatalf("junk: %d %v", n, err)
	}
}

func TestInsertSuggestions_OverlapInsideOneBatch(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	runID := f.run(domain.TriggerManual, base)
	shared := uuid.New()
	n, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{
		merge(domain.KindDuplicate, uuid.New(), shared),
		merge(domain.KindNarrow, uuid.New(), shared),
	}, runID, base)
	if err != nil || n != 1 {
		t.Fatalf("expected the second item skipped, got %d %v", n, err)
	}
}

func TestMarkGuards_OnlyOpenChanges(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	runID := f.run(domain.TriggerManual, base)
	user := f.user()
	target := uuid.New()

	if _, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{merge(domain.KindDuplicate, uuid.New(), uuid.New()), junk(uuid.New())}, runID, base); err != nil {
		t.Fatal(err)
	}
	items, _, _ := f.repo.ListSuggestions(ctx, ListFilter{Status: domain.StatusOpen, Limit: 20})
	first, second := items[0].ID, items[1].ID

	ok, err := f.repo.MarkApplied(ctx, first, user, &target, base.Add(time.Minute))
	if err != nil || !ok {
		t.Fatalf("apply: %v %v", ok, err)
	}
	// Two concurrent decisions cannot both win.
	if ok, _ := f.repo.MarkApplied(ctx, first, user, &target, base); ok {
		t.Fatal("a second apply must not change an applied suggestion")
	}
	if ok, _ := f.repo.MarkDismissed(ctx, first, user, base); ok {
		t.Fatal("dismiss must not change an applied suggestion")
	}
	got, err := f.repo.GetSuggestion(ctx, first)
	if err != nil || got.Status != domain.StatusApplied || got.DecidedBy == nil || *got.DecidedBy != user ||
		got.AppliedTargetID == nil || *got.AppliedTargetID != target || got.DecidedAt == nil {
		t.Fatalf("applied state: %+v err=%v", got, err)
	}

	n, err := f.repo.MarkStale(ctx, []uuid.UUID{first, second})
	if err != nil || n != 1 {
		t.Fatalf("stale must change only the open one: %d %v", n, err)
	}
	if ok, _ := f.repo.MarkDismissed(ctx, second, user, base); ok {
		t.Fatal("dismiss must not change a stale suggestion")
	}
	stale, total, _ := f.repo.ListSuggestions(ctx, ListFilter{Status: domain.StatusStale, Limit: 20})
	if total != 1 || stale[0].ID != second {
		t.Fatalf("stale list: %+v", stale)
	}
	if _, err := f.repo.GetSuggestion(ctx, uuid.New()); err != ErrNotFound {
		t.Fatalf("unknown id: %v", err)
	}
	// A stale suggestion frees its set: the same genres can be proposed again.
	if n, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{{
		Kind: stale[0].Kind, ReasonCodes: []string{"date"}, Reason: "r", Confidence: 0.7, Model: "m", PromptVersion: 1,
		Genres: []domain.SuggestionGenre{{GenreID: stale[0].Genres[0].GenreID, Role: domain.RoleSource, NameSnapshot: "x"}},
	}}, runID, base); err != nil || n != 1 {
		t.Fatalf("re-propose after stale: %d %v", n, err)
	}
}

func TestListSuggestions_Pagination(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	runID := f.run(domain.TriggerManual, base)
	for i := 0; i < 5; i++ {
		s := junk(uuid.New())
		s.Confidence = 0.5 + float64(i)/10
		if _, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{s}, runID, base); err != nil {
			t.Fatal(err)
		}
	}
	page, total, err := f.repo.ListSuggestions(ctx, ListFilter{Status: domain.StatusOpen, Limit: 2, Offset: 2})
	if err != nil || total != 5 || len(page) != 2 {
		t.Fatalf("page: %d/%d %v", len(page), total, err)
	}
	if page[0].Confidence != 0.7 || page[1].Confidence != 0.6 {
		t.Fatalf("order: %v %v", page[0].Confidence, page[1].Confidence)
	}
}

func TestRuns_Lifecycle(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if latest, err := f.repo.LatestRun(ctx); err != nil || latest != nil {
		t.Fatalf("no runs yet: %+v %v", latest, err)
	}
	old := f.run(domain.TriggerManual, base.Add(-3*time.Hour))
	if active, err := f.repo.ActiveRun(ctx); err != nil || active == nil || active.ID != old {
		t.Fatalf("active: %+v %v", active, err)
	}
	n, err := f.repo.InterruptStaleRuns(ctx, base.Add(-2*time.Hour), base)
	if err != nil || n != 1 {
		t.Fatalf("interrupt: %d %v", n, err)
	}
	got, _ := f.repo.GetRun(ctx, old)
	if got.Status != domain.RunFailed || got.ErrorCode == nil || *got.ErrorCode != domain.ErrorInterrupted {
		t.Fatalf("interrupted run: %+v", got)
	}
	if active, _ := f.repo.ActiveRun(ctx); active != nil {
		t.Fatalf("no active run expected: %+v", active)
	}

	id := f.run(domain.TriggerScheduled, base)
	if err := f.repo.StartRun(ctx, id, "model-x", "fp1", 3, base); err != nil {
		t.Fatal(err)
	}
	code := domain.ErrorInvalidOutput
	if err := f.repo.AddBatchResult(ctx, id, BatchResult{SuggestionsCreated: 2, InputTokens: 10, OutputTokens: 5, ErrorCode: &code}); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.AddBatchResult(ctx, id, BatchResult{SuggestionsCreated: 1, InputTokens: 10, OutputTokens: 5}); err != nil {
		t.Fatal(err)
	}
	got, _ = f.repo.GetRun(ctx, id)
	if got.Status != domain.RunRunning || got.BatchesDone != 2 || got.SuggestionsCreated != 3 || got.InputTokens != 20 ||
		got.OutputTokens != 10 || got.BatchesTotal != 3 || got.Model != "model-x" || got.ErrorCode == nil || *got.ErrorCode != code {
		t.Fatalf("progress: %+v", got)
	}
	// A done run with a failure code is not clean.
	if err := f.repo.FinishRun(ctx, id, domain.RunDone, nil, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if fp, _ := f.repo.LastCleanFingerprint(ctx); fp != "" {
		t.Fatalf("a run with a failure code must not be clean, got %q", fp)
	}
	clean := f.run(domain.TriggerManual, base.Add(time.Hour))
	if err := f.repo.StartRun(ctx, clean, "m", "fp2", 1, base); err != nil {
		t.Fatal(err)
	}
	if err := f.repo.FinishRun(ctx, clean, domain.RunDone, nil, base.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if fp, _ := f.repo.LastCleanFingerprint(ctx); fp != "fp2" {
		t.Fatalf("clean fingerprint: %q", fp)
	}
	if latest, _ := f.repo.LatestRun(ctx); latest == nil || latest.ID != clean {
		t.Fatalf("latest: %+v", latest)
	}
}

func TestPurgeRunsOlderThan_KeepsSuggestions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	oldRun := f.run(domain.TriggerManual, base.Add(-200*24*time.Hour))
	newRun := f.run(domain.TriggerManual, base)
	if _, err := f.repo.InsertSuggestions(ctx, []domain.NewSuggestion{junk(uuid.New())}, oldRun, base); err != nil {
		t.Fatal(err)
	}
	n, err := f.repo.PurgeRunsOlderThan(ctx, base.Add(-180*24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("purge: %d %v", n, err)
	}
	if _, err := f.repo.GetRun(ctx, oldRun); err != ErrNotFound {
		t.Fatalf("old run must be gone: %v", err)
	}
	if _, err := f.repo.GetRun(ctx, newRun); err != nil {
		t.Fatalf("new run must stay: %v", err)
	}
	items, total, _ := f.repo.ListSuggestions(ctx, ListFilter{Status: domain.StatusOpen, Limit: 20})
	if total != 1 || items[0].RunID != nil {
		t.Fatalf("suggestion must survive with a null run: %+v", items)
	}
}
