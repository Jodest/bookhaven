package repository

import (
	"context"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// schemaDDL recreates the tables with SQLite-native types (the production
// schema in migration 000028 is Postgres-only).
const schemaDDL = `
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	name TEXT
);
CREATE TABLE curated_collections (
	id TEXT PRIMARY KEY
);
CREATE TABLE ai_collection_drafts (
	id TEXT PRIMARY KEY,
	collection_id TEXT NOT NULL REFERENCES curated_collections(id) ON DELETE CASCADE,
	brief TEXT NOT NULL,
	count INTEGER NOT NULL,
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
CREATE TABLE ai_collection_proposals (
	draft_id TEXT NOT NULL REFERENCES ai_collection_drafts(id) ON DELETE CASCADE,
	rank INTEGER NOT NULL,
	title TEXT NOT NULL,
	author TEXT NOT NULL DEFAULT '',
	original_title TEXT NOT NULL DEFAULT '',
	note TEXT NOT NULL DEFAULT '',
	PRIMARY KEY (draft_id, rank)
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

func (f *fixture) user(email string, name *string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO users (id, email, name) VALUES (?, ?, ?)", id, email, name).Error; err != nil {
		f.t.Fatalf("insert user: %v", err)
	}
	return id
}

func (f *fixture) collection() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO curated_collections (id) VALUES (?)", id).Error; err != nil {
		f.t.Fatalf("insert collection: %v", err)
	}
	return id
}

func (f *fixture) draft(col, user uuid.UUID, status string, created, updated time.Time) uuid.UUID {
	f.t.Helper()
	d := &domain.Draft{
		CollectionID: col, Brief: "научная фантастика", Count: 120, Status: status,
		PromptVersion: domain.PromptVersion, CreatedBy: &user, CreatedAt: created, UpdatedAt: updated,
	}
	if err := f.repo.CreateDraft(context.Background(), d); err != nil {
		f.t.Fatalf("create draft: %v", err)
	}
	return d.ID
}

func (f *fixture) get(id uuid.UUID) *domain.Draft {
	f.t.Helper()
	d, err := f.repo.GetDraft(context.Background(), id)
	if err != nil {
		f.t.Fatalf("get draft: %v", err)
	}
	return d
}

func strp(s string) *string { return &s }

func TestCreateAndGetDraft(t *testing.T) {
	f := newFixture(t)
	col := f.collection()
	u := f.user("a@x", strp("admin"))
	id := f.draft(col, u, domain.StatusQueued, base, base)

	d := f.get(id)
	if d.CollectionID != col || d.Brief != "научная фантастика" || d.Count != 120 || d.Status != domain.StatusQueued ||
		d.ProposalsCount != 0 || d.Creator == nil || d.Creator.Username != "admin" {
		t.Errorf("draft = %+v", d)
	}
	if _, err := f.repo.GetDraft(context.Background(), uuid.New()); err != ErrNotFound {
		t.Errorf("unknown id: %v", err)
	}
}

func TestListDraftsByCollection_NewestFirstWithCreator(t *testing.T) {
	f := newFixture(t)
	col, other := f.collection(), f.collection()
	u := f.user("a@x", nil)
	older := f.draft(col, u, domain.StatusDone, base.Add(-2*time.Hour), base.Add(-2*time.Hour))
	newer := f.draft(col, u, domain.StatusFailed, base.Add(-time.Hour), base.Add(-time.Hour))
	f.draft(other, u, domain.StatusDone, base, base)

	got, err := f.repo.ListDraftsByCollection(context.Background(), col, 20)
	if err != nil || len(got) != 2 || got[0].ID != newer || got[1].ID != older {
		t.Fatalf("list = %+v (%v)", got, err)
	}
	if got[0].Creator == nil || got[0].Creator.Username != "a@x" {
		t.Errorf("creator falls back to email: %+v", got[0].Creator)
	}
	limited, _ := f.repo.ListDraftsByCollection(context.Background(), col, 1)
	if len(limited) != 1 {
		t.Errorf("limit not applied: %d", len(limited))
	}
}

func TestFindActiveDraft_PerCollectionAndStaleness(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	col, other := f.collection(), f.collection()
	u := f.user("a@x", nil)
	staleBefore := base.Add(-time.Hour)

	f.draft(col, u, domain.StatusDone, base.Add(-10*time.Minute), base.Add(-10*time.Minute))
	f.draft(col, u, domain.StatusRunning, base.Add(-3*time.Hour), base.Add(-2*time.Hour)) // stale
	f.draft(other, u, domain.StatusQueued, base.Add(-time.Minute), base.Add(-time.Minute))

	got, err := f.repo.FindActiveDraft(ctx, col, staleBefore)
	if err != nil || got != nil {
		t.Fatalf("expected no active draft, got %+v (%v)", got, err)
	}
	active := f.draft(col, f.user("b@x", nil), domain.StatusQueued, base.Add(-5*time.Minute), base.Add(-5*time.Minute))
	got, err = f.repo.FindActiveDraft(ctx, col, staleBefore)
	if err != nil || got == nil || got.ID != active {
		t.Fatalf("expected active %s, got %+v (%v)", active, got, err)
	}
}

func TestCountDraftsSince(t *testing.T) {
	f := newFixture(t)
	col := f.collection()
	u, other := f.user("a@x", nil), f.user("b@x", nil)
	f.draft(col, u, domain.StatusFailed, base.Add(-25*time.Hour), base.Add(-25*time.Hour))
	f.draft(col, u, domain.StatusFailed, base.Add(-2*time.Hour), base.Add(-2*time.Hour))
	f.draft(col, u, domain.StatusDone, base.Add(-time.Hour), base.Add(-time.Hour))
	f.draft(col, other, domain.StatusDone, base.Add(-time.Hour), base.Add(-time.Hour))

	n, err := f.repo.CountDraftsSince(context.Background(), u, base.Add(-24*time.Hour))
	if err != nil || n != 2 {
		t.Fatalf("count = %d (%v), want 2", n, err)
	}
}

func TestAppendProposals_RankOffsetTokensAndHeartbeat(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user("a@x", nil)
	id := f.draft(f.collection(), u, domain.StatusRunning, base, base)

	first := []domain.NewProposal{{Title: "Гиперион", Author: "Дэн Симмонс", OriginalTitle: "Hyperion", Note: "классика"}, {Title: "Дюна", Author: "Фрэнк Герберт"}}
	if err := f.repo.AppendProposals(ctx, id, 1, first, "claude-sonnet-5-5", 1000, 500, base.Add(time.Minute)); err != nil {
		t.Fatalf("AppendProposals: %v", err)
	}
	if err := f.repo.AppendProposals(ctx, id, 3, []domain.NewProposal{{Title: "Солярис", Author: "Станислав Лем"}}, "claude-sonnet-5-5", 1200, 300, base.Add(2*time.Minute)); err != nil {
		t.Fatalf("AppendProposals: %v", err)
	}

	d := f.get(id)
	if d.InputTokens != 2200 || d.OutputTokens != 800 || d.Model != "claude-sonnet-5-5" || d.ProposalsCount != 3 ||
		!d.UpdatedAt.Equal(base.Add(2*time.Minute)) {
		t.Errorf("draft = %+v", d)
	}
	ps, err := f.repo.ListProposals(ctx, id)
	if err != nil || len(ps) != 3 {
		t.Fatalf("proposals = %+v (%v)", ps, err)
	}
	if ps[0].Rank != 1 || ps[0].Title != "Гиперион" || ps[0].OriginalTitle != "Hyperion" || ps[0].Note != "классика" ||
		ps[2].Rank != 3 || ps[2].Title != "Солярис" {
		t.Errorf("proposals = %+v", ps)
	}

	// A rank conflict rolls the whole call back: tokens are not added.
	if err := f.repo.AppendProposals(ctx, id, 3, []domain.NewProposal{{Title: "Х"}}, "m", 5, 5, base.Add(3*time.Minute)); err == nil {
		t.Fatal("expected a conflict error")
	}
	if d := f.get(id); d.InputTokens != 2200 || d.ProposalsCount != 3 {
		t.Errorf("transaction not rolled back: %+v", d)
	}
}

func TestMarkTransitions(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user("a@x", nil)
	col := f.collection()

	a := f.draft(col, u, domain.StatusQueued, base, base)
	if err := f.repo.MarkRunning(ctx, a, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if d := f.get(a); d.Status != domain.StatusRunning || !d.UpdatedAt.Equal(base.Add(time.Minute)) {
		t.Errorf("running = %+v", d)
	}
	code := domain.ErrorModelTruncated
	if err := f.repo.MarkDone(ctx, a, &code, base.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	d := f.get(a)
	if d.Status != domain.StatusDone || d.ErrorCode == nil || *d.ErrorCode != code || d.FinishedAt == nil {
		t.Errorf("done = %+v", d)
	}
	// A terminal draft is not moved back to running.
	if err := f.repo.MarkRunning(ctx, a, base.Add(3*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if f.get(a).Status != domain.StatusDone {
		t.Error("done draft must stay done")
	}

	b := f.draft(col, u, domain.StatusRunning, base, base)
	if err := f.repo.MarkDone(ctx, b, nil, base); err != nil {
		t.Fatal(err)
	}
	if d := f.get(b); d.ErrorCode != nil {
		t.Errorf("clean done keeps no error code: %+v", d)
	}

	c := f.draft(col, u, domain.StatusRunning, base, base)
	if err := f.repo.MarkFailed(ctx, c, domain.ErrorModelRefusal, base.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if d := f.get(c); d.Status != domain.StatusFailed || d.ErrorCode == nil || *d.ErrorCode != domain.ErrorModelRefusal || d.FinishedAt == nil {
		t.Errorf("failed = %+v", d)
	}
}

func TestPurgeOlderThan(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user("a@x", nil)
	col := f.collection()
	old := f.draft(col, u, domain.StatusDone, base.AddDate(0, 0, -31), base.AddDate(0, 0, -31))
	fresh := f.draft(col, u, domain.StatusDone, base.AddDate(0, 0, -1), base.AddDate(0, 0, -1))
	for _, id := range []uuid.UUID{old, fresh} {
		if err := f.repo.AppendProposals(ctx, id, 1, []domain.NewProposal{{Title: "А"}}, "m", 0, 0, base); err != nil {
			t.Fatal(err)
		}
	}

	n, err := f.repo.PurgeOlderThan(ctx, base.AddDate(0, 0, -domain.RetentionDays))
	if err != nil || n != 1 {
		t.Fatalf("purged = %d (%v)", n, err)
	}
	if _, err := f.repo.GetDraft(ctx, old); err != ErrNotFound {
		t.Errorf("old draft kept: %v", err)
	}
	if ps, _ := f.repo.ListProposals(ctx, old); len(ps) != 0 {
		t.Errorf("old proposals kept: %+v", ps)
	}
	if ps, _ := f.repo.ListProposals(ctx, fresh); len(ps) != 1 {
		t.Errorf("fresh proposals lost: %+v", ps)
	}
}

func TestDeletingCollectionCascadesToDrafts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user("a@x", nil)
	col := f.collection()
	id := f.draft(col, u, domain.StatusDone, base, base)
	if err := f.repo.AppendProposals(ctx, id, 1, []domain.NewProposal{{Title: "А"}}, "m", 0, 0, base); err != nil {
		t.Fatal(err)
	}
	if err := f.db.Exec("DELETE FROM curated_collections WHERE id = ?", col).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.GetDraft(ctx, id); err != ErrNotFound {
		t.Errorf("draft kept after the collection was deleted: %v", err)
	}
	if ps, _ := f.repo.ListProposals(ctx, id); len(ps) != 0 {
		t.Errorf("proposals kept: %+v", ps)
	}
}
