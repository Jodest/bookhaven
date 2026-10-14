package repository

import (
	"context"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// schemaDDL recreates the tables with SQLite-native types (the production
// schema in migration 000023 is Postgres-only).
const schemaDDL = `
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	name TEXT
);
CREATE TABLE ai_curation_requests (
	id TEXT PRIMARY KEY,
	prompt TEXT NOT NULL,
	max_names INTEGER NOT NULL,
	status TEXT NOT NULL DEFAULT 'queued',
	error_code TEXT,
	model TEXT NOT NULL DEFAULT '',
	prompt_version INTEGER NOT NULL,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	created_by TEXT REFERENCES users(id) ON DELETE SET NULL,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL,
	finished_at DATETIME
);
CREATE TABLE ai_curation_candidates (
	id TEXT PRIMARY KEY,
	request_id TEXT NOT NULL REFERENCES ai_curation_requests(id) ON DELETE CASCADE,
	position INTEGER NOT NULL,
	name TEXT NOT NULL,
	note TEXT NOT NULL DEFAULT '',
	match_status TEXT NOT NULL DEFAULT 'pending',
	options TEXT NOT NULL DEFAULT '[]',
	updated_at DATETIME NOT NULL,
	UNIQUE (request_id, position)
);
`

var base = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

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
	if err := db.Exec(schemaDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &fixture{t: t, repo: NewGormRepository(db), db: db}
}

func (f *fixture) user(email string, name *string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO users (id, email, name) VALUES (?, ?, ?)", id, email, name).Error; err != nil {
		f.t.Fatalf("insert user: %v", err)
	}
	return id
}

func (f *fixture) request(user uuid.UUID, status string, created, updated time.Time) uuid.UUID {
	f.t.Helper()
	req := &domain.Request{
		Prompt: "русская классика", MaxNames: 20, Status: status, PromptVersion: domain.PromptVersion,
		CreatedBy: &user, CreatedAt: created, UpdatedAt: updated,
	}
	if err := f.repo.CreateRequest(context.Background(), req); err != nil {
		f.t.Fatalf("create request: %v", err)
	}
	return req.ID
}

func (f *fixture) get(id uuid.UUID) *domain.Request {
	f.t.Helper()
	req, err := f.repo.GetRequest(context.Background(), id)
	if err != nil {
		f.t.Fatalf("get request: %v", err)
	}
	return req
}

func strp(s string) *string { return &s }

func TestFindActiveRequest_StatusAndStaleness(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user("a@x", strp("admin"))
	other := f.user("b@x", nil)
	staleBefore := base.Add(-time.Hour)

	f.request(u, domain.StatusDone, base.Add(-10*time.Minute), base.Add(-10*time.Minute))
	f.request(u, domain.StatusRunning, base.Add(-3*time.Hour), base.Add(-2*time.Hour)) // stale
	f.request(other, domain.StatusQueued, base.Add(-time.Minute), base.Add(-time.Minute))

	got, err := f.repo.FindActiveRequest(ctx, u, staleBefore)
	if err != nil || got != nil {
		t.Fatalf("expected no active request, got %+v (%v)", got, err)
	}

	active := f.request(u, domain.StatusQueued, base.Add(-5*time.Minute), base.Add(-5*time.Minute))
	got, err = f.repo.FindActiveRequest(ctx, u, staleBefore)
	if err != nil || got == nil || got.ID != active {
		t.Fatalf("expected active %s, got %+v (%v)", active, got, err)
	}
}

func TestCountRequestsSince(t *testing.T) {
	f := newFixture(t)
	u := f.user("a@x", nil)
	other := f.user("b@x", nil)
	f.request(u, domain.StatusFailed, base.Add(-25*time.Hour), base.Add(-25*time.Hour))
	f.request(u, domain.StatusFailed, base.Add(-2*time.Hour), base.Add(-2*time.Hour))
	f.request(u, domain.StatusDone, base.Add(-time.Hour), base.Add(-time.Hour))
	f.request(other, domain.StatusDone, base.Add(-time.Hour), base.Add(-time.Hour))

	n, err := f.repo.CountRequestsSince(context.Background(), u, base.Add(-24*time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("count = %d (%v), want 2", n, err)
	}
}

func TestSaveModelResult_StoresCandidatesAndIsAtomic(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user("a@x", nil)
	id := f.request(u, domain.StatusRunning, base, base)

	cands := []domain.NewCandidate{{Name: "Иван Гончаров", Note: "«Обломов»"}, {Name: "Лев Толстой"}}
	if err := f.repo.SaveModelResult(ctx, id, "claude-haiku-5-5", 812, 1630, cands, base.Add(time.Minute)); err != nil {
		t.Fatalf("SaveModelResult: %v", err)
	}
	req := f.get(id)
	if req.Model != "claude-haiku-5-5" || req.InputTokens != 812 || req.OutputTokens != 1630 || !req.UpdatedAt.Equal(base.Add(time.Minute)) {
		t.Errorf("request = %+v", req)
	}
	if len(req.Candidates) != 2 || req.Candidates[0].Name != "Иван Гончаров" || req.Candidates[0].Position != 0 ||
		req.Candidates[1].Position != 1 || req.Candidates[1].MatchStatus != domain.MatchPending || req.Candidates[1].Options == nil {
		t.Errorf("candidates = %+v", req.Candidates)
	}

	// A conflicting position makes the second insert fail: nothing of the
	// second call is kept.
	id2 := f.request(u, domain.StatusRunning, base, base)
	if err := f.db.Exec("INSERT INTO ai_curation_candidates (id, request_id, position, name, updated_at) VALUES (?, ?, 1, 'x', ?)", uuid.New(), id2, base).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := f.repo.SaveModelResult(ctx, id2, "m", 1, 2, cands, base.Add(time.Minute)); err == nil {
		t.Fatal("expected a conflict error")
	}
	req2 := f.get(id2)
	if req2.Model != "" || req2.InputTokens != 0 || len(req2.Candidates) != 1 {
		t.Errorf("transaction not rolled back: %+v", req2)
	}
}

func TestSaveCandidateResolution_BumpsRequest(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user("a@x", nil)
	id := f.request(u, domain.StatusRunning, base, base)
	if err := f.repo.SaveModelResult(ctx, id, "m", 0, 0, []domain.NewCandidate{{Name: "А"}, {Name: "Б"}}, base); err != nil {
		t.Fatal(err)
	}
	pending, err := f.repo.ListPendingCandidates(ctx, id)
	if err != nil || len(pending) != 2 {
		t.Fatalf("pending = %+v (%v)", pending, err)
	}
	later := base.Add(5 * time.Minute)
	opts := []domain.Option{{QID: "Q1", Label: "А", Description: "писатель"}}
	if err := f.repo.SaveCandidateResolution(ctx, id, pending[0].ID, domain.MatchMatched, opts, later); err != nil {
		t.Fatal(err)
	}
	req := f.get(id)
	if !req.UpdatedAt.Equal(later) {
		t.Errorf("updated_at = %v, want %v", req.UpdatedAt, later)
	}
	if req.Candidates[0].MatchStatus != domain.MatchMatched || len(req.Candidates[0].Options) != 1 || req.Candidates[0].Options[0].QID != "Q1" {
		t.Errorf("candidate = %+v", req.Candidates[0])
	}
	if req.CandidatesTotal != 2 || req.CandidatesResolved != 1 {
		t.Errorf("counts = %d/%d", req.CandidatesResolved, req.CandidatesTotal)
	}
	pending, _ = f.repo.ListPendingCandidates(ctx, id)
	if len(pending) != 1 || pending[0].Name != "Б" {
		t.Errorf("pending after = %+v", pending)
	}
}

func TestPurgeOlderThan_Cascades(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user("a@x", nil)
	old := f.request(u, domain.StatusDone, base.Add(-31*24*time.Hour), base.Add(-31*24*time.Hour))
	fresh := f.request(u, domain.StatusDone, base.Add(-29*24*time.Hour), base.Add(-29*24*time.Hour))
	for _, id := range []uuid.UUID{old, fresh} {
		if err := f.repo.SaveModelResult(ctx, id, "m", 0, 0, []domain.NewCandidate{{Name: "А"}}, base); err != nil {
			t.Fatal(err)
		}
	}
	n, err := f.repo.PurgeOlderThan(ctx, base.Add(-30*24*time.Hour))
	if err != nil || n != 1 {
		t.Fatalf("purged %d (%v)", n, err)
	}
	if _, err := f.repo.GetRequest(ctx, old); err != ErrNotFound {
		t.Errorf("old request: err = %v", err)
	}
	var orphans int64
	f.db.Model(&candidateRow{}).Where("request_id = ?", old).Count(&orphans)
	if orphans != 0 {
		t.Errorf("orphan candidates: %d", orphans)
	}
	if req := f.get(fresh); len(req.Candidates) != 1 {
		t.Errorf("fresh request lost candidates: %+v", req)
	}
}

func TestListRecentRequests_CountsCreatorAndOrder(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	named := f.user("a@x", strp("admin"))
	unnamed := f.user("b@x", nil)
	first := f.request(named, domain.StatusDone, base.Add(-2*time.Hour), base.Add(-2*time.Hour))
	second := f.request(unnamed, domain.StatusRunning, base.Add(-time.Hour), base.Add(-time.Hour))
	if err := f.repo.SaveModelResult(ctx, first, "m", 0, 0, []domain.NewCandidate{{Name: "А"}, {Name: "Б"}, {Name: "В"}}, base); err != nil {
		t.Fatal(err)
	}
	pending, _ := f.repo.ListPendingCandidates(ctx, first)
	_ = f.repo.SaveCandidateResolution(ctx, first, pending[0].ID, domain.MatchNotFound, nil, base)
	_ = f.repo.SaveCandidateResolution(ctx, first, pending[1].ID, domain.MatchError, nil, base)
	// A request whose creator is gone.
	orphan := &domain.Request{Prompt: "p", MaxNames: 5, Status: domain.StatusFailed, PromptVersion: 1, CreatedAt: base.Add(-3 * time.Hour), UpdatedAt: base}
	if err := f.repo.CreateRequest(ctx, orphan); err != nil {
		t.Fatal(err)
	}

	items, err := f.repo.ListRecentRequests(ctx, 2)
	if err != nil || len(items) != 2 {
		t.Fatalf("items = %+v (%v)", items, err)
	}
	if items[0].ID != second || items[1].ID != first {
		t.Fatalf("order = %s, %s", items[0].ID, items[1].ID)
	}
	if items[0].Creator == nil || items[0].Creator.Username != "b@x" || items[0].CandidatesTotal != 0 {
		t.Errorf("second = %+v", items[0])
	}
	if items[1].Creator == nil || items[1].Creator.Username != "admin" || items[1].CandidatesTotal != 3 || items[1].CandidatesResolved != 2 {
		t.Errorf("first = %+v creator=%+v", items[1], items[1].Creator)
	}

	all, _ := f.repo.ListRecentRequests(ctx, 20)
	if len(all) != 3 || all[2].Creator != nil {
		t.Errorf("orphan request: %+v", all)
	}
}

func TestMarkTransitions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user("a@x", nil)
	id := f.request(u, domain.StatusQueued, base, base)
	if err := f.repo.MarkRunning(ctx, id, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if req := f.get(id); req.Status != domain.StatusRunning {
		t.Fatalf("status = %s", req.Status)
	}
	if err := f.repo.MarkDone(ctx, id, base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	// A terminal request is never set back to running.
	_ = f.repo.MarkRunning(ctx, id, base.Add(3*time.Minute))
	req := f.get(id)
	if req.Status != domain.StatusDone || req.FinishedAt == nil {
		t.Fatalf("request = %+v", req)
	}

	id2 := f.request(u, domain.StatusQueued, base, base)
	if err := f.repo.MarkFailed(ctx, id2, domain.ErrorEnqueueFailed, base); err != nil {
		t.Fatal(err)
	}
	req2 := f.get(id2)
	if req2.Status != domain.StatusFailed || req2.ErrorCode == nil || *req2.ErrorCode != domain.ErrorEnqueueFailed || req2.FinishedAt == nil {
		t.Fatalf("request = %+v", req2)
	}
}
