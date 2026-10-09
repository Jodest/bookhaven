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
	updated_at DATETIME
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
