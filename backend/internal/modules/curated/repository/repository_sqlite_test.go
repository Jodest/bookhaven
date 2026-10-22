package repository

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const curatedDDL = `
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', slug TEXT NOT NULL DEFAULT '', description TEXT, cover_url TEXT, isbn TEXT, published_year INTEGER, page_count INTEGER, language TEXT DEFAULT 'ru', rating REAL DEFAULT 0, ratings_count INTEGER DEFAULT 0, reviews_count INTEGER DEFAULT 0, status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT, created_at DATETIME, updated_at DATETIME, cover_source_url TEXT, cover_path TEXT, cover_status TEXT, cover_error TEXT, cover_attempts INTEGER NOT NULL DEFAULT 0, cover_next_attempt_at DATETIME, cover_stored_at DATETIME, cover_checked_at DATETIME, published_at DATETIME);
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT);
CREATE TABLE genres (id TEXT PRIMARY KEY, name TEXT, slug TEXT, status TEXT);
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL);
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL);
CREATE TABLE curated_collections (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT 'manual', size INTEGER NOT NULL, source TEXT, source_url TEXT, genre_id TEXT, is_published BOOLEAN NOT NULL DEFAULT 0, published_at DATETIME, rule_metric TEXT, rule_period_days INTEGER, rule_min_ratings INTEGER, rule_confidence INTEGER, computed_at DATETIME, baseline_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE TABLE curated_collection_items (collection_id TEXT NOT NULL, book_id TEXT NOT NULL, position INTEGER NOT NULL, added_at DATETIME, previous_position INTEGER, score REAL, source_screen_work_id TEXT, source_votes INTEGER, source_share REAL, PRIMARY KEY (collection_id, book_id));
CREATE TABLE ratings (id TEXT PRIMARY KEY, book_id TEXT NOT NULL, user_id TEXT NOT NULL, value INTEGER NOT NULL, created_at DATETIME, updated_at DATETIME);
CREATE TABLE shelves (id TEXT PRIMARY KEY, user_id TEXT NOT NULL, name TEXT, type TEXT NOT NULL DEFAULT 'custom');
CREATE TABLE shelf_items (shelf_id TEXT NOT NULL, book_id TEXT NOT NULL, added_at DATETIME, PRIMARY KEY (shelf_id, book_id));
CREATE TABLE screen_works (id TEXT PRIMARY KEY, title TEXT NOT NULL, kind TEXT NOT NULL DEFAULT 'film', year INTEGER);
CREATE TABLE book_screen_works (book_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, hidden BOOLEAN NOT NULL DEFAULT 0, PRIMARY KEY (book_id, screen_work_id));
CREATE TABLE adaptation_votes (user_id TEXT NOT NULL, book_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, value TEXT NOT NULL, created_at DATETIME, updated_at DATETIME, PRIMARY KEY (user_id, book_id, screen_work_id));
`

func newRepo(t *testing.T) (*GormRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(curatedDDL).Error; err != nil {
		t.Fatalf("schema: %v", err)
	}
	return &GormRepository{db: db}, db
}

func mkBook(t *testing.T, db *gorm.DB, status string, cover string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var c any
	if cover != "" {
		c = cover
	}
	now := time.Now()
	if err := db.Exec("INSERT INTO books (id, title, slug, status, cover_url, created_at, updated_at) VALUES (?, 'Книга', ?, ?, ?, ?, ?)",
		id, id.String(), status, c, now, now).Error; err != nil {
		t.Fatalf("insert book: %v", err)
	}
	return id
}

func mkCollection(t *testing.T, r *GormRepository, slug string, size int, published bool) *domain.Collection {
	t.Helper()
	c := &domain.Collection{ID: uuid.New(), Slug: slug, Title: slug, Kind: domain.KindManual, Size: size, IsPublished: published}
	if published {
		now := time.Now()
		c.PublishedAt = &now
	}
	if err := r.Create(context.Background(), c); err != nil {
		t.Fatalf("create collection: %v", err)
	}
	return c
}

func order(t *testing.T, r *GormRepository, id uuid.UUID) []uuid.UUID {
	t.Helper()
	items, err := r.AdminItems(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]uuid.UUID, len(items))
	for i, it := range items {
		if it.Position != i+1 {
			t.Fatalf("positions not dense: item %d has position %d", i, it.Position)
		}
		out[i] = it.Book.ID
	}
	return out
}

func same(a, b []uuid.UUID) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestAddRemoveMove_DensePositions(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "c1", 3, true)
	ids := make([]uuid.UUID, 5)
	for i := range ids {
		ids[i] = mkBook(t, db, catalogDomain.BookStatusPublished, "")
	}
	// append A B C
	for _, id := range ids[:3] {
		if err := r.AddBook(ctx, c.ID, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	// add D at position 1 -> D A B C
	if err := r.AddBook(ctx, c.ID, ids[3], intp(1)); err != nil {
		t.Fatal(err)
	}
	if !same(order(t, r, c.ID), []uuid.UUID{ids[3], ids[0], ids[1], ids[2]}) {
		t.Fatal("add at 1")
	}
	// add E at position 99 (clamped to end)
	if err := r.AddBook(ctx, c.ID, ids[4], intp(99)); err != nil {
		t.Fatal(err)
	}
	if !same(order(t, r, c.ID), []uuid.UUID{ids[3], ids[0], ids[1], ids[2], ids[4]}) {
		t.Fatal("add clamped")
	}
	if err := r.AddBook(ctx, c.ID, ids[0], nil); !errors.Is(err, domain.ErrBookAlreadyIn) {
		t.Fatalf("dup: %v", err)
	}
	// move E (5) to 1 -> E D A B C
	if err := r.MoveBook(ctx, c.ID, ids[4], 1); err != nil {
		t.Fatal(err)
	}
	if !same(order(t, r, c.ID), []uuid.UUID{ids[4], ids[3], ids[0], ids[1], ids[2]}) {
		t.Fatal("move up")
	}
	// move E (1) to 3 -> D A E B C
	if err := r.MoveBook(ctx, c.ID, ids[4], 3); err != nil {
		t.Fatal(err)
	}
	if !same(order(t, r, c.ID), []uuid.UUID{ids[3], ids[0], ids[4], ids[1], ids[2]}) {
		t.Fatal("move down")
	}
	// clamp: move D to 100 -> end
	if err := r.MoveBook(ctx, c.ID, ids[3], 100); err != nil {
		t.Fatal(err)
	}
	if !same(order(t, r, c.ID), []uuid.UUID{ids[0], ids[4], ids[1], ids[2], ids[3]}) {
		t.Fatal("move clamp")
	}
	// remove middle
	if err := r.RemoveBook(ctx, c.ID, ids[1]); err != nil {
		t.Fatal(err)
	}
	if !same(order(t, r, c.ID), []uuid.UUID{ids[0], ids[4], ids[2], ids[3]}) {
		t.Fatal("remove")
	}
	if err := r.RemoveBook(ctx, c.ID, ids[1]); !errors.Is(err, domain.ErrBookNotInList) {
		t.Fatalf("remove missing: %v", err)
	}
	if err := r.MoveBook(ctx, c.ID, ids[1], 1); !errors.Is(err, domain.ErrBookNotInList) {
		t.Fatalf("move missing: %v", err)
	}
	if err := r.AddBook(ctx, uuid.New(), ids[1], nil); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("missing collection: %v", err)
	}
}

func intp(i int) *int { return &i }

func TestPublishedPage_GlobalPlacesAndHidden(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "c2", 2, true)
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "http://a")
	b := mkBook(t, db, catalogDomain.BookStatusPending, "http://b")
	cc := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	d := mkBook(t, db, catalogDomain.BookStatusPublished, "http://d")
	for _, id := range []uuid.UUID{a, b, cc, d} {
		if err := r.AddBook(ctx, c.ID, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	books, err := r.PublishedPage(ctx, c.ID, 10, 0)
	if err != nil || len(books) != 3 || books[0].Book.ID != a || books[1].Book.ID != cc || books[2].Book.ID != d {
		t.Fatalf("page: %v %v", err, books)
	}
	page2, _ := r.PublishedPage(ctx, c.ID, 1, 1)
	if len(page2) != 1 || page2[0].Book.ID != cc {
		t.Fatal("offset page")
	}
	counts, _ := r.Counts(ctx, []uuid.UUID{c.ID})
	if counts[c.ID].Books != 4 || counts[c.ID].Published != 3 {
		t.Fatalf("counts %+v", counts[c.ID])
	}
	covers, _ := r.Covers(ctx, c.ID, 2)
	if len(covers) != 1 || covers[0] != "http://a" {
		t.Fatalf("covers %v", covers)
	}
}

func TestListPublished_ExcludesDrafts(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	mkCollection(t, r, "pub", 1, true)
	mkCollection(t, r, "draft", 1, false)
	got, err := r.ListPublished(ctx)
	if err != nil || len(got) != 1 || got[0].Slug != "pub" {
		t.Fatalf("published: %v %v", err, got)
	}
	if _, err := r.GetPublishedBySlug(ctx, "draft"); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("draft by slug: %v", err)
	}
	all, _ := r.ListAll(ctx)
	if len(all) != 2 {
		t.Fatal("admin list must hold drafts")
	}
}

func TestSlugUniqueAndDeleteRemovesItems(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "dup", 1, false)
	err := r.Create(ctx, &domain.Collection{ID: uuid.New(), Slug: "dup", Title: "x", Kind: domain.KindManual, Size: 1})
	if !errors.Is(err, domain.ErrSlugTaken) {
		t.Fatalf("slug: %v", err)
	}
	book := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	if err := r.AddBook(ctx, c.ID, book, nil); err != nil {
		t.Fatal(err)
	}
	ok, err := r.Delete(ctx, c.ID)
	if err != nil || !ok {
		t.Fatalf("delete: %v %v", ok, err)
	}
	var n int64
	db.Table("curated_collection_items").Count(&n)
	if n != 0 {
		t.Fatal("items must be deleted")
	}
	var books int64
	db.Table("books").Count(&books)
	if books != 1 {
		t.Fatal("books must stay")
	}
}

// Collection cover thumbnails use the displayed link: our copy when stored,
// nothing when the source has no cover, the source link otherwise (#489).
func TestCovers_DisplayedLinks(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "covers", 4, true)
	stored := mkBook(t, db, catalogDomain.BookStatusPublished, "http://stored")
	absent := mkBook(t, db, catalogDomain.BookStatusPublished, "http://absent")
	plain := mkBook(t, db, catalogDomain.BookStatusPublished, "http://plain")
	for _, id := range []uuid.UUID{stored, absent, plain} {
		if err := r.AddBook(ctx, c.ID, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	path := "ab/" + strings.Repeat("ab", 32) + ".jpg"
	db.Exec("UPDATE books SET cover_source_url = cover_url, cover_status = 'stored', cover_path = ? WHERE id = ?", path, stored)
	db.Exec("UPDATE books SET cover_source_url = cover_url, cover_status = 'absent' WHERE id = ?", absent)
	covers, err := r.Covers(ctx, c.ID, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(covers) != 2 || covers[0] != "/covers/"+path || covers[1] != "http://plain" {
		t.Fatalf("covers %v", covers)
	}
}
