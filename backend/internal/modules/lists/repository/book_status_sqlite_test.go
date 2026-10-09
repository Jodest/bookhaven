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

// Lists show and accept only published books
// (docs/features/book-status-model.md rule 4). Runs the real SQL against
// SQLite with SQLite-native types.
const listsBookStatusDDL = `
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', slug TEXT NOT NULL DEFAULT '', description TEXT, cover_url TEXT, isbn TEXT, published_year INTEGER, page_count INTEGER, language TEXT DEFAULT 'ru', rating REAL DEFAULT 0, ratings_count INTEGER DEFAULT 0, reviews_count INTEGER DEFAULT 0, status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT, created_at DATETIME, updated_at DATETIME);
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT);
CREATE TABLE genres (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT);
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL);
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL);
CREATE TABLE list_items (list_id TEXT NOT NULL, book_id TEXT NOT NULL, position INTEGER NOT NULL DEFAULT 0, added_at DATETIME, PRIMARY KEY (list_id, book_id));
`

func TestListsBookStatus(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(listsBookStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	repo := &GormRepository{db: db}
	ctx := context.Background()
	now := time.Now()

	list := uuid.New()
	published, pending, rejected := uuid.New(), uuid.New(), uuid.New()
	pos := 0
	for id, status := range map[uuid.UUID]string{
		published: catalogDomain.BookStatusPublished,
		pending:   catalogDomain.BookStatusPending,
		rejected:  catalogDomain.BookStatusRejected,
	} {
		pos++
		if err := db.Exec("INSERT INTO books (id, title, slug, status, created_at, updated_at) VALUES (?, 'Книга', ?, ?, ?, ?)", id, id.String(), status, now, now).Error; err != nil {
			t.Fatalf("insert book: %v", err)
		}
		if err := db.Exec("INSERT INTO list_items (list_id, book_id, position, added_at) VALUES (?, ?, ?, ?)", list, id, pos, now).Error; err != nil {
			t.Fatalf("insert list item: %v", err)
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

	books, total, err := repo.ListBooks(ctx, list, 20, 0)
	if err != nil || total != 1 || len(books) != 1 || books[0].ID != published {
		t.Fatalf("list books: total=%d books=%+v err=%v", total, books, err)
	}
}
