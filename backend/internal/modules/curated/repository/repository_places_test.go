package repository

import (
	"context"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func placeOf(t *testing.T, r *GormRepository, book uuid.UUID, slug string) (int, bool) {
	t.Helper()
	ps, err := r.BookPlaces(context.Background(), book)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ps {
		if p.Collection.Slug == slug {
			return p.Place, true
		}
	}
	return 0, false
}

func TestBookPlaces_PendingAheadShiftsPlaceAndReserveHidden(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "top", 2, true)
	pending := mkBook(t, db, catalogDomain.BookStatusPending, "")
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	b := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	reserve := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	for _, id := range []uuid.UUID{pending, a, b, reserve} {
		if err := r.AddBook(ctx, c.ID, id, nil); err != nil {
			t.Fatal(err)
		}
	}
	if p, ok := placeOf(t, r, a, "top"); !ok || p != 1 {
		t.Fatalf("a: %d %v", p, ok)
	}
	if p, ok := placeOf(t, r, b, "top"); !ok || p != 2 {
		t.Fatalf("b: %d %v", p, ok)
	}
	if _, ok := placeOf(t, r, reserve, "top"); ok {
		t.Fatal("reserve must have no place")
	}
}

func TestBookPlaces_DraftCollectionHidden(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "draft", 3, false)
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	if err := r.AddBook(ctx, c.ID, a, nil); err != nil {
		t.Fatal(err)
	}
	if _, ok := placeOf(t, r, a, "draft"); ok {
		t.Fatal("draft collection must not give a place")
	}
}

func TestBookPlaces_AutoSnapshotUnpublishedAhead(t *testing.T) {
	r, db := newRepo(t)
	c := mkCollection(t, r, "auto", 10, true)
	db.Exec("UPDATE curated_collections SET kind = 'auto' WHERE id = ?", c.ID)
	hidden := mkBook(t, db, catalogDomain.BookStatusPending, "")
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	b := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	cc := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	for i, id := range []uuid.UUID{a, b, hidden, cc} {
		if err := db.Exec("INSERT INTO curated_collection_items (collection_id, book_id, position) VALUES (?, ?, ?)", c.ID, id, i+1).Error; err != nil {
			t.Fatal(err)
		}
	}
	// A, B, (hidden at 3), C at stored position 4 -> public place 3.
	if p, ok := placeOf(t, r, cc, "auto"); !ok || p != 3 {
		t.Fatalf("auto place: %d %v", p, ok)
	}
}

func TestBookPlaces_OrderBestPlaceThenSizeThenNewest(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	book := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	other := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	mk := func(slug string, size int, ahead bool, daysAgo int) {
		c := mkCollection(t, r, slug, size, true)
		at := time.Now().AddDate(0, 0, -daysAgo)
		db.Exec("UPDATE curated_collections SET published_at = ? WHERE id = ?", at, c.ID)
		if ahead {
			if err := r.AddBook(ctx, c.ID, other, nil); err != nil {
				t.Fatal(err)
			}
		}
		if err := r.AddBook(ctx, c.ID, book, nil); err != nil {
			t.Fatal(err)
		}
	}
	mk("second-place", 10, true, 1)
	mk("first-big-old", 100, false, 5)
	mk("first-small", 20, false, 9)
	mk("first-big-new", 100, false, 2)
	ps, err := r.BookPlaces(ctx, book)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range ps {
		got = append(got, p.Collection.Slug)
	}
	want := []string{"first-small", "first-big-new", "first-big-old", "second-place"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}
	if ps[0].Collection.PublishedAt == nil || ps[0].Collection.Kind != "manual" || ps[0].Collection.Size != 20 {
		t.Fatalf("collection fields: %+v", ps[0].Collection)
	}
}

// Every published book in the visible range must get the same place that
// PublishedPage serves for /top/{slug}.
func TestBookPlaces_MatchesPublishedPage(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	c := mkCollection(t, r, "mix", 4, true)
	statuses := []string{"published", "pending", "published", "rejected", "published", "published", "published", "pending", "published"}
	ids := make([]uuid.UUID, len(statuses))
	for i, st := range statuses {
		ids[i] = mkBook(t, db, st, "")
		if err := r.AddBook(ctx, c.ID, ids[i], nil); err != nil {
			t.Fatal(err)
		}
	}
	page, err := r.PublishedPage(ctx, c.ID, 100, 0)
	if err != nil {
		t.Fatal(err)
	}
	for i, pb := range page {
		p, ok := placeOf(t, r, pb.Book.ID, "mix")
		if i < c.Size {
			if !ok || p != i+1 {
				t.Fatalf("index %d: place %d ok=%v", i, p, ok)
			}
		} else if ok {
			t.Fatalf("index %d is reserve but has place %d", i, p)
		}
	}
}
