package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// schemaDDL recreates the tables with SQLite-native types (the production
// schema in migration 000024 is Postgres-only).
const schemaDDL = `
CREATE TABLE users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE
);
CREATE TABLE books (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	description TEXT,
	published_year INTEGER,
	status TEXT NOT NULL DEFAULT 'pending',
	created_at DATETIME NOT NULL
);
CREATE TABLE book_ai_descriptions (
	book_id TEXT PRIMARY KEY REFERENCES books(id) ON DELETE CASCADE,
	status TEXT NOT NULL,
	text TEXT NOT NULL DEFAULT '',
	error_code TEXT,
	edited BOOLEAN NOT NULL DEFAULT false,
	model TEXT NOT NULL,
	prompt_version INTEGER NOT NULL,
	input_hash TEXT NOT NULL,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	generated_at DATETIME NOT NULL,
	reviewed_by TEXT REFERENCES users(id) ON DELETE SET NULL,
	reviewed_at DATETIME
);
`

var base = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

type fixture struct {
	t    *testing.T
	repo *GormRepository
	db   *gorm.DB
	n    int
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

func (f *fixture) user() uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO users (id, email) VALUES (?, ?)", id, id.String()+"@x").Error; err != nil {
		f.t.Fatalf("insert user: %v", err)
	}
	return id
}

func (f *fixture) book(status string, description *string) uuid.UUID {
	f.t.Helper()
	f.n++
	id := uuid.New()
	year := 1990 + f.n
	if err := f.db.Exec("INSERT INTO books (id, title, description, published_year, status, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, "Книга", description, year, status, base.Add(time.Duration(f.n)*time.Minute)).Error; err != nil {
		f.t.Fatalf("insert book: %v", err)
	}
	return id
}

func (f *fixture) upsert(bookID uuid.UUID, status, text string, at time.Time) {
	f.t.Helper()
	if err := f.repo.Upsert(context.Background(), domain.Description{
		BookID: bookID, Status: status, Text: text, Model: "claude-sonnet-5-5", PromptVersion: domain.PromptVersion,
		InputHash: "h", InputTokens: 1000, OutputTokens: 300, GeneratedAt: at,
	}); err != nil {
		f.t.Fatalf("upsert: %v", err)
	}
}

func (f *fixture) get(bookID uuid.UUID) *domain.Item {
	f.t.Helper()
	it, err := f.repo.Get(context.Background(), bookID)
	if err != nil {
		f.t.Fatalf("get: %v", err)
	}
	return it
}

func strp(s string) *string { return &s }

const draftText = "Роман рассказывает о жизни небольшого города."

func TestUpsert_ReplacesAndResetsReview(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	b := f.book("published", nil)
	u := f.user()

	if it := f.get(b); it != nil {
		t.Fatalf("expected no record, got %+v", it)
	}
	f.upsert(b, domain.StatusDraft, draftText, base)
	if err := f.repo.Approve(ctx, b, u, strp("Исправленный текст описания книги."), base.Add(time.Minute)); err != nil {
		t.Fatalf("approve: %v", err)
	}
	it := f.get(b)
	if it.Status != domain.StatusApproved || !it.Edited || it.ReviewedBy == nil || *it.ReviewedBy != u || it.ReviewedAt == nil {
		t.Fatalf("approved record = %+v", it)
	}

	f.upsert(b, domain.StatusDraft, "Новый текст", base.Add(time.Hour))
	it = f.get(b)
	if it.Status != domain.StatusDraft || it.Text != "Новый текст" || it.Edited || it.ReviewedBy != nil || it.ReviewedAt != nil {
		t.Fatalf("regenerated record = %+v", it)
	}
	if !it.GeneratedAt.Equal(base.Add(time.Hour)) {
		t.Errorf("generated_at = %v", it.GeneratedAt)
	}
	var n int64
	f.db.Table("book_ai_descriptions").Count(&n)
	if n != 1 {
		t.Errorf("rows = %d, want 1", n)
	}
	if it.Book.Title != "Книга" || it.Book.Status != "published" || it.Book.PublishedYear == nil {
		t.Errorf("book card = %+v", it.Book)
	}
}

func TestMissingCandidates(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	oldest := f.book("published", nil)
	blank := f.book("published", strp("   "))
	f.book("published", strp("Своё описание"))
	f.book("pending", nil)
	f.book("rejected", nil)
	discarded := f.book("published", nil)
	f.upsert(discarded, domain.StatusDiscarded, "", base)
	unknown := f.book("published", nil)
	f.upsert(unknown, domain.StatusUnknown, "", base)

	ids, err := f.repo.ListMissingCandidateIDs(ctx, 10)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(ids) != 2 || ids[0] != oldest || ids[1] != blank {
		t.Fatalf("candidates = %v, want [%s %s]", ids, oldest, blank)
	}
	if ids, _ := f.repo.ListMissingCandidateIDs(ctx, 1); len(ids) != 1 || ids[0] != oldest {
		t.Errorf("limit 1 = %v", ids)
	}
	if n, err := f.repo.CountMissing(ctx); err != nil || n != 2 {
		t.Errorf("CountMissing = %d (%v), want 2", n, err)
	}
}

func TestApproveDiscard_Conditional(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user()

	if err := f.repo.Approve(ctx, uuid.New(), u, nil, base); !errors.Is(err, ErrNotFound) {
		t.Errorf("approve missing: %v", err)
	}
	if err := f.repo.Discard(ctx, uuid.New(), u, base); !errors.Is(err, ErrNotFound) {
		t.Errorf("discard missing: %v", err)
	}

	failed := f.book("published", nil)
	f.upsert(failed, domain.StatusFailed, "", base)
	if err := f.repo.Approve(ctx, failed, u, nil, base); !errors.Is(err, ErrNotReviewable) {
		t.Errorf("approve failed: %v", err)
	}
	if err := f.repo.Discard(ctx, failed, u, base); !errors.Is(err, ErrNotReviewable) {
		t.Errorf("discard failed: %v", err)
	}

	own := f.book("published", strp("Своё описание"))
	f.upsert(own, domain.StatusDraft, draftText, base)
	if err := f.repo.Approve(ctx, own, u, nil, base); !errors.Is(err, ErrHasOwnDescription) {
		t.Errorf("approve with own description: %v", err)
	}
	if it := f.get(own); it.Status != domain.StatusDraft {
		t.Errorf("draft must stay a draft, got %s", it.Status)
	}

	b := f.book("published", nil)
	f.upsert(b, domain.StatusDraft, draftText, base)
	if err := f.repo.Approve(ctx, b, u, nil, base); err != nil {
		t.Fatalf("approve: %v", err)
	}
	if it := f.get(b); it.Status != domain.StatusApproved || it.Edited || it.Text != draftText {
		t.Errorf("approved as is = %+v", it)
	}
	// Same text: not edited.
	if err := f.repo.Approve(ctx, b, u, strp(draftText), base); err != nil {
		t.Fatalf("re-approve: %v", err)
	}
	if it := f.get(b); it.Edited {
		t.Errorf("same text must not mark edited")
	}
	if err := f.repo.Discard(ctx, b, u, base); err != nil {
		t.Fatalf("discard: %v", err)
	}
	if it := f.get(b); it.Status != domain.StatusDiscarded {
		t.Errorf("status = %s", it.Status)
	}
	if err := f.repo.Discard(ctx, b, u, base); !errors.Is(err, ErrNotReviewable) {
		t.Errorf("discard twice: %v", err)
	}

	var desc *string
	f.db.Raw("SELECT description FROM books WHERE id = ?", b).Scan(&desc)
	if desc != nil {
		t.Errorf("book description changed to %q", *desc)
	}
}

func TestList_SupersededAndCounts(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	a := f.book("published", nil)
	f.upsert(a, domain.StatusDraft, draftText, base)
	sup := f.book("published", nil)
	f.upsert(sup, domain.StatusDraft, draftText, base.Add(time.Minute))
	f.db.Exec("UPDATE books SET description = 'Из Open Library' WHERE id = ?", sup)
	other := f.book("published", nil)
	f.upsert(other, domain.StatusUnknown, "", base)

	items, total, err := f.repo.List(ctx, domain.StatusDraft, 1, 20)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("total=%d len=%d", total, len(items))
	}
	if items[0].BookID != sup || !items[0].Superseded || items[1].BookID != a || items[1].Superseded {
		t.Errorf("order/superseded wrong: %+v", items)
	}
	if page2, _, _ := f.repo.List(ctx, domain.StatusDraft, 2, 1); len(page2) != 1 || page2[0].BookID != a {
		t.Errorf("page 2 = %+v", page2)
	}
	if n, err := f.repo.CountDrafts(ctx); err != nil || n != 1 {
		t.Errorf("CountDrafts = %d (%v), want 1", n, err)
	}
}

func TestApprovedAIDescription(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	u := f.user()
	b := f.book("published", nil)

	if _, _, ok, err := f.repo.ApprovedAIDescription(ctx, b); ok || err != nil {
		t.Errorf("no record: ok=%v err=%v", ok, err)
	}
	f.upsert(b, domain.StatusDraft, draftText, base)
	if _, _, ok, _ := f.repo.ApprovedAIDescription(ctx, b); ok {
		t.Errorf("a draft must not be returned")
	}
	if err := f.repo.Approve(ctx, b, u, nil, base); err != nil {
		t.Fatalf("approve: %v", err)
	}
	text, at, ok, err := f.repo.ApprovedAIDescription(ctx, b)
	if err != nil || !ok || text != draftText || !at.Equal(base) {
		t.Errorf("approved = %q %v %v %v", text, at, ok, err)
	}
}
