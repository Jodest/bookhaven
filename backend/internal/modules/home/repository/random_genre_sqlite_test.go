package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// The random-book pool genre filter only counts a published genre
// (docs/features/genre-status.md): run the real poolQuery SQL against an
// in-memory SQLite database with SQLite-native types.
const randomGenreDDL = `
CREATE TABLE books (
	id TEXT PRIMARY KEY,
	cover_url TEXT,
	rating REAL DEFAULT 0,
	ratings_count INTEGER DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'pending'
);
CREATE TABLE genres (id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'pending');
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL, PRIMARY KEY (book_id, genre_id));
`

func TestPickRandomBookID_GenreFilterRequiresPublishedGenre(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(randomGenreDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	repo := &GormRepository{db: db}

	book := uuid.New()
	genres := map[string]uuid.UUID{"published": uuid.New(), "pending": uuid.New(), "rejected": uuid.New()}
	if err := db.Exec("INSERT INTO books (id, status) VALUES (?, 'published')", book).Error; err != nil {
		t.Fatalf("insert book: %v", err)
	}
	for status, id := range genres {
		if err := db.Exec("INSERT INTO genres (id, status) VALUES (?, ?)", id, status).Error; err != nil {
			t.Fatalf("insert genre: %v", err)
		}
		if err := db.Exec("INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", book, id).Error; err != nil {
			t.Fatalf("insert book_genre: %v", err)
		}
	}

	ctx := context.Background()
	pub := genres["published"]
	got, err := repo.PickRandomBookID(ctx, PoolFilter{GenreID: &pub})
	if err != nil || got == nil || *got != book {
		t.Fatalf("published genre: expected the book, got %v err=%v", got, err)
	}
	for _, status := range []string{"pending", "rejected"} {
		id := genres[status]
		got, err := repo.PickRandomBookID(ctx, PoolFilter{GenreID: &id})
		if err != nil || got != nil {
			t.Fatalf("%s genre: expected no book, got %v err=%v", status, got, err)
		}
	}
}
