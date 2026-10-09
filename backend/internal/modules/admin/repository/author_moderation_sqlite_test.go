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

// moderationDDL recreates the authors/books/book_authors columns the
// moderation queue queries touch, with SQLite-native types (the production
// schema is Postgres-only). Search (plainto_tsquery) is Postgres-only and
// not exercised here.
const moderationDDL = `
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
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);
CREATE TABLE books (
	id TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'pending'
);
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL, PRIMARY KEY (book_id, author_id));
`

type modFixture struct {
	t    *testing.T
	repo *GormRepository
	db   *gorm.DB
	base time.Time
	n    int
}

func newModerationRepo(t *testing.T) *modFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(moderationDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &modFixture{t: t, repo: &GormRepository{db: db}, db: db, base: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)}
}

// author inserts an author; each call is one minute newer than the last.
func (f *modFixture) author(name, status string, qid *string) uuid.UUID {
	f.t.Helper()
	f.n++
	id := uuid.New()
	var source *string
	if qid != nil {
		s := "wikidata"
		source = &s
	}
	ts := f.base.Add(time.Duration(f.n) * time.Minute)
	if err := f.db.Exec(
		"INSERT INTO authors (id, name, slug, status, external_source, external_id, biography, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		id, name, id.String(), status, source, qid, "bio", ts, ts,
	).Error; err != nil {
		f.t.Fatalf("insert author: %v", err)
	}
	return id
}

func (f *modFixture) book(title string, hidden bool, authorIDs ...uuid.UUID) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	status := catalogDomain.BookStatusPublished
	if hidden {
		status = catalogDomain.BookStatusPending
	}
	if err := f.db.Exec("INSERT INTO books (id, title, status) VALUES (?, ?, ?)", id, title, status).Error; err != nil {
		f.t.Fatalf("insert book: %v", err)
	}
	for _, a := range authorIDs {
		if err := f.db.Exec("INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", id, a).Error; err != nil {
			f.t.Fatalf("link book: %v", err)
		}
	}
	return id
}

func strPtr(s string) *string { return &s }

func rowIDs(rows []AuthorModerationRow) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Author.ID)
	}
	return out
}

func TestListAuthorsForModeration_FiltersCountsAndOrder(t *testing.T) {
	f := newModerationRepo(t)
	ctx := context.Background()

	noBooks := f.author("Без Книг", "pending", nil)
	onlyHidden := f.author("Только Скрытые", "pending", strPtr("Q1"))
	mixed := f.author("Смешанный Автор", "pending", nil)
	published := f.author("Опубликованный", "published", strPtr("Q2"))
	f.author("Отклонённый", "rejected", nil)

	f.book("Скрытая 1", true, onlyHidden)
	f.book("Скрытая 2", true, onlyHidden, mixed)
	f.book("Видимая", false, mixed)
	f.book("Чужая", false, published)

	rows, total, err := f.repo.ListAuthorsForModeration(ctx, AuthorModerationFilter{Status: "pending", Limit: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("expected 3 pending, got %d (total %d)", len(rows), total)
	}
	// Newest first.
	if got := rowIDs(rows); got[0] != mixed || got[1] != onlyHidden || got[2] != noBooks {
		t.Fatalf("unexpected order: %v", got)
	}
	counts := map[uuid.UUID][2]int64{}
	for _, r := range rows {
		counts[r.Author.ID] = [2]int64{r.BooksTotal, r.BooksVisible}
	}
	if counts[noBooks] != [2]int64{0, 0} || counts[onlyHidden] != [2]int64{2, 0} || counts[mixed] != [2]int64{2, 1} {
		t.Fatalf("unexpected counts: %v", counts)
	}
	if rows[1].Author.ExternalID == nil || *rows[1].Author.ExternalID != "Q1" || rows[1].Author.Name != "Только Скрытые" || rows[1].Author.CreatedAt.IsZero() {
		t.Fatalf("author fields not scanned: %+v", rows[1].Author)
	}

	rows, total, err = f.repo.ListAuthorsForModeration(ctx, AuthorModerationFilter{Status: "pending", QID: "with"})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Author.ID != onlyHidden {
		t.Fatalf("qid=with: %v %d %v", err, total, rowIDs(rows))
	}
	rows, total, err = f.repo.ListAuthorsForModeration(ctx, AuthorModerationFilter{Status: "pending", QID: "without"})
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("qid=without: %v %d %v", err, total, rowIDs(rows))
	}
	rows, total, err = f.repo.ListAuthorsForModeration(ctx, AuthorModerationFilter{Status: "pending", Books: "none"})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Author.ID != noBooks {
		t.Fatalf("books=none: %v %d %v", err, total, rowIDs(rows))
	}
	rows, total, err = f.repo.ListAuthorsForModeration(ctx, AuthorModerationFilter{Status: "published", QID: "with"})
	if err != nil || total != 1 || len(rows) != 1 || rows[0].Author.ID != published || rows[0].BooksVisible != 1 {
		t.Fatalf("published+qid: %v %d %+v", err, total, rows)
	}
	// Pagination keeps total.
	rows, total, err = f.repo.ListAuthorsForModeration(ctx, AuthorModerationFilter{Status: "pending", Limit: 1, Offset: 1})
	if err != nil || total != 3 || len(rows) != 1 || rows[0].Author.ID != onlyHidden {
		t.Fatalf("paging: %v %d %v", err, total, rowIDs(rows))
	}
}

func TestListAuthorSampleBooks_AtMostThreeByTitle(t *testing.T) {
	f := newModerationRepo(t)
	a := f.author("Автор А", "pending", nil)
	b := f.author("Автор Б", "pending", nil)
	none := f.author("Автор В", "pending", nil)

	f.book("Г", false, a)
	f.book("А", true, a)
	f.book("В", false, a, b)
	f.book("Б", false, a)

	samples, err := f.repo.ListAuthorSampleBooks(context.Background(), []uuid.UUID{a, b, none})
	if err != nil {
		t.Fatalf("samples: %v", err)
	}
	got := samples[a]
	if len(got) != 3 || got[0].Title != "А" || got[0].Status != catalogDomain.BookStatusPending || got[1].Title != "Б" || got[2].Title != "В" {
		t.Fatalf("unexpected samples for a: %+v", got)
	}
	if len(samples[b]) != 1 || samples[b][0].Title != "В" {
		t.Fatalf("unexpected samples for b: %+v", samples[b])
	}
	if len(samples[none]) != 0 {
		t.Fatalf("expected no samples, got %+v", samples[none])
	}
}

func TestFindAuthorsBySurnames_Prefilter(t *testing.T) {
	f := newModerationRepo(t)
	// SQLite lower() is ASCII-only, so the prefilter is checked with
	// Latin names; Cyrillic folding is Postgres' lower() + replace().
	self := f.author("L. N. Tolstoy", "pending", nil)
	full := f.author("Lev Tolstoy", "published", nil)
	comma := f.author("Tolstoy, Lev", "pending", nil)
	commaSpace := f.author("Tolstoy , Aleksey", "published", nil)
	bare := f.author("TOLSTOY", "published", nil)
	f.author("Lev Tolstoy", "rejected", nil)
	f.author("Tolstoyevsky", "published", nil)
	f.author("Ivan Turgenev", "published", nil)
	f.author("100% Tolstoy_x", "published", nil)

	found, err := f.repo.FindAuthorsBySurnames(context.Background(), []string{"tolstoy"}, []uuid.UUID{self})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	got := make([]string, 0, len(found))
	for _, a := range found {
		got = append(got, a.ID.String())
	}
	want := []string{full.String(), comma.String(), commaSpace.String(), bare.String()}
	sort.Strings(got)
	sort.Strings(want)
	if len(got) != len(want) {
		t.Fatalf("expected %d candidates, got %d: %+v", len(want), len(got), found)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("unexpected candidates: %+v", found)
		}
	}

	// LIKE metacharacters in a surname are literal.
	found, err = f.repo.FindAuthorsBySurnames(context.Background(), []string{"tolst_y"}, nil)
	if err != nil || len(found) != 0 {
		t.Fatalf("expected escaped '_' to match nothing, got %v %+v", err, found)
	}

	found, err = f.repo.FindAuthorsBySurnames(context.Background(), nil, nil)
	if err != nil || len(found) != 0 {
		t.Fatalf("expected no candidates for no surnames, got %v %+v", err, found)
	}
}

func TestSetAuthorStatus_OnlyStatusChanges(t *testing.T) {
	f := newModerationRepo(t)
	ctx := context.Background()
	id := f.author("Лев Толстой", "pending", strPtr("Q7243"))
	bookID := f.book("Война и мир", false, id)

	type snapshot struct {
		Name       string
		Slug       string
		Biography  *string
		Status     string
		ExternalID *string
		UpdatedAt  time.Time
	}
	load := func() snapshot {
		var s snapshot
		if err := f.db.Raw("SELECT name, slug, biography, status, external_id, updated_at FROM authors WHERE id = ?", id).Scan(&s).Error; err != nil {
			t.Fatalf("load: %v", err)
		}
		return s
	}
	before := load()

	found, err := f.repo.SetAuthorStatus(ctx, id, "rejected")
	if err != nil || !found {
		t.Fatalf("SetAuthorStatus: found=%v err=%v", found, err)
	}
	after := load()
	if after.Status != "rejected" {
		t.Fatalf("expected rejected, got %q", after.Status)
	}
	if after.Name != before.Name || after.Slug != before.Slug || after.Biography == nil || *after.Biography != "bio" ||
		after.ExternalID == nil || *after.ExternalID != "Q7243" {
		t.Fatalf("non-status columns changed: before %+v after %+v", before, after)
	}
	if !after.UpdatedAt.After(before.UpdatedAt) {
		t.Fatalf("expected updated_at bumped: %v -> %v", before.UpdatedAt, after.UpdatedAt)
	}
	var links int64
	if err := f.db.Raw("SELECT COUNT(*) FROM book_authors WHERE author_id = ? AND book_id = ?", id, bookID).Scan(&links).Error; err != nil || links != 1 {
		t.Fatalf("expected book link kept, got %d (%v)", links, err)
	}

	// Same status again is still "found" (no-op success).
	if found, err := f.repo.SetAuthorStatus(ctx, id, "rejected"); err != nil || !found {
		t.Fatalf("no-op: found=%v err=%v", found, err)
	}

	if found, err := f.repo.SetAuthorStatus(ctx, uuid.New(), "published"); err != nil || found {
		t.Fatalf("unknown id: found=%v err=%v", found, err)
	}
}
