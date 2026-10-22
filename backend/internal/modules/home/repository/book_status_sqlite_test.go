package repository

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Home blocks show only published books (docs/features/book-status-model.md
// rule 4) while review hiding stays unchanged. Runs the real SQL against
// SQLite with SQLite-native types; Postgres-only queries (DISTINCT ON,
// EXTRACT) are covered by SQL-text checks.
const homeBookStatusDDL = `
CREATE TABLE books (
	id TEXT PRIMARY KEY,
	cover_url TEXT,
	rating REAL DEFAULT 0,
	ratings_count INTEGER DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'pending',
	created_at DATETIME,
	cover_source_url TEXT,
	cover_path TEXT,
	cover_status TEXT,
	cover_error TEXT,
	cover_attempts INTEGER NOT NULL DEFAULT 0,
	cover_next_attempt_at DATETIME,
	cover_stored_at DATETIME,
	cover_checked_at DATETIME,
	published_at DATETIME
);
CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT, avatar_url TEXT);
CREATE TABLE reviews (
	id TEXT PRIMARY KEY,
	book_id TEXT NOT NULL,
	user_id TEXT NOT NULL,
	rating INTEGER,
	content TEXT,
	likes_count INTEGER DEFAULT 0,
	is_hidden BOOLEAN NOT NULL DEFAULT FALSE,
	created_at DATETIME
);
CREATE TABLE shelves (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, type TEXT NOT NULL);
CREATE TABLE shelf_items (shelf_id TEXT NOT NULL, book_id TEXT NOT NULL, added_at DATETIME);
`

type homeStatusFixture struct {
	repo                         *GormRepository
	db                           *gorm.DB
	published, pending, rejected uuid.UUID
}

func newHomeStatusFixture(t *testing.T) *homeStatusFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(homeBookStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	f := &homeStatusFixture{repo: &GormRepository{db: db}, db: db, published: uuid.New(), pending: uuid.New(), rejected: uuid.New()}
	now := time.Now()
	for _, b := range []struct {
		id     uuid.UUID
		status string
	}{{f.published, "published"}, {f.pending, "pending"}, {f.rejected, "rejected"}} {
		if err := db.Exec("INSERT INTO books (id, cover_url, rating, ratings_count, status, created_at) VALUES (?, 'c.jpg', 5, 10, ?, ?)",
			b.id, b.status, now).Error; err != nil {
			t.Fatalf("insert book: %v", err)
		}
	}
	return f
}

func onlyPublished(t *testing.T, name string, got []uuid.UUID, want uuid.UUID) {
	t.Helper()
	if len(got) != 1 || got[0] != want {
		t.Fatalf("%s: expected only the published book, got %v", name, got)
	}
}

func TestHomeBookStatus_PoolNewTopRated(t *testing.T) {
	f := newHomeStatusFixture(t)
	ctx := context.Background()

	pool, err := f.repo.ListPoolIDs(ctx, PoolFilter{RequireCover: true, MinRating: 1})
	if err != nil {
		t.Fatalf("pool: %v", err)
	}
	onlyPublished(t, "pool", pool, f.published)

	for i := 0; i < 5; i++ {
		got, err := f.repo.PickRandomBookID(ctx, PoolFilter{})
		if err != nil || got == nil || *got != f.published {
			t.Fatalf("random: expected the published book, got %v err=%v", got, err)
		}
	}

	newIDs, err := f.repo.ListNewBookIDs(ctx, 10)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	onlyPublished(t, "new", newIDs, f.published)

	top, err := f.repo.ListTopRatedBookIDs(ctx, 1, 1, 10)
	if err != nil {
		t.Fatalf("top: %v", err)
	}
	onlyPublished(t, "top rated", top, f.published)
}

// TestHomeNewBooks_OrderByPublishedAt: «Новое в каталоге» orders by the
// first-publication time (#245), not by creation time; books without one
// (raw-SQL fixtures) go last.
func TestHomeNewBooks_OrderByPublishedAt(t *testing.T) {
	f := newHomeStatusFixture(t)
	ctx := context.Background()
	now := time.Now()
	oldHarvest, recentCreate := uuid.New(), uuid.New()
	for _, b := range []struct {
		id                   uuid.UUID
		createdAt, published time.Time
	}{
		// Harvested a month ago, published today: ranks first.
		{oldHarvest, now.AddDate(0, -1, 0), now.Add(-time.Minute)},
		// Created and published yesterday.
		{recentCreate, now.AddDate(0, 0, -1), now.AddDate(0, 0, -1)},
	} {
		if err := f.db.Exec("INSERT INTO books (id, cover_url, rating, ratings_count, status, created_at, published_at) VALUES (?, 'c.jpg', 5, 10, 'published', ?, ?)",
			b.id, b.createdAt, b.published).Error; err != nil {
			t.Fatalf("insert book: %v", err)
		}
	}

	got, err := f.repo.ListNewBookIDs(ctx, 10)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	// f.published has no published_at (raw-SQL fixture) and a newer
	// created_at, yet goes last (NULLS LAST).
	want := []uuid.UUID{oldHarvest, recentCreate, f.published}
	if len(got) != len(want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("position %d: expected %v, got %v (all: %v)", i, want[i], got[i], got)
		}
	}
}

func TestHomeBookStatus_ReviewsByIDs(t *testing.T) {
	f := newHomeStatusFixture(t)
	user := uuid.New()
	if err := f.db.Exec("INSERT INTO users (id, name) VALUES (?, 'u')", user).Error; err != nil {
		t.Fatalf("insert user: %v", err)
	}
	visible, hiddenReview, onPending, onRejected := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	for _, r := range []struct {
		id     uuid.UUID
		book   uuid.UUID
		hidden bool
	}{
		{visible, f.published, false},
		{hiddenReview, f.published, true},
		{onPending, f.pending, false},
		{onRejected, f.rejected, false},
	} {
		if err := f.db.Exec("INSERT INTO reviews (id, book_id, user_id, rating, content, is_hidden, created_at) VALUES (?, ?, ?, 5, 'text', ?, ?)",
			r.id, r.book, user, r.hidden, time.Now()).Error; err != nil {
			t.Fatalf("insert review: %v", err)
		}
	}

	rows, err := f.repo.GetReviewsByIDs(context.Background(), []uuid.UUID{visible, hiddenReview, onPending, onRejected})
	if err != nil {
		t.Fatalf("reviews: %v", err)
	}
	// A hidden review on a published book stays hidden; reviews of
	// non-published books are excluded.
	if len(rows) != 1 || rows[0].ID != visible {
		t.Fatalf("expected only the visible review of the published book, got %+v", rows)
	}
}

func TestHomeBookStatus_ShelfBooks(t *testing.T) {
	f := newHomeStatusFixture(t)
	ctx := context.Background()
	user, shelf := uuid.New(), uuid.New()
	if err := f.db.Exec("INSERT INTO shelves (id, user_id, type) VALUES (?, ?, 'reading')", shelf, user).Error; err != nil {
		t.Fatalf("insert shelf: %v", err)
	}
	for _, b := range []uuid.UUID{f.published, f.pending, f.rejected} {
		if err := f.db.Exec("INSERT INTO shelf_items (shelf_id, book_id, added_at) VALUES (?, ?, ?)", shelf, b, time.Now()).Error; err != nil {
			t.Fatalf("insert shelf item: %v", err)
		}
	}

	got, err := f.repo.ListShelfBookIDs(ctx, shelf, 10)
	if err != nil {
		t.Fatalf("shelf books: %v", err)
	}
	onlyPublished(t, "continue reading", got, f.published)

	ref, err := f.repo.GetSystemShelf(ctx, user, "reading")
	if err != nil || ref == nil || ref.Total != 1 {
		t.Fatalf("system shelf total: %+v err=%v", ref, err)
	}
}

// TestHomeBookStatus_PostgresOnlyQueries checks the SQL text of the queries
// SQLite cannot run: the book predicate is status-based and the review
// predicate is unchanged.
func TestHomeBookStatus_PostgresOnlyQueries(t *testing.T) {
	for name, q := range map[string]string{
		"recent reviews":  recentReviewsQuery,
		"calendar events": calendarEventsQuery,
		"anniversary":     anniversaryQuery,
		"reviews by ids":  reviewsByIDsQuery,
		"seeds":           recommendationSeedsQuery,
	} {
		if strings.Contains(q, "b.is_hidden") || strings.Contains(q, "books.is_hidden") || strings.Contains(q, "WHERE is_hidden") {
			t.Fatalf("%s: book is_hidden predicate must be gone:\n%s", name, q)
		}
		if !strings.Contains(q, "status = 'published'") {
			t.Fatalf("%s: expected a status = 'published' book predicate:\n%s", name, q)
		}
	}
	for _, q := range []string{recentReviewsQuery, reviewsByIDsQuery} {
		if !strings.Contains(q, "r.is_hidden = FALSE") {
			t.Fatalf("review hiding predicate must stay:\n%s", q)
		}
	}
}

// A book whose cover copy is "absent" for its current link (the source has
// no image) is not in the cover-required pools; a stale absent state for an
// older link does not exclude it (#489).
func TestHomePool_AbsentCoverExcludedFromCoverPool(t *testing.T) {
	f := newHomeStatusFixture(t)
	ctx := context.Background()
	if err := f.db.Exec("UPDATE books SET cover_source_url = cover_url, cover_status = 'absent' WHERE id = ?", f.published).Error; err != nil {
		t.Fatal(err)
	}
	pool, err := f.repo.ListPoolIDs(ctx, PoolFilter{RequireCover: true, MinRating: 1})
	if err != nil || len(pool) != 0 {
		t.Fatalf("absent cover must leave the cover pool: %v err=%v", pool, err)
	}
	withoutCover, err := f.repo.ListPoolIDs(ctx, PoolFilter{MinRating: 1})
	if err != nil || len(withoutCover) != 1 {
		t.Fatalf("pool without the cover requirement keeps the book: %v err=%v", withoutCover, err)
	}
	if err := f.db.Exec("UPDATE books SET cover_source_url = 'https://old.example/x.jpg' WHERE id = ?", f.published).Error; err != nil {
		t.Fatal(err)
	}
	pool, err = f.repo.ListPoolIDs(ctx, PoolFilter{RequireCover: true, MinRating: 1})
	if err != nil || len(pool) != 1 {
		t.Fatalf("stale absent state must not exclude: %v err=%v", pool, err)
	}
}
