package repository

import (
	"context"
	"errors"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// withinTransactionDDL recreates the tables an import row writes through
// WithinTransaction (authors, books and the book link tables), with
// SQLite-native types.
const withinTransactionDDL = `
CREATE TABLE authors (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	biography TEXT,
	photo_url TEXT,
	birth_date DATE,
	death_date DATE,
	status TEXT NOT NULL DEFAULT 'published',
	external_source TEXT,
	external_id TEXT,
	photo_author TEXT,
	photo_license TEXT,
	photo_license_url TEXT,
	photo_source_url TEXT,
	created_at DATETIME,
	updated_at DATETIME
);
CREATE TABLE books (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL DEFAULT '',
	slug TEXT NOT NULL UNIQUE,
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
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL, PRIMARY KEY (book_id, author_id));
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL, PRIMARY KEY (book_id, genre_id));
`

func newWithinTransactionRepo(t *testing.T) (*GormRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(withinTransactionDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &GormRepository{db: db}, db
}

func countRows(t *testing.T, db *gorm.DB, query string, args ...interface{}) int64 {
	t.Helper()
	var n int64
	if err := db.Raw(query, args...).Scan(&n).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	return n
}

func TestWithinTransaction_RollsBackOnError(t *testing.T) {
	repo, db := newWithinTransactionRepo(t)
	ctx := context.Background()
	author := &catalogDomain.Author{ID: uuid.New(), Name: "Иван Новиков", Slug: "ivan-novikov", Status: catalogDomain.AuthorStatusPublished}
	sentinel := errors.New("row failed")

	err := repo.WithinTransaction(ctx, func(tx Repository) error {
		if err := tx.CreateAuthor(ctx, author); err != nil {
			t.Fatalf("create author: %v", err)
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("expected fn's error, got %v", err)
	}
	if n := countRows(t, db, "SELECT COUNT(*) FROM authors WHERE id = ?", author.ID); n != 0 {
		t.Fatalf("author must be rolled back, found %d", n)
	}
}

func TestWithinTransaction_CommitsWithNestedCreateBook(t *testing.T) {
	repo, db := newWithinTransactionRepo(t)
	ctx := context.Background()
	author := &catalogDomain.Author{ID: uuid.New(), Name: "Иван Новиков", Slug: "ivan-novikov", Status: catalogDomain.AuthorStatusPublished}
	book := &catalogDomain.Book{ID: uuid.New(), Title: "Книга", Slug: "kniga", Language: "ru", Status: catalogDomain.BookStatusPublished}

	err := repo.WithinTransaction(ctx, func(tx Repository) error {
		if err := tx.CreateAuthor(ctx, author); err != nil {
			return err
		}
		return tx.CreateBook(ctx, book, []uuid.UUID{author.ID}, nil)
	})
	if err != nil {
		t.Fatalf("WithinTransaction: %v", err)
	}
	if n := countRows(t, db, "SELECT COUNT(*) FROM authors WHERE id = ?", author.ID); n != 1 {
		t.Fatalf("author must be committed, found %d", n)
	}
	if n := countRows(t, db, "SELECT COUNT(*) FROM books WHERE id = ?", book.ID); n != 1 {
		t.Fatalf("book must be committed, found %d", n)
	}
	if n := countRows(t, db, "SELECT COUNT(*) FROM book_authors WHERE book_id = ? AND author_id = ?", book.ID, author.ID); n != 1 {
		t.Fatalf("book-author link must be committed, found %d", n)
	}
}
