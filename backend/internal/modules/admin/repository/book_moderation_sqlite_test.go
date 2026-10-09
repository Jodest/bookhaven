package repository

import (
	"context"
	"sort"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Book moderation queue repository tests
// (docs/features/book-moderation-queue.md). Reuses bookStatusDDL
// (books/authors/genres/book_authors/book_genres with SQLite-native
// types). Search (to_tsvector) is Postgres-only and not exercised here.

type bookModFixture struct {
	t    *testing.T
	repo *GormRepository
	db   *gorm.DB
	base time.Time
	n    int
}

func newBookModRepo(t *testing.T) *bookModFixture {
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
	if err := db.Exec(bookStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &bookModFixture{t: t, repo: &GormRepository{db: db}, db: db, base: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)}
}

// book inserts a book; each call is one minute newer than the last unless
// sameTime is set (then it reuses the previous timestamp).
func (f *bookModFixture) book(title, status string, isbn, source *string, sameTime bool) uuid.UUID {
	f.t.Helper()
	if !sameTime {
		f.n++
	}
	id := uuid.New()
	at := f.base.Add(time.Duration(f.n) * time.Minute)
	if err := f.db.Exec("INSERT INTO books (id, title, slug, status, isbn, external_source, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
		id, title, id.String(), status, isbn, source, at, at).Error; err != nil {
		f.t.Fatalf("insert book: %v", err)
	}
	return id
}

func (f *bookModFixture) author(name, status string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO authors (id, name, slug, status) VALUES (?, ?, ?, ?)", id, name, id.String(), status).Error; err != nil {
		f.t.Fatalf("insert author: %v", err)
	}
	return id
}

func (f *bookModFixture) genre(name, status string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO genres (id, name, slug, status) VALUES (?, ?, ?, ?)", id, name, id.String(), status).Error; err != nil {
		f.t.Fatalf("insert genre: %v", err)
	}
	return id
}

func (f *bookModFixture) link(bookID uuid.UUID, authorIDs []uuid.UUID, genreIDs []uuid.UUID) {
	f.t.Helper()
	for _, a := range authorIDs {
		if err := f.db.Exec("INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", bookID, a).Error; err != nil {
			f.t.Fatalf("link author: %v", err)
		}
	}
	for _, g := range genreIDs {
		if err := f.db.Exec("INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", bookID, g).Error; err != nil {
			f.t.Fatalf("link genre: %v", err)
		}
	}
}

func bookIDs(books []catalogDomain.Book) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(books))
	for _, b := range books {
		out = append(out, b.ID)
	}
	return out
}

func TestListBooksForModeration_FiltersOrderingAndPreload(t *testing.T) {
	f := newBookModRepo(t)
	ctx := context.Background()
	src := "openlibrary"

	oldPending := f.book("Старая", catalogDomain.BookStatusPending, nil, nil, false)
	f.book("Опубликованная", catalogDomain.BookStatusPublished, nil, nil, false)
	harvested := f.book("Собранная", catalogDomain.BookStatusPending, nil, &src, false)
	tieB := f.book("Б-книга", catalogDomain.BookStatusPending, nil, nil, false)
	tieA := f.book("А-книга", catalogDomain.BookStatusPending, nil, nil, true)
	f.book("Отклонённая", catalogDomain.BookStatusRejected, nil, nil, false)

	author := f.author("Лев Толстой", catalogDomain.AuthorStatusPending)
	genre := f.genre("роман", catalogDomain.GenreStatusRejected)
	f.link(tieA, []uuid.UUID{author}, []uuid.UUID{genre})

	books, total, err := f.repo.ListBooksForModeration(ctx, BookModerationFilter{Status: catalogDomain.BookStatusPending, Limit: 2})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 4 {
		t.Fatalf("expected total 4 (ignores limit), got %d", total)
	}
	if got := bookIDs(books); len(got) != 2 || got[0] != tieA || got[1] != tieB {
		t.Fatalf("expected newest first with title tie-break [%s %s], got %v", tieA, tieB, got)
	}
	if len(books[0].Authors) != 1 || books[0].Authors[0].Status != catalogDomain.AuthorStatusPending ||
		len(books[0].Genres) != 1 || books[0].Genres[0].Status != catalogDomain.GenreStatusRejected {
		t.Fatalf("expected preloaded authors/genres with status, got %+v / %+v", books[0].Authors, books[0].Genres)
	}

	books, total, err = f.repo.ListBooksForModeration(ctx, BookModerationFilter{Status: catalogDomain.BookStatusPending, Limit: 2, Offset: 2})
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if got := bookIDs(books); total != 4 || len(got) != 2 || got[0] != harvested || got[1] != oldPending {
		t.Fatalf("unexpected page 2: total=%d %v", total, got)
	}

	books, total, err = f.repo.ListBooksForModeration(ctx, BookModerationFilter{Status: catalogDomain.BookStatusPending, Source: "harvested"})
	if err != nil {
		t.Fatalf("list harvested: %v", err)
	}
	if got := bookIDs(books); total != 1 || len(got) != 1 || got[0] != harvested {
		t.Fatalf("unexpected harvested: total=%d %v", total, got)
	}

	_, total, err = f.repo.ListBooksForModeration(ctx, BookModerationFilter{Status: catalogDomain.BookStatusPending, Source: "manual"})
	if err != nil {
		t.Fatalf("list manual: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected 3 manual pending, got %d", total)
	}

	_, total, err = f.repo.ListBooksForModeration(ctx, BookModerationFilter{Status: catalogDomain.BookStatusRejected})
	if err != nil {
		t.Fatalf("list rejected: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected 1 rejected, got %d", total)
	}
}

func TestFindBooksByNormalizedISBNs(t *testing.T) {
	f := newBookModRepo(t)
	ctx := context.Background()
	hyphen, spaced, lower, other := "978-0-306-40615-7", "978 0306 406157", "080442957x", "9791090636071"

	a := f.book("Дефисы", catalogDomain.BookStatusRejected, &hyphen, nil, false)
	b := f.book("Пробелы", catalogDomain.BookStatusPublished, &spaced, nil, false)
	c := f.book("Строчная x", catalogDomain.BookStatusPending, &lower, nil, false)
	f.book("Другой", catalogDomain.BookStatusPublished, &other, nil, false)
	f.book("Без ISBN", catalogDomain.BookStatusPublished, nil, nil, false)

	got, err := f.repo.FindBooksByNormalizedISBNs(ctx, []string{"9780306406157", "080442957X"}, []uuid.UUID{c})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	ids := map[uuid.UUID]BookHintCandidate{}
	for _, row := range got {
		ids[row.ID] = row
	}
	if len(got) != 2 || ids[a].ID != a || ids[b].ID != b {
		t.Fatalf("expected hyphenated and spaced matches (excluded id skipped), got %+v", got)
	}
	if ids[a].Status != catalogDomain.BookStatusRejected || ids[a].ISBN == nil || *ids[a].ISBN != hyphen || ids[a].Title != "Дефисы" || ids[a].Slug != a.String() {
		t.Fatalf("unexpected candidate fields: %+v", ids[a])
	}

	got, err = f.repo.FindBooksByNormalizedISBNs(ctx, nil, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil result for empty input, got %v err=%v", got, err)
	}
}

func TestFindBooksByAuthorIDs(t *testing.T) {
	f := newBookModRepo(t)
	ctx := context.Background()
	a1 := f.author("Автор 1", catalogDomain.AuthorStatusPublished)
	a2 := f.author("Автор 2", catalogDomain.AuthorStatusPublished)
	a3 := f.author("Автор 3", catalogDomain.AuthorStatusPublished)

	both := f.book("Соавторы", catalogDomain.BookStatusPublished, nil, nil, false)
	f.link(both, []uuid.UUID{a1, a2, a3}, nil)
	one := f.book("Один", catalogDomain.BookStatusRejected, nil, nil, false)
	f.link(one, []uuid.UUID{a2}, nil)
	excluded := f.book("Исключённая", catalogDomain.BookStatusPending, nil, nil, false)
	f.link(excluded, []uuid.UUID{a1}, nil)
	unrelated := f.book("Чужая", catalogDomain.BookStatusPublished, nil, nil, false)
	f.link(unrelated, []uuid.UUID{a3}, nil)

	got, err := f.repo.FindBooksByAuthorIDs(ctx, []uuid.UUID{a1, a2}, []uuid.UUID{excluded})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	byID := map[uuid.UUID]BookHintCandidate{}
	for _, row := range got {
		byID[row.ID] = row
	}
	if len(got) != 2 || byID[both].ID != both || byID[one].ID != one {
		t.Fatalf("expected two grouped candidates, got %+v", got)
	}
	gotAuthors := append([]uuid.UUID(nil), byID[both].AuthorIDs...)
	sort.Slice(gotAuthors, func(i, j int) bool { return gotAuthors[i].String() < gotAuthors[j].String() })
	wantAuthors := []uuid.UUID{a1, a2}
	sort.Slice(wantAuthors, func(i, j int) bool { return wantAuthors[i].String() < wantAuthors[j].String() })
	if len(gotAuthors) != 2 || gotAuthors[0] != wantAuthors[0] || gotAuthors[1] != wantAuthors[1] {
		t.Fatalf("expected matched author ids %v, got %v", wantAuthors, gotAuthors)
	}
	if len(byID[one].AuthorIDs) != 1 || byID[one].AuthorIDs[0] != a2 || byID[one].Status != catalogDomain.BookStatusRejected {
		t.Fatalf("unexpected candidate: %+v", byID[one])
	}

	got, err = f.repo.FindBooksByAuthorIDs(ctx, nil, nil)
	if err != nil || got == nil || len(got) != 0 {
		t.Fatalf("expected empty non-nil result for empty input, got %v err=%v", got, err)
	}
}
