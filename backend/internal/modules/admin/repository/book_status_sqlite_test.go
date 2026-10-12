package repository

import (
	"context"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// bookStatusDDL recreates the tables the admin book list, SetBookStatus and
// DashboardCounts touch, with SQLite-native types
// (docs/features/book-status-model.md).
const bookStatusDDL = `
CREATE TABLE books (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL DEFAULT '',
	slug TEXT NOT NULL DEFAULT '',
	description TEXT,
	cover_url TEXT,
	isbn TEXT,
	published_year INTEGER,
	page_count INTEGER,
	language TEXT DEFAULT 'ru',
	rating REAL DEFAULT 0,
	ratings_count INTEGER DEFAULT 0,
	reviews_count INTEGER DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'pending',
	external_source TEXT,
	external_id TEXT,
	created_at DATETIME,
	updated_at DATETIME,
	published_at DATETIME
);
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', slug TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'published');
CREATE TABLE genres (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', slug TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', merged_into_id TEXT);
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL, PRIMARY KEY (book_id, author_id));
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL, PRIMARY KEY (book_id, genre_id));
CREATE TABLE reviews (id TEXT PRIMARY KEY, is_hidden BOOLEAN NOT NULL DEFAULT 0, deleted_at DATETIME);
`

func newBookStatusRepo(t *testing.T) (*GormRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(bookStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &GormRepository{db: db}, db
}

func insertStatusBook(t *testing.T, db *gorm.DB, status string, source *string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	old := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := db.Exec("INSERT INTO books (id, title, slug, status, external_source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		id, "Книга", id.String(), status, source, old, old).Error; err != nil {
		t.Fatalf("insert book: %v", err)
	}
	return id
}

func TestAdminListBooks_StatusFilter(t *testing.T) {
	repo, db := newBookStatusRepo(t)
	ctx := context.Background()
	src := "wikidata"
	published := insertStatusBook(t, db, catalogDomain.BookStatusPublished, nil)
	pendingManual := insertStatusBook(t, db, catalogDomain.BookStatusPending, nil)
	pendingHarvested := insertStatusBook(t, db, catalogDomain.BookStatusPending, &src)
	rejected := insertStatusBook(t, db, catalogDomain.BookStatusRejected, nil)

	ids := func(books []catalogDomain.Book) map[uuid.UUID]bool {
		out := map[uuid.UUID]bool{}
		for _, b := range books {
			out[b.ID] = true
		}
		return out
	}

	all, total, err := repo.ListBooks(ctx, BookFilter{})
	if err != nil || total != 4 || len(all) != 4 {
		t.Fatalf("all: total=%d len=%d err=%v", total, len(all), err)
	}

	pending, total, err := repo.ListBooks(ctx, BookFilter{Status: catalogDomain.BookStatusPending})
	got := ids(pending)
	if err != nil || total != 2 || !got[pendingManual] || !got[pendingHarvested] {
		t.Fatalf("pending (manual + harvested): total=%d got=%v err=%v", total, got, err)
	}
	for _, b := range pending {
		if b.Status != catalogDomain.BookStatusPending {
			t.Fatalf("expected status pending, got %q", b.Status)
		}
	}

	harvested, total, err := repo.ListBooks(ctx, BookFilter{Status: catalogDomain.BookStatusPending, Source: "harvested"})
	if err != nil || total != 1 || len(harvested) != 1 || harvested[0].ID != pendingHarvested {
		t.Fatalf("pending+harvested: total=%d err=%v", total, err)
	}

	pub, total, err := repo.ListBooks(ctx, BookFilter{Status: catalogDomain.BookStatusPublished})
	if err != nil || total != 1 || pub[0].ID != published {
		t.Fatalf("published: total=%d err=%v", total, err)
	}
	rej, total, err := repo.ListBooks(ctx, BookFilter{Status: catalogDomain.BookStatusRejected})
	if err != nil || total != 1 || rej[0].ID != rejected {
		t.Fatalf("rejected: total=%d err=%v", total, err)
	}
}

func TestSetBookStatus_FoundAndNotFound(t *testing.T) {
	repo, db := newBookStatusRepo(t)
	ctx := context.Background()
	id := insertStatusBook(t, db, catalogDomain.BookStatusPending, nil)

	found, err := repo.SetBookStatus(ctx, id, catalogDomain.BookStatusPublished)
	if err != nil || !found {
		t.Fatalf("expected found, got found=%v err=%v", found, err)
	}
	var row struct {
		Title     string
		Status    string
		UpdatedAt time.Time
	}
	if err := db.Raw("SELECT title, status, updated_at FROM books WHERE id = ?", id).Scan(&row).Error; err != nil {
		t.Fatalf("select: %v", err)
	}
	if row.Status != catalogDomain.BookStatusPublished || row.Title != "Книга" {
		t.Fatalf("unexpected row after update: %+v", row)
	}
	if !row.UpdatedAt.After(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected updated_at to be bumped, got %v", row.UpdatedAt)
	}

	found, err = repo.SetBookStatus(ctx, uuid.New(), catalogDomain.BookStatusPublished)
	if err != nil || found {
		t.Fatalf("expected not found, got found=%v err=%v", found, err)
	}
}

func TestDashboardCounts_PendingAndRejectedBooks(t *testing.T) {
	repo, db := newBookStatusRepo(t)
	src := "wikidata"
	insertStatusBook(t, db, catalogDomain.BookStatusPublished, nil)
	insertStatusBook(t, db, catalogDomain.BookStatusPending, nil)
	insertStatusBook(t, db, catalogDomain.BookStatusPending, &src)
	insertStatusBook(t, db, catalogDomain.BookStatusRejected, nil)

	stats, err := repo.DashboardCounts(context.Background())
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if stats.Books != 4 || stats.PendingBooks != 2 || stats.RejectedBooks != 1 {
		t.Fatalf("expected books=4 pending=2 rejected=1, got %+v", stats)
	}
}

// readPublishedAt returns the stored first-publication time of a book.
func readPublishedAt(t *testing.T, db *gorm.DB, id uuid.UUID) *time.Time {
	t.Helper()
	var row struct{ PublishedAt *time.Time }
	if err := db.Raw("SELECT published_at FROM books WHERE id = ?", id).Scan(&row).Error; err != nil {
		t.Fatalf("select published_at: %v", err)
	}
	return row.PublishedAt
}

func setPublishedAt(t *testing.T, db *gorm.DB, id uuid.UUID, at time.Time) {
	t.Helper()
	if err := db.Exec("UPDATE books SET published_at = ? WHERE id = ?", at, id).Error; err != nil {
		t.Fatalf("set published_at: %v", err)
	}
}

// TestUpdateBook_PublishedAt: the first publish through UpdateBook sets the
// time; unpublish/republish and edits carrying a stale or nil PublishedAt
// keep it (#245).
func TestUpdateBook_PublishedAt(t *testing.T) {
	repo, db := newBookStatusRepo(t)
	ctx := context.Background()
	id := insertStatusBook(t, db, catalogDomain.BookStatusPending, nil)

	load := func() *catalogDomain.Book {
		t.Helper()
		var b catalogDomain.Book
		if err := db.First(&b, "id = ?", id).Error; err != nil {
			t.Fatalf("load book: %v", err)
		}
		return &b
	}
	update := func(b *catalogDomain.Book) {
		t.Helper()
		if err := repo.UpdateBook(ctx, b, nil, nil); err != nil {
			t.Fatalf("update book: %v", err)
		}
	}

	// An edit of a pending book leaves published_at empty.
	b := load()
	b.Title = "Правка"
	update(b)
	if got := readPublishedAt(t, db, id); got != nil {
		t.Fatalf("pending edit must not set published_at, got %v", got)
	}

	// First publish sets it.
	before := time.Now().Add(-time.Second)
	b = load()
	b.Status = catalogDomain.BookStatusPublished
	update(b)
	first := readPublishedAt(t, db, id)
	if first == nil || first.Before(before) {
		t.Fatalf("first publish must set published_at to now, got %v", first)
	}

	// Unpublish keeps it.
	b = load()
	b.Status = catalogDomain.BookStatusRejected
	update(b)
	if got := readPublishedAt(t, db, id); got == nil || !got.Equal(*first) {
		t.Fatalf("unpublish must keep published_at %v, got %v", first, got)
	}

	// Republish keeps the original.
	b = load()
	b.Status = catalogDomain.BookStatusPublished
	update(b)
	if got := readPublishedAt(t, db, id); got == nil || !got.Equal(*first) {
		t.Fatalf("republish must keep published_at %v, got %v", first, got)
	}

	// An edit carrying a nil or stale PublishedAt (struct loaded before a
	// concurrent publish, or a client-sent value) does not overwrite it.
	stale := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	for _, v := range []*time.Time{nil, &stale} {
		b = load()
		b.Title = "Ещё правка"
		b.PublishedAt = v
		update(b)
		if got := readPublishedAt(t, db, id); got == nil || !got.Equal(*first) {
			t.Fatalf("edit with PublishedAt=%v must keep %v, got %v", v, first, got)
		}
	}
}

// TestSetBookStatus_PublishedAt covers the bulk-status scenario: a bulk
// publish sets the time for a never-published book, keeps it for a
// previously published one, and a bulk reject keeps it (#245).
func TestSetBookStatus_PublishedAt(t *testing.T) {
	repo, db := newBookStatusRepo(t)
	ctx := context.Background()
	never := insertStatusBook(t, db, catalogDomain.BookStatusPending, nil)
	before := insertStatusBook(t, db, catalogDomain.BookStatusPending, nil)
	earlier := time.Date(2025, 6, 1, 12, 0, 0, 0, time.UTC)
	setPublishedAt(t, db, before, earlier)

	start := time.Now().Add(-time.Second)
	for _, id := range []uuid.UUID{never, before} {
		if found, err := repo.SetBookStatus(ctx, id, catalogDomain.BookStatusPublished); err != nil || !found {
			t.Fatalf("publish: found=%v err=%v", found, err)
		}
	}
	first := readPublishedAt(t, db, never)
	if first == nil || first.Before(start) {
		t.Fatalf("never-published book must get now, got %v", first)
	}
	if got := readPublishedAt(t, db, before); got == nil || !got.Equal(earlier) {
		t.Fatalf("previously published book must keep %v, got %v", earlier, got)
	}

	for _, id := range []uuid.UUID{never, before} {
		if found, err := repo.SetBookStatus(ctx, id, catalogDomain.BookStatusRejected); err != nil || !found {
			t.Fatalf("reject: found=%v err=%v", found, err)
		}
	}
	if got := readPublishedAt(t, db, never); got == nil || !got.Equal(*first) {
		t.Fatalf("reject must keep %v, got %v", first, got)
	}
	if got := readPublishedAt(t, db, before); got == nil || !got.Equal(earlier) {
		t.Fatalf("reject must keep %v, got %v", earlier, got)
	}
}
