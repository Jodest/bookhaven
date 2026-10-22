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

// Shelves show and accept only published books
// (docs/features/book-status-model.md rule 4). Runs the real SQL against
// SQLite with SQLite-native types.
const shelvesBookStatusDDL = `
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', slug TEXT NOT NULL DEFAULT '', description TEXT, cover_url TEXT, isbn TEXT, published_year INTEGER, page_count INTEGER, language TEXT DEFAULT 'ru', rating REAL DEFAULT 0, ratings_count INTEGER DEFAULT 0, reviews_count INTEGER DEFAULT 0, status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT, created_at DATETIME, updated_at DATETIME, cover_source_url TEXT, cover_path TEXT, cover_status TEXT, cover_error TEXT, cover_attempts INTEGER NOT NULL DEFAULT 0, cover_next_attempt_at DATETIME, cover_stored_at DATETIME, cover_checked_at DATETIME, published_at DATETIME);
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT);
CREATE TABLE genres (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT);
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL);
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL);
CREATE TABLE shelves (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, name TEXT NOT NULL DEFAULT '', type TEXT NOT NULL DEFAULT 'custom', created_at DATETIME, updated_at DATETIME);
CREATE TABLE shelf_items (shelf_id TEXT NOT NULL, book_id TEXT NOT NULL, added_at DATETIME, PRIMARY KEY (shelf_id, book_id));
`

func TestShelvesBookStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(shelvesBookStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	repo := &GormRepository{db: db}
	ctx := context.Background()
	now := time.Now()

	user, shelf := uuid.New(), uuid.New()
	published, pending, rejected := uuid.New(), uuid.New(), uuid.New()
	if err := db.Exec("INSERT INTO shelves (id, user_id, name, type, created_at, updated_at) VALUES (?, ?, 'Читаю', 'reading', ?, ?)", shelf, user, now, now).Error; err != nil {
		t.Fatalf("insert shelf: %v", err)
	}
	for id, status := range map[uuid.UUID]string{
		published: catalogDomain.BookStatusPublished,
		pending:   catalogDomain.BookStatusPending,
		rejected:  catalogDomain.BookStatusRejected,
	} {
		if err := db.Exec("INSERT INTO books (id, title, slug, status, created_at, updated_at) VALUES (?, 'Книга', ?, ?, ?, ?)", id, id.String(), status, now, now).Error; err != nil {
			t.Fatalf("insert book: %v", err)
		}
		if err := db.Exec("INSERT INTO shelf_items (shelf_id, book_id, added_at) VALUES (?, ?, ?)", shelf, id, now).Error; err != nil {
			t.Fatalf("insert shelf item: %v", err)
		}
	}

	if ok, err := repo.BookExists(ctx, published); err != nil || !ok {
		t.Fatalf("published book must exist: %v %v", ok, err)
	}
	for _, id := range []uuid.UUID{pending, rejected, uuid.New()} {
		if ok, err := repo.BookExists(ctx, id); err != nil || ok {
			t.Fatalf("non-published/unknown book must not exist: %v %v", ok, err)
		}
	}

	books, total, err := repo.ListBooks(ctx, shelf, 20, 0)
	if err != nil || total != 1 || len(books) != 1 || books[0].ID != published {
		t.Fatalf("shelf books: total=%d books=%+v err=%v", total, books, err)
	}

	shelves, err := repo.ListByUserVisible(ctx, user)
	if err != nil || len(shelves) != 1 || shelves[0].BooksCount != 1 {
		t.Fatalf("visible shelf count: %+v err=%v", shelves, err)
	}
}
