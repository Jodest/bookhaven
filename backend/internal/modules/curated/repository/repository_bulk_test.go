package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func mkTitled(t *testing.T, db *gorm.DB, title, status, author string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	now := time.Now()
	if err := db.Exec("INSERT INTO books (id, title, slug, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, title, id.String(), status, now, now).Error; err != nil {
		t.Fatal(err)
	}
	if author != "" {
		aid := uuid.New()
		if err := db.Exec("INSERT INTO authors (id, name, slug, status) VALUES (?, ?, ?, 'published')", aid, author, aid.String()).Error; err != nil {
			t.Fatal(err)
		}
		if err := db.Exec("INSERT INTO book_authors (book_id, author_id) VALUES (?, ?)", id, aid).Error; err != nil {
			t.Fatal(err)
		}
	}
	return id
}

func TestAddBooks_AppendAndInsert(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "bulk", 10, true)
	ids := make([]uuid.UUID, 7)
	for i := range ids {
		ids[i] = mkBook(t, db, catalogDomain.BookStatusPublished, "")
	}
	res, err := r.AddBooks(ctx, c.ID, ids[:3], nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 3 || res.Added[0].Position != 1 || res.Added[2].Position != 3 || res.Total != 3 {
		t.Fatalf("append result: %+v", res)
	}
	if !same(order(t, r, c.ID), ids[:3]) {
		t.Fatal("append order")
	}
	// insert D, E at position 2 -> A D E B C
	res, err = r.AddBooks(ctx, c.ID, ids[3:5], intp(2))
	if err != nil {
		t.Fatal(err)
	}
	if res.Added[0].Position != 2 || res.Added[1].Position != 3 || res.Total != 5 {
		t.Fatalf("insert result: %+v", res)
	}
	if !same(order(t, r, c.ID), []uuid.UUID{ids[0], ids[3], ids[4], ids[1], ids[2]}) {
		t.Fatal("insert order")
	}
	// clamp: position 99 -> end, position 0 -> start
	if _, err := r.AddBooks(ctx, c.ID, ids[5:6], intp(99)); err != nil {
		t.Fatal(err)
	}
	if _, err := r.AddBooks(ctx, c.ID, ids[6:7], intp(0)); err != nil {
		t.Fatal(err)
	}
	want := []uuid.UUID{ids[6], ids[0], ids[3], ids[4], ids[1], ids[2], ids[5]}
	if !same(order(t, r, c.ID), want) {
		t.Fatal("clamped order")
	}
}

func TestAddBooks_SkipsPresentAndRepeats(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "bulk2", 10, true)
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	b := mkBook(t, db, catalogDomain.BookStatusPending, "")
	if err := r.AddBook(ctx, c.ID, a, nil); err != nil {
		t.Fatal(err)
	}
	res, err := r.AddBooks(ctx, c.ID, []uuid.UUID{a, b, b}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Added) != 1 || res.Added[0].BookID != b || res.Added[0].Position != 2 {
		t.Fatalf("added: %+v", res.Added)
	}
	if len(res.AlreadyIn) != 2 || res.AlreadyIn[0] != a || res.AlreadyIn[1] != b {
		t.Fatalf("already in: %+v", res.AlreadyIn)
	}
	if !same(order(t, r, c.ID), []uuid.UUID{a, b}) {
		t.Fatal("order")
	}
}

func TestAddBooks_FullWritesNothing(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "bulk3", 10, true)
	now := time.Now()
	for i := 0; i < domain.MaxBooks-2; i++ {
		id := uuid.New()
		if err := db.Exec("INSERT INTO curated_collection_items (collection_id, book_id, position, added_at) VALUES (?, ?, ?, ?)",
			c.ID, id, i+1, now).Error; err != nil {
			t.Fatal(err)
		}
	}
	ids := []uuid.UUID{
		mkBook(t, db, catalogDomain.BookStatusPublished, ""),
		mkBook(t, db, catalogDomain.BookStatusPublished, ""),
		mkBook(t, db, catalogDomain.BookStatusPublished, ""),
	}
	if _, err := r.AddBooks(ctx, c.ID, ids, nil); !errors.Is(err, domain.ErrCollectionFull) {
		t.Fatalf("want full, got %v", err)
	}
	got, _ := r.CollectionBookIDs(ctx, c.ID)
	if len(got) != domain.MaxBooks-2 {
		t.Fatalf("collection changed: %d books", len(got))
	}
	// two fit exactly
	if res, err := r.AddBooks(ctx, c.ID, ids[:2], nil); err != nil || res.Total != domain.MaxBooks {
		t.Fatalf("exact fit: %+v %v", res, err)
	}
}

func TestAddBooks_UnknownCollection(t *testing.T) {
	r, _ := newRepo(t)
	if _, err := r.AddBooks(context.Background(), uuid.New(), []uuid.UUID{uuid.New()}, nil); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("got %v", err)
	}
}

func TestBooksByIDsAndCollectionBookIDs(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	b := mkBook(t, db, catalogDomain.BookStatusRejected, "")
	books, err := r.BooksByIDs(ctx, []uuid.UUID{a, b, uuid.New()})
	if err != nil || len(books) != 2 {
		t.Fatalf("books: %d %v", len(books), err)
	}
	if got, err := r.BooksByIDs(ctx, nil); err != nil || len(got) != 0 {
		t.Fatalf("empty: %v %v", got, err)
	}
	c := mkCollection(t, r, "ids", 5, true)
	if _, err := r.AddBooks(ctx, c.ID, []uuid.UUID{b, a}, nil); err != nil {
		t.Fatal(err)
	}
	ids, _ := r.CollectionBookIDs(ctx, c.ID)
	if !same(ids, []uuid.UUID{b, a}) {
		t.Fatal("collection ids order")
	}
}

// TitleCandidates runs a Postgres word-index query in production; SQLite
// takes the instr fallback, which only folds ASCII case, so titles here are
// stored lower-case. The real full-text query is not covered by SQLite tests.
func TestTitleCandidates_SQLiteFallback(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	short := mkTitled(t, db, "мать", catalogDomain.BookStatusPublished, "Максим Горький")
	long := mkTitled(t, db, "мать и дочь", catalogDomain.BookStatusPublished, "")
	mkTitled(t, db, "война и мир", catalogDomain.BookStatusPublished, "")
	yo := mkTitled(t, db, "ёлка: повесть", catalogDomain.BookStatusPending, "")

	got, err := r.TitleCandidates(ctx, "мать", 50)
	if err != nil || len(got) != 2 || got[0].ID != short || got[1].ID != long {
		t.Fatalf("mother: %+v %v", got, err)
	}
	if len(got[0].Authors) != 1 || got[0].Authors[0].Name != "Максим Горький" {
		t.Fatalf("authors not preloaded: %+v", got[0].Authors)
	}
	got, _ = r.TitleCandidates(ctx, "елка повесть", 50)
	if len(got) != 1 || got[0].ID != yo {
		t.Fatalf("yo: %+v", got)
	}
	if got, _ = r.TitleCandidates(ctx, "мать", 1); len(got) != 1 {
		t.Fatalf("limit: %d", len(got))
	}
	if got, _ = r.TitleCandidates(ctx, "", 50); len(got) != 0 {
		t.Fatal("empty key")
	}
}
