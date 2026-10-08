package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// These tests run the real GormRepository SQL for the author-status public
// filters (docs/features/author-status.md, "Public read paths") against an
// in-memory SQLite database. The production schema is Postgres-only
// (uuid columns, gen_random_uuid()), so the tables are recreated here with
// SQLite-native types - same column names, same status default. Queries
// that rely on Postgres-only functions (full-text search) are not covered.
const catalogStatusDDL = `
CREATE TABLE authors (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	biography TEXT,
	photo_url TEXT,
	birth_date DATETIME,
	death_date DATETIME,
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
	title TEXT NOT NULL,
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
	is_hidden BOOLEAN DEFAULT FALSE,
	external_source TEXT,
	external_id TEXT,
	created_at DATETIME,
	updated_at DATETIME
);
CREATE TABLE genres (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	status TEXT NOT NULL DEFAULT 'pending',
	wikidata_qid TEXT,
	merged_into_id TEXT REFERENCES genres(id),
	created_at DATETIME
);
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL, PRIMARY KEY (book_id, author_id));
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL, PRIMARY KEY (book_id, genre_id));
`

type statusFixture struct {
	db        *gorm.DB
	repo      *GormRepository
	published uuid.UUID
	pending   uuid.UUID
	rejected  uuid.UUID
	// source is co-authored by published+pending; pendingOnly shares only
	// the pending author with source; publishedOnly shares the published
	// author.
	source        uuid.UUID
	pendingOnly   uuid.UUID
	publishedOnly uuid.UUID
}

func newStatusFixture(t *testing.T) *statusFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(catalogStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}

	f := &statusFixture{
		db:            db,
		repo:          &GormRepository{db: db},
		published:     uuid.New(),
		pending:       uuid.New(),
		rejected:      uuid.New(),
		source:        uuid.New(),
		pendingOnly:   uuid.New(),
		publishedOnly: uuid.New(),
	}
	now := time.Now()
	authors := []domain.Author{
		{ID: f.published, Name: "Published", Slug: "published", Status: domain.AuthorStatusPublished},
		{ID: f.pending, Name: "Pending", Slug: "pending", Status: domain.AuthorStatusPending},
		{ID: f.rejected, Name: "Rejected", Slug: "rejected", Status: domain.AuthorStatusRejected},
	}
	for _, a := range authors {
		if err := db.Exec("INSERT INTO authors (id, name, slug, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			a.ID, a.Name, a.Slug, a.Status, now, now).Error; err != nil {
			t.Fatalf("insert author: %v", err)
		}
	}
	books := map[uuid.UUID][]uuid.UUID{
		f.source:        {f.published, f.pending},
		f.pendingOnly:   {f.pending},
		f.publishedOnly: {f.published},
	}
	i := 0
	for id, authorIDs := range books {
		i++
		if err := db.Exec("INSERT INTO books (id, title, slug, is_hidden, created_at, updated_at) VALUES (?, ?, ?, FALSE, ?, ?)",
			id, "Book", "book-"+id.String(), now, now).Error; err != nil {
			t.Fatalf("insert book: %v", err)
		}
		for _, a := range authorIDs {
			if err := db.Exec("INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", id, a).Error; err != nil {
				t.Fatalf("insert book_author: %v", err)
			}
		}
	}
	return f
}

func TestGetAuthorBySlug_OnlyPublished(t *testing.T) {
	f := newStatusFixture(t)
	ctx := context.Background()

	if a, err := f.repo.GetAuthorBySlug(ctx, "published"); err != nil || a.ID != f.published {
		t.Fatalf("expected published author, got %+v err=%v", a, err)
	}
	for _, slug := range []string{"pending", "rejected"} {
		if _, err := f.repo.GetAuthorBySlug(ctx, slug); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("%s: expected ErrRecordNotFound, got %v", slug, err)
		}
	}
}

func TestGetAuthorBySlug_CoAuthorsPreloadedForNameOnlyMapping(t *testing.T) {
	f := newStatusFixture(t)
	a, err := f.repo.GetAuthorBySlug(context.Background(), "published")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, b := range a.Books {
		for _, co := range b.Authors {
			if co.ID == f.pending && co.Status != domain.AuthorStatusPending {
				t.Fatalf("expected co-author status loaded for mapping, got %q", co.Status)
			}
		}
	}
}

func TestListAuthors_OnlyPublished(t *testing.T) {
	f := newStatusFixture(t)
	authors, total, err := f.repo.ListAuthors(context.Background(), "", 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 || len(authors) != 1 || authors[0].ID != f.published {
		t.Fatalf("expected only the published author (total 1), got total=%d %+v", total, authors)
	}
}

func TestListBooks_AuthorFilterRequiresPublishedAuthor(t *testing.T) {
	f := newStatusFixture(t)
	ctx := context.Background()

	_, total, err := f.repo.ListBooks(ctx, BookFilter{AuthorID: &f.published})
	if err != nil || total != 2 {
		t.Fatalf("published author: expected 2 books, got %d err=%v", total, err)
	}
	for _, id := range []uuid.UUID{f.pending, f.rejected} {
		books, total, err := f.repo.ListBooks(ctx, BookFilter{AuthorID: &id})
		if err != nil || total != 0 || len(books) != 0 {
			t.Fatalf("non-published author: expected empty result, got %d err=%v", total, err)
		}
	}
}

func TestSimilar_IgnoresNonPublishedSharedAuthors(t *testing.T) {
	f := newStatusFixture(t)
	ctx := context.Background()

	src, err := f.repo.GetSimilarSource(ctx, f.source)
	if err != nil {
		t.Fatalf("GetSimilarSource: %v", err)
	}
	if len(src.AuthorIDs) != 1 || src.AuthorIDs[0] != f.published {
		t.Fatalf("expected only the published source author, got %v", src.AuthorIDs)
	}

	// Even if a non-published ID reaches the candidate query, the
	// per-candidate author IDs are filtered to published authors.
	cands, err := f.repo.ListSimilarCandidates(ctx, f.source, *src, 10, 10)
	if err != nil {
		t.Fatalf("ListSimilarCandidates: %v", err)
	}
	if len(cands) != 1 || cands[0].BookID != f.publishedOnly {
		t.Fatalf("expected only the published-author candidate, got %+v", cands)
	}
	for _, id := range cands[0].AuthorIDs {
		if id != f.published {
			t.Fatalf("expected only published candidate author IDs, got %v", cands[0].AuthorIDs)
		}
	}
}

func TestListBookAuthorIDs_OnlyPublished(t *testing.T) {
	f := newStatusFixture(t)
	links, err := f.repo.ListBookAuthorIDs(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := links[f.pendingOnly]; ok {
		t.Fatalf("book with only a pending author must have no author links, got %v", links[f.pendingOnly])
	}
	if got := links[f.source]; len(got) != 1 || got[0] != f.published {
		t.Fatalf("expected source linked only to the published author, got %v", got)
	}
}
