package repository

import (
	"context"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// These tests run the real GormRepository SQL for the genre-status public
// filters (docs/features/genre-status.md, "Public read paths") against an
// in-memory SQLite database, reusing catalogStatusDDL from
// author_status_sqlite_test.go.

type genreFixture struct {
	repo      *GormRepository
	published uuid.UUID
	pending   uuid.UUID
	rejected  uuid.UUID
	// source has all three genres; pubOnly has the published genre;
	// pendOnly has the pending genre; mixed has published + pending.
	source   uuid.UUID
	pubOnly  uuid.UUID
	pendOnly uuid.UUID
	mixed    uuid.UUID
}

func newGenreFixture(t *testing.T) *genreFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(catalogStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	f := &genreFixture{
		repo:      &GormRepository{db: db},
		published: uuid.New(), pending: uuid.New(), rejected: uuid.New(),
		source: uuid.New(), pubOnly: uuid.New(), pendOnly: uuid.New(), mixed: uuid.New(),
	}
	now := time.Now()
	for _, g := range []struct {
		id           uuid.UUID
		name, status string
	}{
		{f.published, "published", domain.GenreStatusPublished},
		{f.pending, "pending", domain.GenreStatusPending},
		{f.rejected, "rejected", domain.GenreStatusRejected},
	} {
		if err := db.Exec("INSERT INTO genres (id, name, slug, status, created_at) VALUES (?, ?, ?, ?, ?)",
			g.id, g.name, g.name, g.status, now).Error; err != nil {
			t.Fatalf("insert genre: %v", err)
		}
	}
	books := map[uuid.UUID][]uuid.UUID{
		f.source:   {f.published, f.pending, f.rejected},
		f.pubOnly:  {f.published},
		f.pendOnly: {f.pending},
		f.mixed:    {f.published, f.pending},
	}
	for id, genreIDs := range books {
		if err := db.Exec("INSERT INTO books (id, title, slug, is_hidden, created_at, updated_at) VALUES (?, ?, ?, FALSE, ?, ?)",
			id, "Book", "book-"+id.String(), now, now).Error; err != nil {
			t.Fatalf("insert book: %v", err)
		}
		for _, g := range genreIDs {
			if err := db.Exec("INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", id, g).Error; err != nil {
				t.Fatalf("insert book_genre: %v", err)
			}
		}
	}
	return f
}

func TestListGenres_OnlyPublished(t *testing.T) {
	f := newGenreFixture(t)
	genres, err := f.repo.ListGenres(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(genres) != 1 || genres[0].ID != f.published {
		t.Fatalf("expected only the published genre, got %+v", genres)
	}
}

func TestListBooks_GenreFilterRequiresPublishedGenre(t *testing.T) {
	f := newGenreFixture(t)
	ctx := context.Background()

	for _, id := range []uuid.UUID{f.pending, f.rejected} {
		books, total, err := f.repo.ListBooks(ctx, BookFilter{GenreID: []uuid.UUID{id}})
		if err != nil || total != 0 || len(books) != 0 {
			t.Fatalf("non-published genre: expected empty result, got %d err=%v", total, err)
		}
	}

	// Mixed IDs filter by the published one only: source, pubOnly, mixed
	// (pendOnly is excluded, and no book is duplicated).
	books, total, err := f.repo.ListBooks(ctx, BookFilter{GenreID: []uuid.UUID{f.published, f.pending, f.rejected}})
	if err != nil || total != 3 || len(books) != 3 {
		t.Fatalf("mixed ids: expected 3 books, got %d/%d err=%v", len(books), total, err)
	}
	for _, b := range books {
		if b.ID == f.pendOnly {
			t.Fatalf("book with only a pending genre must be excluded")
		}
	}
}

func TestSimilar_IgnoresNonPublishedGenres(t *testing.T) {
	f := newGenreFixture(t)
	ctx := context.Background()

	src, err := f.repo.GetSimilarSource(ctx, f.source)
	if err != nil {
		t.Fatalf("GetSimilarSource: %v", err)
	}
	if len(src.GenreIDs) != 1 || src.GenreIDs[0] != f.published {
		t.Fatalf("expected only the published source genre, got %v", src.GenreIDs)
	}

	cands, err := f.repo.ListSimilarCandidates(ctx, f.source, *src, 10, 10)
	if err != nil {
		t.Fatalf("ListSimilarCandidates: %v", err)
	}
	got := map[uuid.UUID]SimilarCandidate{}
	for _, c := range cands {
		got[c.BookID] = c
	}
	if len(got) != 2 {
		t.Fatalf("expected pubOnly and mixed as candidates, got %+v", cands)
	}
	if _, ok := got[f.pendOnly]; ok {
		t.Fatalf("pending-only book must not be a genre candidate")
	}
	// mixed has published + pending: only the published one counts.
	if c := got[f.mixed]; c.GenreCount != 1 || c.SharedGenres != 1 {
		t.Fatalf("mixed: expected genre_count=1 shared=1, got %+v", c)
	}
	if c := got[f.pubOnly]; c.GenreCount != 1 || c.SharedGenres != 1 {
		t.Fatalf("pubOnly: expected genre_count=1 shared=1, got %+v", c)
	}
}

func TestGetBooksByIDs_PreloadsAllGenresForMapperFiltering(t *testing.T) {
	// The repository preloads every genre; public filtering happens in the
	// transport mappers via domain.PublishedGenres (one choke point).
	f := newGenreFixture(t)
	books, err := f.repo.GetBooksByIDs(context.Background(), []uuid.UUID{f.source})
	if err != nil || len(books) != 1 {
		t.Fatalf("GetBooksByIDs: %d err=%v", len(books), err)
	}
	if len(books[0].Genres) != 3 {
		t.Fatalf("expected 3 preloaded genres, got %d", len(books[0].Genres))
	}
	if pub := domain.PublishedGenres(books[0].Genres); len(pub) != 1 || pub[0].ID != f.published {
		t.Fatalf("expected PublishedGenres to keep only the published genre, got %+v", pub)
	}
}
