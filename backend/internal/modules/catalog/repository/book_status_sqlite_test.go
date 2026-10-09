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

// Book-status public filters (docs/features/book-status-model.md rule 4):
// only status = 'published' books are visible on any public read path.
// Runs the real SQL against SQLite (catalogStatusDDL plus the tables the
// similarity interactions query joins).
const bookStatusExtraDDL = `
CREATE TABLE users (id TEXT PRIMARY KEY, email_verified_at DATETIME);
CREATE TABLE shelves (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, type TEXT NOT NULL);
CREATE TABLE shelf_items (shelf_id TEXT NOT NULL, book_id TEXT NOT NULL);
CREATE TABLE ratings (user_id TEXT NOT NULL, book_id TEXT NOT NULL, value INTEGER NOT NULL);
`

type bookStatusFixture struct {
	repo      *GormRepository
	db        *gorm.DB
	author    uuid.UUID
	published uuid.UUID
	other     uuid.UUID
	pending   uuid.UUID
	rejected  uuid.UUID
}

func newBookStatusFixture(t *testing.T) *bookStatusFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(catalogStatusDDL + bookStatusExtraDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	f := &bookStatusFixture{
		repo: &GormRepository{db: db}, db: db,
		author: uuid.New(), published: uuid.New(), other: uuid.New(), pending: uuid.New(), rejected: uuid.New(),
	}
	now := time.Now()
	genre := uuid.New()
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if err := db.Exec(q, args...).Error; err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec("INSERT INTO authors (id, name, slug, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		f.author, "Автор", "avtor", domain.AuthorStatusPublished, now, now)
	exec("INSERT INTO genres (id, name, slug, status, created_at) VALUES (?, ?, ?, ?, ?)",
		genre, "роман", "roman", domain.GenreStatusPublished, now)
	books := []struct {
		id     uuid.UUID
		slug   string
		status string
	}{
		{f.published, "published", domain.BookStatusPublished},
		{f.other, "other", domain.BookStatusPublished},
		{f.pending, "pending", domain.BookStatusPending},
		{f.rejected, "rejected", domain.BookStatusRejected},
	}
	user := uuid.New()
	shelf := uuid.New()
	exec("INSERT INTO users (id, email_verified_at) VALUES (?, ?)", user, now)
	exec("INSERT INTO shelves (id, user_id, type) VALUES (?, ?, 'read')", shelf, user)
	for _, b := range books {
		exec("INSERT INTO books (id, title, slug, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
			b.id, "Книга", b.slug, b.status, now, now)
		exec("INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", b.id, f.author)
		exec("INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", b.id, genre)
		exec("INSERT INTO shelf_items (shelf_id, book_id) VALUES (?, ?)", shelf, b.id)
	}
	return f
}

func (f *bookStatusFixture) hidden() []uuid.UUID { return []uuid.UUID{f.pending, f.rejected} }

func idSet[T any](items []T, id func(T) uuid.UUID) map[uuid.UUID]bool {
	out := make(map[uuid.UUID]bool, len(items))
	for _, it := range items {
		out[id(it)] = true
	}
	return out
}

func TestBookStatus_ListBooksAndAuthorPage(t *testing.T) {
	f := newBookStatusFixture(t)
	ctx := context.Background()
	bookID := func(b domain.Book) uuid.UUID { return b.ID }

	books, total, err := f.repo.ListBooks(ctx, BookFilter{})
	got := idSet(books, bookID)
	if err != nil || total != 2 || len(books) != 2 || !got[f.published] || !got[f.other] {
		t.Fatalf("list: total=%d got=%v err=%v", total, got, err)
	}
	// Joined filters must keep the (table-qualified) book predicate.
	books, total, err = f.repo.ListBooks(ctx, BookFilter{AuthorID: &f.author})
	if err != nil || total != 2 || len(books) != 2 {
		t.Fatalf("list by author: total=%d err=%v", total, err)
	}

	a, err := f.repo.GetAuthorBySlug(ctx, "avtor")
	if err != nil {
		t.Fatalf("author: %v", err)
	}
	got = idSet(a.Books, bookID)
	if len(a.Books) != 2 || !got[f.published] || !got[f.other] {
		t.Fatalf("author page books: %v", got)
	}
}

func TestBookStatus_GetBookBySlugAndByIDs(t *testing.T) {
	f := newBookStatusFixture(t)
	ctx := context.Background()

	if b, err := f.repo.GetBookBySlug(ctx, "published"); err != nil || b.ID != f.published || b.Status != domain.BookStatusPublished {
		t.Fatalf("published slug: %+v err=%v", b, err)
	}
	for _, slug := range []string{"pending", "rejected"} {
		if _, err := f.repo.GetBookBySlug(ctx, slug); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("%s slug: expected not found, got %v", slug, err)
		}
	}

	books, err := f.repo.GetBooksByIDs(ctx, []uuid.UUID{f.published, f.other, f.pending, f.rejected})
	got := idSet(books, func(b domain.Book) uuid.UUID { return b.ID })
	if err != nil || len(books) != 2 || !got[f.published] || !got[f.other] {
		t.Fatalf("by ids: %v err=%v", got, err)
	}
}

func TestBookStatus_Similar(t *testing.T) {
	f := newBookStatusFixture(t)
	ctx := context.Background()

	for _, id := range f.hidden() {
		if _, err := f.repo.GetSimilarSource(ctx, id); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("hidden similar source: expected not found, got %v", err)
		}
	}
	src, err := f.repo.GetSimilarSource(ctx, f.published)
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	cands, err := f.repo.ListSimilarCandidates(ctx, f.published, *src, 10, 10)
	if err != nil || len(cands) != 1 || cands[0].BookID != f.other {
		t.Fatalf("candidates: %+v err=%v", cands, err)
	}
}

func TestBookStatus_SimilarityJobPool(t *testing.T) {
	f := newBookStatusFixture(t)
	ctx := context.Background()

	stats, err := f.repo.ListVisibleBookStats(ctx)
	got := idSet(stats, func(s BookStat) uuid.UUID { return s.ID })
	if err != nil || len(stats) != 2 || !got[f.published] || !got[f.other] {
		t.Fatalf("visible stats: %v err=%v", got, err)
	}

	inter, err := f.repo.ListInteractions(ctx)
	got = idSet(inter, func(i Interaction) uuid.UUID { return i.BookID })
	if err != nil || len(inter) != 2 || !got[f.published] || !got[f.other] {
		t.Fatalf("interactions: %v err=%v", got, err)
	}
}
