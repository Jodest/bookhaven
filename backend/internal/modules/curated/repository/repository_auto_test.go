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

func mkAuto(t *testing.T, r *GormRepository, metric string, size int, genre *uuid.UUID) *domain.Collection {
	t.Helper()
	c := &domain.Collection{ID: uuid.New(), Slug: uuid.NewString(), Title: "auto", Kind: domain.KindAuto, Size: size, GenreID: genre}
	c.SetRule(domain.Rule{Metric: metric})
	if err := r.Create(context.Background(), c); err != nil {
		t.Fatalf("create auto: %v", err)
	}
	return c
}

func rate(t *testing.T, db *gorm.DB, book uuid.UUID, value int, at time.Time) {
	t.Helper()
	if err := db.Exec("INSERT INTO ratings (id, book_id, user_id, value, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)",
		uuid.New(), book, uuid.New(), value, at, at).Error; err != nil {
		t.Fatal(err)
	}
}

func shelve(t *testing.T, db *gorm.DB, book uuid.UUID, typ string, at time.Time) {
	t.Helper()
	shelf := uuid.New()
	if err := db.Exec("INSERT INTO shelves (id, user_id, name, type) VALUES (?, ?, ?, ?)", shelf, uuid.New(), typ, typ).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("INSERT INTO shelf_items (shelf_id, book_id, added_at) VALUES (?, ?, ?)", shelf, book, at).Error; err != nil {
		t.Fatal(err)
	}
}

// rankBy ranks the collection like the service does, with an optional cut-off.
func rankBy(since *time.Time) RankFunc {
	return func(ctx context.Context, c domain.Collection, src AutoSource) ([]domain.ScoredBook, error) {
		rule := *domain.RuleOf(&c)
		rows, err := src.AutoRows(ctx, c, since)
		if err != nil {
			return nil, err
		}
		return domain.RankAuto(rows, rule, domain.SnapshotLimit(c.Size)), nil
	}
}

func snapshot(t *testing.T, r *GormRepository, id uuid.UUID) []AdminItem {
	t.Helper()
	items, err := r.AdminItems(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return items
}

func TestRecompute_MostReadScopeAndShelves(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	now := time.Now()
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	b := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	c := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	pending := mkBook(t, db, catalogDomain.BookStatusPending, "")
	for i := 0; i < 3; i++ {
		shelve(t, db, a, "read", now)
	}
	shelve(t, db, b, "read", now)
	shelve(t, db, c, "want_to_read", now) // other shelf type
	shelve(t, db, b, "custom", now)       // custom shelf never counts
	for i := 0; i < 5; i++ {
		shelve(t, db, pending, "read", now) // unpublished never ranks
	}
	col := mkAuto(t, r, domain.MetricMostRead, 10, nil)
	if err := r.Recompute(ctx, col.ID, true, now, rankBy(nil)); err != nil {
		t.Fatal(err)
	}
	items := snapshot(t, r, col.ID)
	if len(items) != 2 || items[0].Book.ID != a || items[1].Book.ID != b {
		t.Fatalf("snapshot: %+v", items)
	}
	if items[0].Score == nil || *items[0].Score != 3 || items[0].PreviousPosition != nil {
		t.Fatalf("score/prev: %+v", items[0])
	}
	got, _ := r.GetByID(ctx, col.ID)
	if got.ComputedAt == nil || got.BaselineAt != nil {
		t.Fatalf("times: %+v", got)
	}

	wanted := mkAuto(t, r, domain.MetricMostWanted, 10, nil)
	if err := r.Recompute(ctx, wanted.ID, true, now, rankBy(nil)); err != nil {
		t.Fatal(err)
	}
	if it := snapshot(t, r, wanted.ID); len(it) != 1 || it[0].Book.ID != c {
		t.Fatalf("wanted: %+v", it)
	}
}

func TestRecompute_GenreFilterAndPeriod(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	now := time.Now()
	genre := uuid.New()
	in := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	out := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	old := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	for _, id := range []uuid.UUID{in, old} {
		if err := db.Exec("INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", id, genre).Error; err != nil {
			t.Fatal(err)
		}
	}
	shelve(t, db, in, "want_to_read", now.Add(-24*time.Hour))
	shelve(t, db, out, "want_to_read", now)
	shelve(t, db, old, "want_to_read", now.Add(-10*24*time.Hour))

	col := mkAuto(t, r, domain.MetricMostWanted, 10, &genre)
	since := now.Add(-7 * 24 * time.Hour)
	if err := r.Recompute(ctx, col.ID, true, now, rankBy(&since)); err != nil {
		t.Fatal(err)
	}
	if it := snapshot(t, r, col.ID); len(it) != 1 || it[0].Book.ID != in {
		t.Fatalf("genre+period snapshot: %+v", it)
	}
}

func TestRecompute_WeightedRatingExcludesPendingAndPeriod(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	now := time.Now()
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	pending := mkBook(t, db, catalogDomain.BookStatusPending, "")
	for i := 0; i < 3; i++ {
		rate(t, db, a, 4, now)
		rate(t, db, pending, 5, now)
	}
	rate(t, db, a, 1, now.Add(-100*24*time.Hour)) // outside the period
	col := mkAuto(t, r, domain.MetricWeightedRating, 10, nil)
	since := now.Add(-30 * 24 * time.Hour)
	if err := r.Recompute(ctx, col.ID, true, now, rankBy(&since)); err != nil {
		t.Fatal(err)
	}
	items := snapshot(t, r, col.ID)
	if len(items) != 1 || items[0].Book.ID != a {
		t.Fatalf("snapshot: %+v", items)
	}
	// C = 4 (pending ratings and the old rating do not count): score stays 4.
	if s := *items[0].Score; s < 3.999 || s > 4.001 {
		t.Fatalf("score %v", s)
	}
}

func TestRecompute_TrendingMergesRatingsAndShelvings(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	now := time.Now()
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	rate(t, db, a, 5, now)
	rate(t, db, a, 3, now)
	for i := 0; i < 3; i++ {
		shelve(t, db, a, "reading", now)
	}
	shelve(t, db, a, "custom", now)
	col := mkAuto(t, r, domain.MetricTrending, 10, nil)
	since := now.Add(-30 * 24 * time.Hour)
	if err := r.Recompute(ctx, col.ID, true, now, rankBy(&since)); err != nil {
		t.Fatal(err)
	}
	items := snapshot(t, r, col.ID)
	if len(items) != 1 || *items[0].Score != 5 {
		t.Fatalf("snapshot: %+v", items)
	}
}

func TestRecompute_BaselineAndReset(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	t0 := time.Now().Add(-48 * time.Hour)
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	b := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	shelve(t, db, a, "read", t0)
	shelve(t, db, a, "read", t0)
	shelve(t, db, b, "read", t0)
	col := mkAuto(t, r, domain.MetricMostRead, 10, nil)
	if err := r.Recompute(ctx, col.ID, true, t0, rankBy(nil)); err != nil {
		t.Fatal(err)
	}
	// b overtakes a and a new book c enters.
	c := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	shelve(t, db, c, "read", t0)
	for i := 0; i < 3; i++ {
		shelve(t, db, b, "read", t0)
	}
	t1 := time.Now()
	if err := r.Recompute(ctx, col.ID, false, t1, rankBy(nil)); err != nil {
		t.Fatal(err)
	}
	items := snapshot(t, r, col.ID)
	if len(items) != 3 || items[0].Book.ID != b || items[1].Book.ID != a || items[2].Book.ID != c {
		t.Fatalf("order: %+v", items)
	}
	if p := items[0].PreviousPosition; p == nil || *p != 2 {
		t.Fatalf("b prev: %v", p)
	}
	if p := items[1].PreviousPosition; p == nil || *p != 1 {
		t.Fatalf("a prev: %v", p)
	}
	if items[2].PreviousPosition != nil {
		t.Fatal("c is new")
	}
	got, _ := r.GetByID(ctx, col.ID)
	if got.BaselineAt == nil || !got.BaselineAt.Equal(t0) || got.ComputedAt == nil || !got.ComputedAt.Equal(t1) {
		t.Fatalf("times: computed %v baseline %v", got.ComputedAt, got.BaselineAt)
	}

	if err := r.Recompute(ctx, col.ID, true, time.Now(), rankBy(nil)); err != nil {
		t.Fatal(err)
	}
	for _, it := range snapshot(t, r, col.ID) {
		if it.PreviousPosition != nil {
			t.Fatal("reset must clear previous positions")
		}
	}
	got, _ = r.GetByID(ctx, col.ID)
	if got.BaselineAt != nil {
		t.Fatalf("reset must clear baseline: %v", got.BaselineAt)
	}
}

func TestRecompute_FailureKeepsSnapshot(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	a := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	shelve(t, db, a, "read", time.Now())
	col := mkAuto(t, r, domain.MetricMostRead, 10, nil)
	if err := r.Recompute(ctx, col.ID, true, time.Now(), rankBy(nil)); err != nil {
		t.Fatal(err)
	}
	before, _ := r.GetByID(ctx, col.ID)
	boom := errors.New("boom")
	err := r.Recompute(ctx, col.ID, false, time.Now(), func(context.Context, domain.Collection, AutoSource) ([]domain.ScoredBook, error) {
		return nil, boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err: %v", err)
	}
	after, _ := r.GetByID(ctx, col.ID)
	if it := snapshot(t, r, col.ID); len(it) != 1 || !after.ComputedAt.Equal(*before.ComputedAt) {
		t.Fatalf("snapshot changed: %+v %v", it, after.ComputedAt)
	}
}

func TestRecompute_ManualAndMissing(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	m := mkCollection(t, r, "manual", 5, false)
	if err := r.Recompute(ctx, m.ID, true, time.Now(), rankBy(nil)); !errors.Is(err, domain.ErrNotAutomatic) {
		t.Fatalf("manual: %v", err)
	}
	if err := r.Recompute(ctx, uuid.New(), true, time.Now(), rankBy(nil)); !errors.Is(err, domain.ErrCollectionNotFound) {
		t.Fatalf("missing: %v", err)
	}
	ids, err := r.AutoCollectionIDs(ctx)
	if err != nil || len(ids) != 0 {
		t.Fatalf("auto ids: %v %v", ids, err)
	}
	a := mkAuto(t, r, domain.MetricMostRead, 5, nil)
	if ids, _ = r.AutoCollectionIDs(ctx); len(ids) != 1 || ids[0] != a.ID {
		t.Fatalf("auto ids: %v", ids)
	}
}

func TestBookWritesRejectedOnAutoCollection(t *testing.T) {
	r, db := newRepo(t)
	ctx := context.Background()
	col := mkAuto(t, r, domain.MetricMostRead, 5, nil)
	b := mkBook(t, db, catalogDomain.BookStatusPublished, "")
	one := 1
	if err := r.AddBook(ctx, col.ID, b, nil); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Fatalf("AddBook: %v", err)
	}
	if _, err := r.AddBooks(ctx, col.ID, []uuid.UUID{b}, nil); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Fatalf("AddBooks: %v", err)
	}
	if err := r.MoveBook(ctx, col.ID, b, one); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Fatalf("MoveBook: %v", err)
	}
	if err := r.RemoveBook(ctx, col.ID, b); !errors.Is(err, domain.ErrCollectionAutomatic) {
		t.Fatalf("RemoveBook: %v", err)
	}
}
