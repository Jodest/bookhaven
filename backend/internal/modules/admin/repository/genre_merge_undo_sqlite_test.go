package repository

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// genreMergeJournalDDL mirrors migration 000033 (without FKs to genres) plus
// the minimal users table the history joins.
const genreMergeJournalDDL = `
CREATE TABLE users (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', email TEXT NOT NULL DEFAULT '');
CREATE TABLE genre_merges (
	id TEXT PRIMARY KEY,
	target_id TEXT NOT NULL,
	target_name TEXT NOT NULL,
	books_reassigned INTEGER NOT NULL,
	collections_switched INTEGER NOT NULL,
	performed_by TEXT REFERENCES users(id) ON DELETE SET NULL,
	created_at DATETIME NOT NULL,
	undone_at DATETIME,
	undone_by TEXT REFERENCES users(id) ON DELETE SET NULL
);
CREATE TABLE genre_merge_sources (
	merge_id TEXT NOT NULL REFERENCES genre_merges(id) ON DELETE CASCADE,
	source_id TEXT NOT NULL,
	position INTEGER NOT NULL,
	source_name TEXT NOT NULL,
	prev_status TEXT NOT NULL,
	prev_qid TEXT,
	qid_carried BOOLEAN NOT NULL DEFAULT 0,
	PRIMARY KEY (merge_id, source_id)
);
CREATE TABLE genre_merge_items (
	merge_id TEXT NOT NULL,
	source_id TEXT NOT NULL,
	kind TEXT NOT NULL CHECK (kind IN ('book_moved', 'book_linked', 'alias', 'collection')),
	ref_id TEXT NOT NULL,
	PRIMARY KEY (merge_id, source_id, kind, ref_id),
	FOREIGN KEY (merge_id, source_id) REFERENCES genre_merge_sources (merge_id, source_id) ON DELETE CASCADE
);
`

func (f *genreModFixture) user(name, email string) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO users (id, name, email) VALUES (?, ?, ?)", id, name, email).Error; err != nil {
		f.t.Fatalf("insert user: %v", err)
	}
	return id
}

// items returns the sorted ref ids of one kind recorded for a source.
func (f *genreModFixture) items(merge, source uuid.UUID, kind string) []string {
	f.t.Helper()
	var ids []string
	if err := f.db.Raw("SELECT ref_id FROM genre_merge_items WHERE merge_id = ? AND source_id = ? AND kind = ?",
		merge, source, kind).Scan(&ids).Error; err != nil {
		f.t.Fatalf("load items: %v", err)
	}
	sort.Strings(ids)
	return ids
}

func ids(list ...uuid.UUID) []string {
	out := make([]string, 0, len(list))
	for _, id := range list {
		out = append(out, id.String())
	}
	sort.Strings(out)
	return out
}

func sameStrings(a, b []string) bool {
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

func (f *genreModFixture) count(query string, args ...any) int {
	f.t.Helper()
	var n int
	if err := f.db.Raw(query, args...).Scan(&n).Error; err != nil {
		f.t.Fatalf("count: %v", err)
	}
	return n
}

func (f *genreModFixture) setCollectionGenre(id, genre uuid.UUID) {
	f.t.Helper()
	if err := f.db.Exec("UPDATE curated_collections SET genre_id = ? WHERE id = ?", genre, id).Error; err != nil {
		f.t.Fatalf("update collection: %v", err)
	}
}

func (f *genreModFixture) unlink(book, genre uuid.UUID) {
	f.t.Helper()
	if err := f.db.Exec("DELETE FROM book_genres WHERE book_id = ? AND genre_id = ?", book, genre).Error; err != nil {
		f.t.Fatalf("unlink: %v", err)
	}
}

func (f *genreModFixture) deleteBook(book uuid.UUID) {
	f.t.Helper()
	if err := f.db.Exec("DELETE FROM book_genres WHERE book_id = ?", book).Error; err != nil {
		f.t.Fatalf("delete links: %v", err)
	}
	if err := f.db.Exec("DELETE FROM books WHERE id = ?", book).Error; err != nil {
		f.t.Fatalf("delete book: %v", err)
	}
}

func (f *genreModFixture) setMergeTime(id uuid.UUID, at time.Time) {
	f.t.Helper()
	if err := f.db.Exec("UPDATE genre_merges SET created_at = ? WHERE id = ?", at, id).Error; err != nil {
		f.t.Fatalf("set merge time: %v", err)
	}
}

func (f *genreModFixture) mustMerge(sources []uuid.UUID, target uuid.UUID, actor *uuid.UUID) GenreMergeResult {
	f.t.Helper()
	res, err := f.repo.MergeGenresBatch(context.Background(), sources, target, actor)
	if err != nil {
		f.t.Fatalf("merge: %v", err)
	}
	return res
}

func blockedCode(t *testing.T, err error) string {
	t.Helper()
	var blocked *GenreMergeUndoBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("expected a blocked error, got %v", err)
	}
	return blocked.Code
}

func TestGenreMergeJournal_SingleMergeWritesOneEntry(t *testing.T) {
	f := newGenreModRepo(t)
	qid := "Q1"
	source := f.genre("романы", "romany", "pending", &qid)
	target := f.genre("роман", "roman", "published", nil)
	al := f.alias("романа", "romana", source, nil)
	b1 := f.book("b1", false, source)
	b2 := f.book("b2", false, source, target)
	col := f.collection("Лучшие", "auto", true, &source)
	admin := f.user("Админ", "a@x.ru")

	res, err := f.repo.MergeGenres(context.Background(), source, target, &admin)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if res.MergeID == uuid.Nil {
		t.Fatal("expected a merge id")
	}
	if n := f.count("SELECT COUNT(*) FROM genre_merges"); n != 1 {
		t.Fatalf("entries = %d, want 1", n)
	}
	var head struct {
		ID                  string
		TargetName          string
		BooksReassigned     int
		CollectionsSwitched int
		PerformedBy         *string
	}
	if err := f.db.Raw("SELECT id, target_name, books_reassigned, collections_switched, performed_by FROM genre_merges").Scan(&head).Error; err != nil {
		t.Fatal(err)
	}
	if head.ID != res.MergeID.String() || head.TargetName != "роман" || head.BooksReassigned != 1 || head.CollectionsSwitched != 1 ||
		head.PerformedBy == nil || *head.PerformedBy != admin.String() {
		t.Fatalf("unexpected header: %+v", head)
	}
	var src genreMergeSourceRecord
	if err := f.db.Where("merge_id = ?", res.MergeID).First(&src).Error; err != nil {
		t.Fatal(err)
	}
	if src.SourceID != source || src.Position != 0 || src.SourceName != "романы" || src.PrevStatus != "pending" ||
		src.PrevQID == nil || *src.PrevQID != "Q1" || !src.QIDCarried {
		t.Fatalf("unexpected source record: %+v", src)
	}
	for kind, want := range map[string][]string{
		"book_moved":  ids(b1),
		"book_linked": ids(b2),
		"alias":       ids(al),
		"collection":  ids(col),
	} {
		if got := f.items(res.MergeID, source, kind); !sameStrings(got, want) {
			t.Fatalf("%s items = %v, want %v", kind, got, want)
		}
	}
}

func TestGenreMergeJournal_BatchIsOneEntryAndSharedBookIsPerSource(t *testing.T) {
	f := newGenreModRepo(t)
	a := f.genre("a", "a", "pending", nil)
	b := f.genre("b", "b", "published", nil)
	target := f.genre("t", "t", "published", nil)
	shared := f.book("shared", false, a, b)

	res := f.mustMerge([]uuid.UUID{a, b}, target, nil)
	if n := f.count("SELECT COUNT(*) FROM genre_merges"); n != 1 {
		t.Fatalf("entries = %d, want 1", n)
	}
	var positions []string
	if err := f.db.Raw("SELECT source_id FROM genre_merge_sources WHERE merge_id = ? ORDER BY position", res.MergeID).Scan(&positions).Error; err != nil {
		t.Fatal(err)
	}
	if len(positions) != 2 || positions[0] != a.String() || positions[1] != b.String() {
		t.Fatalf("sources in order = %v", positions)
	}
	if got := f.items(res.MergeID, a, "book_moved"); !sameStrings(got, ids(shared)) {
		t.Fatalf("A moved = %v", got)
	}
	if got := f.items(res.MergeID, b, "book_linked"); !sameStrings(got, ids(shared)) {
		t.Fatalf("B linked = %v", got)
	}
	if got := f.items(res.MergeID, b, "book_moved"); len(got) != 0 {
		t.Fatalf("B moved = %v, want none", got)
	}
	var performer *string
	if err := f.db.Raw("SELECT performed_by FROM genre_merges").Scan(&performer).Error; err != nil || performer != nil {
		t.Fatalf("performed_by = %v (%v), want NULL without actor", performer, err)
	}
}

func TestGenreMergeJournal_FailedMergeWritesNothing(t *testing.T) {
	f := newGenreModRepo(t)
	target := f.genre("t", "t", "published", nil)
	a := f.genre("a", "a", "pending", nil)
	other := f.genre("o", "o", "published", nil)
	merged := f.alias("m", "m", other, nil)
	if _, err := f.repo.MergeGenresBatch(context.Background(), []uuid.UUID{a, merged}, target, nil); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	for _, table := range []string{"genre_merges", "genre_merge_sources", "genre_merge_items"} {
		if n := f.count("SELECT COUNT(*) FROM " + table); n != 0 {
			t.Fatalf("%s has %d rows after a failed merge", table, n)
		}
	}
}

func TestUndoGenreMerge_SingleRoundTrip(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	qid := "Q1"
	source := f.genre("романы", "romany", "pending", &qid)
	target := f.genre("роман", "roman", "published", nil)
	al := f.alias("романа", "romana", source, nil)
	moved := f.book("moved", false, source)
	both := f.book("both", false, source, target)
	own := f.book("own", false, target)
	manual := f.collection("Ручная", "manual", true, &source)
	auto := f.collection("Авто", "auto", true, &source)
	admin := f.user("", "admin@x.ru")

	merge := f.mustMerge([]uuid.UUID{source}, target, &admin)
	if got := f.load(target).WikidataQID; got == nil || *got != "Q1" {
		t.Fatalf("QID was not carried: %v", got)
	}

	preview, err := f.repo.PreviewGenreMergeUndo(ctx, merge.MergeID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.BooksReturned != 1 || preview.BooksRelinked != 1 || len(preview.Collections) != 2 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	if preview.Collections[0].Title != "Авто" || preview.Collections[0].SourceGenreID != source || preview.Collections[0].SourceGenreName != "романы" {
		t.Fatalf("unexpected preview collections: %+v", preview.Collections)
	}
	if f.load(source).MergedIntoID == nil {
		t.Fatal("preview must not change anything")
	}

	res, err := f.repo.UndoGenreMerge(ctx, merge.MergeID, admin)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if res.TargetID != target || res.SourcesRestored != 1 || res.BooksReturned != preview.BooksReturned ||
		res.BooksRelinked != preview.BooksRelinked || res.CollectionsSwitchedBack != 2 ||
		len(res.AutoCollectionIDs) != 1 || res.AutoCollectionIDs[0] != auto {
		t.Fatalf("unexpected result: %+v", res)
	}
	src := f.load(source)
	if src.MergedIntoID != nil || src.Status != "pending" || src.Name != "романы" || src.Slug != "romany" ||
		src.WikidataQID == nil || *src.WikidataQID != "Q1" {
		t.Fatalf("source not restored: %+v", src)
	}
	if got := f.load(target).WikidataQID; got != nil {
		t.Fatalf("target kept the carried QID: %v", *got)
	}
	if got := f.load(al).MergedIntoID; got == nil || *got != source.String() {
		t.Fatalf("alias not re-pointed to the source: %v", got)
	}
	if got := f.links(moved); !sameStrings(got, ids(source)) {
		t.Fatalf("moved book links = %v", got)
	}
	if got := f.links(both); !sameStrings(got, ids(source, target)) {
		t.Fatalf("shared book links = %v", got)
	}
	if got := f.links(own); !sameStrings(got, ids(target)) {
		t.Fatalf("own book links = %v", got)
	}
	for _, c := range []uuid.UUID{manual, auto} {
		if g := f.collectionGenre(c); g == nil || *g != source.String() {
			t.Fatalf("collection %s not switched back: %v", c, g)
		}
	}
	var undoneBy *string
	if err := f.db.Raw("SELECT undone_by FROM genre_merges WHERE id = ? AND undone_at IS NOT NULL", merge.MergeID).Scan(&undoneBy).Error; err != nil ||
		undoneBy == nil || *undoneBy != admin.String() {
		t.Fatalf("undone marker = %v (%v)", undoneBy, err)
	}
	if _, err := f.repo.UndoGenreMerge(ctx, merge.MergeID, admin); blockedCode(t, err) != GenreMergeUndoAlreadyUndone {
		t.Fatal("second undo must be already_undone")
	}
}

func TestUndoGenreMerge_BatchRoundTripWithSharedBook(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	a := f.genre("a", "a", "pending", nil)
	b := f.genre("b", "b", "published", nil)
	target := f.genre("t", "t", "published", nil)
	shared := f.book("shared", false, a, b)
	onlyA := f.book("onlyA", false, a)
	onlyB := f.book("onlyB", false, b)
	admin := f.user("Админ", "a@x.ru")

	merge := f.mustMerge([]uuid.UUID{a, b}, target, &admin)
	res, err := f.repo.UndoGenreMerge(ctx, merge.MergeID, admin)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if res.SourcesRestored != 2 || res.BooksReturned != 3 || res.BooksRelinked != 1 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if f.load(a).MergedIntoID != nil || f.load(a).Status != "pending" || f.load(b).MergedIntoID != nil || f.load(b).Status != "published" {
		t.Fatal("sources not restored")
	}
	if got := f.links(shared); !sameStrings(got, ids(a, b)) {
		t.Fatalf("shared links = %v, want both sources and not the target", got)
	}
	if got := f.links(onlyA); !sameStrings(got, ids(a)) {
		t.Fatalf("onlyA links = %v", got)
	}
	if got := f.links(onlyB); !sameStrings(got, ids(b)) {
		t.Fatalf("onlyB links = %v", got)
	}
}

func TestUndoGenreMerge_SkipsWhatChanged(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	source := f.genre("романы", "romany", "pending", nil)
	target := f.genre("роман", "roman", "published", nil)
	other := f.genre("проза", "proza", "published", nil)
	unlinked := f.book("unlinked", false, source)
	deleted := f.book("deleted", false, source)
	deletedLinked := f.book("deletedLinked", false, source, target)
	kept := f.book("kept", false, source)
	repointed := f.collection("Перенастроена", "manual", true, &source)
	removed := f.collection("Удалена", "manual", true, &source)
	still := f.collection("Осталась", "manual", true, &source)
	admin := f.user("Админ", "a@x.ru")

	merge := f.mustMerge([]uuid.UUID{source}, target, &admin)
	f.unlink(unlinked, target)
	f.deleteBook(deleted)
	f.deleteBook(deletedLinked)
	f.setCollectionGenre(repointed, other)
	if err := f.db.Exec("DELETE FROM curated_collections WHERE id = ?", removed).Error; err != nil {
		t.Fatal(err)
	}

	preview, err := f.repo.PreviewGenreMergeUndo(ctx, merge.MergeID)
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	res, err := f.repo.UndoGenreMerge(ctx, merge.MergeID, admin)
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	if res.BooksReturned != 1 || res.BooksRelinked != 0 || res.CollectionsSwitchedBack != 1 ||
		preview.BooksReturned != res.BooksReturned || len(preview.Collections) != 1 || preview.Collections[0].ID != still {
		t.Fatalf("unexpected result %+v / preview %+v", res, preview)
	}
	if got := f.links(unlinked); len(got) != 0 {
		t.Fatalf("unlinked book links = %v, want none", got)
	}
	if got := f.links(kept); !sameStrings(got, ids(source)) {
		t.Fatalf("kept links = %v", got)
	}
	if g := f.collectionGenre(repointed); g == nil || *g != other.String() {
		t.Fatalf("re-pointed collection changed: %v", g)
	}
	if g := f.collectionGenre(still); g == nil || *g != source.String() {
		t.Fatalf("collection not switched back: %v", g)
	}
}

func TestUndoGenreMerge_LaterMergeKeepsTargetLink(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	romany := f.genre("романы", "romany", "published", nil)
	novelly := f.genre("новеллы", "novelly", "published", nil)
	target := f.genre("роман", "roman", "published", nil)
	book := f.book("book", false, romany, novelly)
	admin := f.user("Админ", "a@x.ru")

	// Merge 1 moves the book to the target; merge 2 then drops the book's
	// "новеллы" link because it is already in the target.
	m1 := f.mustMerge([]uuid.UUID{romany}, target, &admin)
	f.setMergeTime(m1.MergeID, f.base)
	m2 := f.mustMerge([]uuid.UUID{novelly}, target, &admin)
	f.setMergeTime(m2.MergeID, f.base.Add(time.Hour))

	if _, err := f.repo.UndoGenreMerge(ctx, m1.MergeID, admin); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := f.links(book); !sameStrings(got, ids(romany, target)) {
		t.Fatalf("links = %v, want романы and роман", got)
	}
	// Merge 2 stays undoable and restores the dropped link; the book stays
	// in the target, where it already was when merge 2 ran.
	if _, err := f.repo.UndoGenreMerge(ctx, m2.MergeID, admin); err != nil {
		t.Fatalf("undo of the later merge: %v", err)
	}
	if got := f.links(book); !sameStrings(got, ids(romany, novelly, target)) {
		t.Fatalf("links after both undos = %v, want романы, новеллы and роман", got)
	}
}

func TestUndoGenreMerge_BlockedLeavesEverythingUnchanged(t *testing.T) {
	type setup func(f *genreModFixture, source, target uuid.UUID)
	qid := "Q7"
	cases := []struct {
		name  string
		qid   *string
		setup setup
		code  string
	}{
		{"target missing", nil, func(f *genreModFixture, source, target uuid.UUID) {
			if err := f.db.Exec("PRAGMA foreign_keys = OFF").Error; err != nil {
				t.Fatal(err)
			}
			if err := f.db.Exec("UPDATE genres SET merged_into_id = NULL, status = 'pending' WHERE id = ?", source).Error; err != nil {
				t.Fatal(err)
			}
			if err := f.db.Exec("DELETE FROM genres WHERE id = ?", target).Error; err != nil {
				t.Fatal(err)
			}
		}, GenreMergeUndoTargetMissing},
		{"target merged", nil, func(f *genreModFixture, source, target uuid.UUID) {
			proza := f.genre("проза", "proza", "published", nil)
			f.mustMerge([]uuid.UUID{target}, proza, nil)
		}, GenreMergeUndoTargetMerged},
		{"source missing", nil, func(f *genreModFixture, source, target uuid.UUID) {
			if err := f.db.Exec("DELETE FROM genres WHERE id = ?", source).Error; err != nil {
				t.Fatal(err)
			}
		}, GenreMergeUndoSourceMissing},
		{"source not alias", nil, func(f *genreModFixture, source, target uuid.UUID) {
			other := f.genre("проза", "proza", "published", nil)
			if err := f.db.Exec("UPDATE genres SET merged_into_id = ? WHERE id = ?", other, source).Error; err != nil {
				t.Fatal(err)
			}
		}, GenreMergeUndoSourceNotAlias},
		{"qid conflict", &qid, func(f *genreModFixture, source, target uuid.UUID) {
			if err := f.db.Exec("UPDATE genres SET wikidata_qid = NULL WHERE id = ?", target).Error; err != nil {
				t.Fatal(err)
			}
			third := f.genre("третий", "third", "published", &qid)
			_ = third
		}, GenreMergeUndoQIDConflict},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newGenreModRepo(t)
			ctx := context.Background()
			source := f.genre("романы", "romany", "pending", tc.qid)
			target := f.genre("роман", "roman", "published", nil)
			book := f.book("book", false, source)
			admin := f.user("Админ", "a@x.ru")
			merge := f.mustMerge([]uuid.UUID{source}, target, &admin)
			tc.setup(f, source, target)

			linksBefore := f.links(book)
			if _, err := f.repo.PreviewGenreMergeUndo(ctx, merge.MergeID); blockedCode(t, err) != tc.code {
				t.Fatalf("preview: expected %s, got %v", tc.code, err)
			}
			if _, err := f.repo.UndoGenreMerge(ctx, merge.MergeID, admin); blockedCode(t, err) != tc.code {
				t.Fatalf("undo: expected %s, got %v", tc.code, err)
			}
			if got := f.links(book); !sameStrings(got, linksBefore) {
				t.Fatalf("links changed by a blocked undo: %v -> %v", linksBefore, got)
			}
			if n := f.count("SELECT COUNT(*) FROM genre_merges WHERE undone_at IS NOT NULL"); n != 0 {
				t.Fatal("a blocked undo marked the entry undone")
			}
		})
	}
}

func TestUndoGenreMerge_UnknownEntry(t *testing.T) {
	f := newGenreModRepo(t)
	admin := f.user("Админ", "a@x.ru")
	if _, err := f.repo.UndoGenreMerge(context.Background(), uuid.New(), admin); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("undo: expected not found, got %v", err)
	}
	if _, err := f.repo.PreviewGenreMergeUndo(context.Background(), uuid.New()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("preview: expected not found, got %v", err)
	}
}

func TestUndoGenreMerge_CarriedQIDWhenNobodyHoldsIt(t *testing.T) {
	f := newGenreModRepo(t)
	qid := "Q9"
	source := f.genre("романы", "romany", "pending", &qid)
	target := f.genre("роман", "roman", "published", nil)
	admin := f.user("Админ", "a@x.ru")
	merge := f.mustMerge([]uuid.UUID{source}, target, &admin)
	// The target lost the carried QID after the merge.
	if err := f.db.Exec("UPDATE genres SET wikidata_qid = NULL WHERE id = ?", target).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := f.repo.UndoGenreMerge(context.Background(), merge.MergeID, admin); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := f.load(source).WikidataQID; got == nil || *got != "Q9" {
		t.Fatalf("source QID = %v, want Q9", got)
	}
}

func TestUndoGenreMerge_KeptQIDStaysOnSource(t *testing.T) {
	f := newGenreModRepo(t)
	srcQID, tgtQID := "Q1", "Q2"
	source := f.genre("романы", "romany", "pending", &srcQID)
	target := f.genre("роман", "roman", "published", &tgtQID)
	admin := f.user("Админ", "a@x.ru")
	merge := f.mustMerge([]uuid.UUID{source}, target, &admin)
	if _, err := f.repo.UndoGenreMerge(context.Background(), merge.MergeID, admin); err != nil {
		t.Fatalf("undo: %v", err)
	}
	if got := f.load(source).WikidataQID; got == nil || *got != "Q1" {
		t.Fatalf("source QID = %v", got)
	}
	if got := f.load(target).WikidataQID; got == nil || *got != "Q2" {
		t.Fatalf("target QID = %v", got)
	}
}

func TestListGenreMerges_OrderPagingAndNames(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	target := f.genre("t", "t", "published", nil)
	named := f.user("Админ", "named@x.ru")
	unnamed := f.user("", "unnamed@x.ru")

	var merges []uuid.UUID
	for i, name := range []string{"a", "b", "c"} {
		src := f.genre(name, name, "pending", nil)
		actor := &named
		if i == 1 {
			actor = &unnamed
		}
		m := f.mustMerge([]uuid.UUID{src}, target, actor)
		f.setMergeTime(m.MergeID, f.base.Add(time.Duration(i)*time.Hour))
		merges = append(merges, m.MergeID)
	}
	// An older merge without a performer.
	extra := f.genre("d", "d", "pending", nil)
	old := f.mustMerge([]uuid.UUID{extra}, target, nil)
	f.setMergeTime(old.MergeID, f.base.Add(-time.Hour))

	if _, err := f.repo.UndoGenreMerge(ctx, merges[2], named); err != nil {
		t.Fatalf("undo: %v", err)
	}

	page1, total, err := f.repo.ListGenreMerges(ctx, 1, 2)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 4 || len(page1) != 2 || page1[0].ID != merges[2] || page1[1].ID != merges[1] {
		t.Fatalf("unexpected first page (total %d): %+v", total, page1)
	}
	if page1[0].UndoneAt == nil || page1[0].UndoneBy == nil || page1[0].UndoneBy.Name != "Админ" {
		t.Fatalf("undone entry: %+v", page1[0])
	}
	if page1[1].UndoneAt != nil || page1[1].UndoneBy != nil || page1[1].PerformedBy == nil || page1[1].PerformedBy.Name != "unnamed@x.ru" {
		t.Fatalf("e-mail fallback: %+v", page1[1])
	}
	if len(page1[0].Sources) != 1 || page1[0].Sources[0].Name != "c" || page1[0].Target.Name != "t" {
		t.Fatalf("sources/target: %+v", page1[0])
	}
	page2, _, err := f.repo.ListGenreMerges(ctx, 2, 2)
	if err != nil {
		t.Fatalf("list page 2: %v", err)
	}
	if len(page2) != 2 || page2[0].ID != merges[0] || page2[1].ID != old.MergeID || page2[1].PerformedBy != nil {
		t.Fatalf("unexpected second page: %+v", page2)
	}
}
