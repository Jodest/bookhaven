package repository

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// collection inserts a curated collection linked to genreID (nil: no genre).
func (f *genreModFixture) collection(title, kind string, published bool, genreID *uuid.UUID) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO curated_collections (id, slug, title, kind, size, genre_id, is_published) VALUES (?, ?, ?, ?, 10, ?, ?)",
		id, "slug-"+id.String(), title, kind, genreID, published).Error; err != nil {
		f.t.Fatalf("insert collection: %v", err)
	}
	return id
}

func (f *genreModFixture) collectionGenre(id uuid.UUID) *string {
	f.t.Helper()
	var g *string
	if err := f.db.Raw("SELECT genre_id FROM curated_collections WHERE id = ?", id).Scan(&g).Error; err != nil {
		f.t.Fatalf("load collection: %v", err)
	}
	return g
}

func TestMergeGenresBatch_MergesAllSources(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	target := f.genre("роман", "roman", "published", nil)
	a := f.genre("романы", "romany", "pending", nil)
	b := f.genre("Роман (жанр)", "roman-zhanr", "pending", nil)
	c := f.genre("роман-эпопея", "roman-epopeya", "pending", nil)
	oldAlias := f.alias("романа", "romana", a, nil)
	onlyA := f.book("Только A", false, a)
	shared := f.book("A и B", false, a, b)
	inTarget := f.book("В цели", false, b, target)
	onlyC := f.book("Только C", false, c)

	res, err := f.repo.MergeGenresBatch(ctx, []uuid.UUID{a, b, c}, target, nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	// onlyA, shared and onlyC move; inTarget was already linked.
	if res.SourcesMerged != 3 || res.BooksReassigned != 3 {
		t.Fatalf("unexpected result: %+v", res)
	}
	for _, bk := range []uuid.UUID{onlyA, shared, inTarget, onlyC} {
		if links := f.links(bk); len(links) != 1 || links[0] != target.String() {
			t.Fatalf("book %s: expected one link to the target, got %v", bk, links)
		}
	}
	for _, s := range []uuid.UUID{a, b, c} {
		if row := f.load(s); row.Status != "rejected" || row.MergedIntoID == nil || *row.MergedIntoID != target.String() {
			t.Fatalf("source %s must be an alias of the target: %+v", s, row)
		}
	}
	if a := f.load(oldAlias); a.MergedIntoID == nil || *a.MergedIntoID != target.String() {
		t.Fatalf("existing alias must be re-pointed: %+v", a)
	}
	if tgt := f.load(target); tgt.Name != "роман" || tgt.Status != "published" || tgt.MergedIntoID != nil {
		t.Fatalf("target must not change: %+v", tgt)
	}
}

func TestMergeGenresBatch_FirstQIDInRequestOrderWins(t *testing.T) {
	f := newGenreModRepo(t)
	target := f.genre("роман", "roman", "published", nil)
	a := f.genre("романы", "romany", "pending", strPtr("Q1"))
	b := f.genre("роман-эпопея", "roman-epopeya", "pending", strPtr("Q2"))

	res, err := f.repo.MergeGenresBatch(context.Background(), []uuid.UUID{b, a}, target, nil)
	if err != nil {
		t.Fatalf("batch: %v", err)
	}
	if q := f.load(target).WikidataQID; q == nil || *q != "Q2" {
		t.Fatalf("target must get the first source QID in request order, got %v", q)
	}
	if q := f.load(a).WikidataQID; q == nil || *q != "Q1" {
		t.Fatalf("later source keeps its QID on the alias, got %v", q)
	}
	if f.load(b).WikidataQID != nil {
		t.Fatal("carried QID must leave the source")
	}
	if res.CarriedQIDFrom == nil || *res.CarriedQIDFrom != b || res.KeptQIDs[a] != "Q1" {
		t.Fatalf("unexpected QID report: %+v", res)
	}
}

func TestMergeGenresBatch_AliasSourceChangesNothing(t *testing.T) {
	f := newGenreModRepo(t)
	target := f.genre("роман", "roman", "published", nil)
	other := f.genre("проза", "proza", "published", nil)
	a := f.genre("романы", "romany", "pending", strPtr("Q1"))
	merged := f.alias("слитый", "slityj", other, nil)
	bk := f.book("Книга", false, a)
	col := f.collection("Подборка", "auto", true, &a)

	_, err := f.repo.MergeGenresBatch(context.Background(), []uuid.UUID{a, merged}, target, nil)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if row := f.load(a); row.MergedIntoID != nil || row.Status != "pending" || row.WikidataQID == nil {
		t.Fatalf("source must be untouched: %+v", row)
	}
	if links := f.links(bk); len(links) != 1 || links[0] != a.String() {
		t.Fatalf("links must be untouched: %v", links)
	}
	if g := f.collectionGenre(col); g == nil || *g != a.String() {
		t.Fatalf("collection must be untouched: %v", g)
	}
	if f.load(target).WikidataQID != nil {
		t.Fatal("target must be untouched")
	}
}

func TestMergeGenres_SwitchesCollections(t *testing.T) {
	ctx := context.Background()
	for _, batch := range []bool{false, true} {
		f := newGenreModRepo(t)
		target := f.genre("роман", "roman", "published", nil)
		a := f.genre("романы", "romany", "pending", nil)
		b := f.genre("эпопея", "epopeya", "pending", nil)
		other := f.genre("проза", "proza", "published", nil)
		manual := f.collection("Ручная", "manual", true, &a)
		auto := f.collection("Авто", "auto", false, &b)
		unrelated := f.collection("Чужая", "auto", true, &other)
		noGenre := f.collection("Без жанра", "manual", true, nil)

		var res GenreMergeResult
		var err error
		if batch {
			res, err = f.repo.MergeGenresBatch(ctx, []uuid.UUID{a, b}, target, nil)
		} else {
			res, err = f.repo.MergeGenres(ctx, a, target, nil)
			if err == nil {
				// the single merge leaves b's collection alone
				if g := f.collectionGenre(auto); g == nil || *g != b.String() {
					t.Fatalf("single: unrelated source collection must stay: %v", g)
				}
				auto = uuid.Nil
			}
		}
		if err != nil {
			t.Fatalf("batch=%v: %v", batch, err)
		}
		if g := f.collectionGenre(manual); g == nil || *g != target.String() {
			t.Fatalf("batch=%v: manual collection must follow the merge: %v", batch, g)
		}
		if g := f.collectionGenre(unrelated); g == nil || *g != other.String() {
			t.Fatalf("batch=%v: unrelated collection must stay: %v", batch, g)
		}
		if g := f.collectionGenre(noGenre); g != nil {
			t.Fatalf("batch=%v: collection without genre must stay: %v", batch, g)
		}
		if batch {
			if g := f.collectionGenre(auto); g == nil || *g != target.String() {
				t.Fatalf("auto collection must follow the merge: %v", g)
			}
			if res.CollectionsSwitched != 2 || len(res.AutoCollectionIDs) != 1 || res.AutoCollectionIDs[0] != auto {
				t.Fatalf("unexpected batch result: %+v", res)
			}
		} else if res.CollectionsSwitched != 1 || len(res.AutoCollectionIDs) != 0 {
			t.Fatalf("unexpected single result: %+v", res)
		}
	}
}

func TestPreviewGenreMerge(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	target := f.genre("роман", "roman", "published", nil)
	a := f.genre("романы", "romany", "pending", nil)
	b := f.genre("эпопея", "epopeya", "pending", nil)
	f.book("1", false, a)
	f.book("2", false, a, target)
	f.book("3", false, a, b)
	f.book("4", false, b, target)
	colB := f.collection("Яблоки", "auto", true, &b)
	colA := f.collection("Арбузы", "manual", false, &a)
	f.collection("Чужая", "manual", true, &target)

	preview, err := f.repo.PreviewGenreMerge(ctx, []uuid.UUID{a, b}, target)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	// books 1 and 3 would move; 2 and 4 are already in the target.
	if preview.BooksReassigned != 2 {
		t.Fatalf("expected 2 books, got %d", preview.BooksReassigned)
	}
	if len(preview.Collections) != 2 {
		t.Fatalf("expected 2 collections, got %+v", preview.Collections)
	}
	first, second := preview.Collections[0], preview.Collections[1]
	if first.ID != colA || first.Title != "Арбузы" || first.SourceGenreName != "романы" || first.SourceGenreID != a || first.Kind != "manual" || first.IsPublished ||
		second.ID != colB || second.SourceGenreName != "эпопея" || second.Kind != "auto" || !second.IsPublished {
		t.Fatalf("unexpected collections: %+v", preview.Collections)
	}

	// Nothing changed: the count equals what the merge then reports.
	if row := f.load(a); row.MergedIntoID != nil {
		t.Fatal("preview must not change genres")
	}
	if g := f.collectionGenre(colA); g == nil || *g != a.String() {
		t.Fatal("preview must not change collections")
	}
	res, err := f.repo.MergeGenresBatch(ctx, []uuid.UUID{a, b}, target, nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if res.BooksReassigned != preview.BooksReassigned {
		t.Fatalf("preview %d != merge %d", preview.BooksReassigned, res.BooksReassigned)
	}
}

func TestPreviewGenreMerge_NotFoundAndEmptyCollections(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	target := f.genre("роман", "roman", "published", nil)
	a := f.genre("романы", "romany", "pending", nil)
	merged := f.alias("слитый", "slityj", target, nil)

	if _, err := f.repo.PreviewGenreMerge(ctx, []uuid.UUID{a, merged}, target); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("alias source: expected not found, got %v", err)
	}
	if _, err := f.repo.PreviewGenreMerge(ctx, []uuid.UUID{uuid.New()}, target); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing source: expected not found, got %v", err)
	}
	preview, err := f.repo.PreviewGenreMerge(ctx, []uuid.UUID{a}, target)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.BooksReassigned != 0 || preview.Collections == nil || len(preview.Collections) != 0 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
}
