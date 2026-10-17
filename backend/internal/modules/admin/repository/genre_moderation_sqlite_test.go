package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// genreModerationDDL recreates the genres/books/book_genres shape of
// migrations 000017 + 000018 (merged_into_id with its CHECKs, the unique
// wikidata_qid partial index) with SQLite-native types, plus the tables
// DashboardCounts touches. Search (ILIKE) is Postgres-only and not
// exercised here.
const genreModerationDDL = `
CREATE TABLE genres (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	status TEXT NOT NULL DEFAULT 'pending',
	wikidata_qid TEXT,
	merged_into_id TEXT REFERENCES genres(id),
	created_at DATETIME,
	CHECK (merged_into_id IS NULL OR merged_into_id <> id),
	CHECK (merged_into_id IS NULL OR status = 'rejected')
);
CREATE UNIQUE INDEX idx_genres_wikidata_qid ON genres(wikidata_qid) WHERE wikidata_qid IS NOT NULL;
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, deleted_at DATETIME);
CREATE TABLE book_genres (book_id TEXT NOT NULL, genre_id TEXT NOT NULL, PRIMARY KEY (book_id, genre_id));
CREATE TABLE authors (id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'published');
CREATE TABLE reviews (id TEXT PRIMARY KEY, is_hidden BOOLEAN NOT NULL DEFAULT 0, deleted_at DATETIME);
CREATE TABLE curated_collections (id TEXT PRIMARY KEY, slug TEXT NOT NULL UNIQUE, title TEXT NOT NULL, description TEXT NOT NULL DEFAULT '', kind TEXT NOT NULL DEFAULT 'manual', size INTEGER NOT NULL, source TEXT, source_url TEXT, genre_id TEXT, is_published BOOLEAN NOT NULL DEFAULT 0, published_at DATETIME, rule_metric TEXT, rule_period_days INTEGER, rule_min_ratings INTEGER, rule_confidence INTEGER, computed_at DATETIME, baseline_at DATETIME, created_at DATETIME, updated_at DATETIME);
`

type genreModFixture struct {
	t    *testing.T
	repo *GormRepository
	db   *gorm.DB
	base time.Time
	n    int
}

func newGenreModRepo(t *testing.T) *genreModFixture {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// One connection: every query (and the merge transaction) sees the
	// same in-memory database.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(genreModerationDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &genreModFixture{t: t, repo: &GormRepository{db: db}, db: db, base: time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)}
}

// genre inserts a genre; each call is one minute newer than the last.
func (f *genreModFixture) genre(name, slug, status string, qid *string) uuid.UUID {
	f.t.Helper()
	f.n++
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO genres (id, name, slug, status, wikidata_qid, created_at) VALUES (?, ?, ?, ?, ?, ?)",
		id, name, slug, status, qid, f.base.Add(time.Duration(f.n)*time.Minute)).Error; err != nil {
		f.t.Fatalf("insert genre: %v", err)
	}
	return id
}

// alias inserts a rejected alias row of target.
func (f *genreModFixture) alias(name, slug string, target uuid.UUID, qid *string) uuid.UUID {
	f.t.Helper()
	f.n++
	id := uuid.New()
	if err := f.db.Exec("INSERT INTO genres (id, name, slug, status, wikidata_qid, merged_into_id, created_at) VALUES (?, ?, ?, 'rejected', ?, ?, ?)",
		id, name, slug, qid, target, f.base.Add(time.Duration(f.n)*time.Minute)).Error; err != nil {
		f.t.Fatalf("insert alias: %v", err)
	}
	return id
}

func (f *genreModFixture) book(title string, hidden bool, genreIDs ...uuid.UUID) uuid.UUID {
	f.t.Helper()
	id := uuid.New()
	status := catalogDomain.BookStatusPublished
	if hidden {
		status = catalogDomain.BookStatusPending
	}
	if err := f.db.Exec("INSERT INTO books (id, title, status) VALUES (?, ?, ?)", id, title, status).Error; err != nil {
		f.t.Fatalf("insert book: %v", err)
	}
	for _, g := range genreIDs {
		if err := f.db.Exec("INSERT INTO book_genres (book_id, genre_id) VALUES (?, ?)", id, g).Error; err != nil {
			f.t.Fatalf("link book: %v", err)
		}
	}
	return id
}

type genreRawRow struct {
	Name         string
	Slug         string
	Status       string
	WikidataQID  *string `gorm:"column:wikidata_qid"`
	MergedIntoID *string `gorm:"column:merged_into_id"`
}

func (f *genreModFixture) load(id uuid.UUID) genreRawRow {
	f.t.Helper()
	var row genreRawRow
	if err := f.db.Raw("SELECT name, slug, status, wikidata_qid, merged_into_id FROM genres WHERE id = ?", id).Scan(&row).Error; err != nil {
		f.t.Fatalf("load genre: %v", err)
	}
	return row
}

// links returns the genre ids linked to bookID.
func (f *genreModFixture) links(bookID uuid.UUID) []string {
	f.t.Helper()
	var ids []string
	if err := f.db.Raw("SELECT genre_id FROM book_genres WHERE book_id = ? ORDER BY genre_id", bookID).Scan(&ids).Error; err != nil {
		f.t.Fatalf("load links: %v", err)
	}
	return ids
}

func genreRowIDs(rows []GenreModerationRow) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.Genre.ID)
	}
	return out
}

func TestListGenresForModeration_FiltersCountsAndOrder(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()

	noBooks := f.genre("без книг", "bez-knig", "pending", nil)
	onlyHidden := f.genre("только скрытые", "tolko-skrytye", "pending", strPtr("Q1"))
	mixed := f.genre("смешанный", "smeshannyy", "pending", nil)
	published := f.genre("опубликованный", "opublikovannyy", "published", strPtr("Q2"))
	f.genre("отклонённый", "otklonennyy", "rejected", nil)
	f.alias("алиас", "alias", mixed, nil)

	f.book("Скрытая 1", true, onlyHidden)
	f.book("Скрытая 2", true, onlyHidden, mixed)
	f.book("Видимая", false, mixed)
	f.book("Чужая", false, published)

	rows, total, err := f.repo.ListGenresForModeration(ctx, GenreModerationFilter{Status: "pending", Limit: 20})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Fatalf("expected 3 pending, got %d (total %d)", len(rows), total)
	}
	if got := genreRowIDs(rows); got[0] != mixed || got[1] != onlyHidden || got[2] != noBooks {
		t.Fatalf("unexpected order (newest first): %v", got)
	}
	counts := map[uuid.UUID][2]int64{}
	for _, r := range rows {
		counts[r.Genre.ID] = [2]int64{r.BooksTotal, r.BooksVisible}
	}
	if counts[noBooks] != [2]int64{0, 0} || counts[onlyHidden] != [2]int64{2, 0} || counts[mixed] != [2]int64{2, 1} {
		t.Fatalf("unexpected counts: %v", counts)
	}
	if q := rows[1].Genre.WikidataQID; q == nil || *q != "Q1" || rows[1].Genre.Slug != "tolko-skrytye" || rows[1].Genre.CreatedAt.IsZero() {
		t.Fatalf("unexpected mapped genre: %+v", rows[1].Genre)
	}

	with, _, err := f.repo.ListGenresForModeration(ctx, GenreModerationFilter{Status: "pending", QID: "with"})
	if err != nil || len(with) != 1 || with[0].Genre.ID != onlyHidden {
		t.Fatalf("qid=with: %v err=%v", genreRowIDs(with), err)
	}
	without, _, err := f.repo.ListGenresForModeration(ctx, GenreModerationFilter{Status: "pending", QID: "without"})
	if err != nil || len(without) != 2 {
		t.Fatalf("qid=without: %v err=%v", genreRowIDs(without), err)
	}

	// The rejected tab lists the rejected genre but never the alias row.
	rejected, total, err := f.repo.ListGenresForModeration(ctx, GenreModerationFilter{Status: "rejected"})
	if err != nil || total != 1 || len(rejected) != 1 || rejected[0].Genre.Name != "отклонённый" {
		t.Fatalf("rejected tab: %+v total=%d err=%v", rejected, total, err)
	}
	all, total, err := f.repo.ListGenresForModeration(ctx, GenreModerationFilter{})
	if err != nil || total != 5 || len(all) != 5 {
		t.Fatalf("expected 5 non-alias genres, got %d (total %d) err=%v", len(all), total, err)
	}
}

func TestListGenreSampleBooks_AtMostThreeByTitle(t *testing.T) {
	f := newGenreModRepo(t)
	g := f.genre("роман", "roman", "pending", nil)
	other := f.genre("повесть", "povest", "pending", nil)
	d := f.book("Г", false, g)
	f.book("Б", true, g)
	f.book("В", false, g)
	a := f.book("А", false, g, other)

	samples, err := f.repo.ListGenreSampleBooks(context.Background(), []uuid.UUID{g, other})
	if err != nil {
		t.Fatalf("samples: %v", err)
	}
	got := samples[g]
	if len(got) != 3 || got[0].ID != a || got[1].Title != "Б" || got[1].Status != catalogDomain.BookStatusPending || got[2].Title != "В" {
		t.Fatalf("unexpected samples: %+v", got)
	}
	for _, s := range got {
		if s.ID == d {
			t.Fatal("the 4th book by title must be cut")
		}
	}
	if len(samples[other]) != 1 || samples[other][0].ID != a {
		t.Fatalf("unexpected samples for other: %+v", samples[other])
	}
	empty, err := f.repo.ListGenreSampleBooks(context.Background(), nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("expected empty map, got %v err=%v", empty, err)
	}
}

func TestListGenreHintCandidates_ExcludesRejectedAndAliases(t *testing.T) {
	f := newGenreModRepo(t)
	pending := f.genre("романы", "romany", "pending", nil)
	published := f.genre("роман", "roman", "published", strPtr("Q8261"))
	f.genre("мусор", "musor", "rejected", nil)
	f.alias("романа", "romana", published, nil)

	got, err := f.repo.ListGenreHintCandidates(context.Background())
	if err != nil {
		t.Fatalf("candidates: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates, got %+v", got)
	}
	ids := map[uuid.UUID]catalogDomain.Genre{}
	for _, g := range got {
		ids[g.ID] = g
	}
	if _, ok := ids[pending]; !ok {
		t.Fatalf("pending genre missing: %+v", got)
	}
	if g := ids[published]; g.WikidataQID == nil || *g.WikidataQID != "Q8261" || g.Status != "published" || g.Slug != "roman" {
		t.Fatalf("unexpected published candidate: %+v", g)
	}
}

func TestSetGenreStatus_OnlyStatusAndAliasNotFound(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	g := f.genre("роман", "roman", "pending", strPtr("Q8261"))
	alias := f.alias("романы", "romany", g, nil)
	book := f.book("Война и мир", false, g)

	found, err := f.repo.SetGenreStatus(ctx, g, catalogDomain.GenreStatusRejected)
	if err != nil || !found {
		t.Fatalf("set status: found=%v err=%v", found, err)
	}
	row := f.load(g)
	if row.Status != "rejected" || row.Name != "роман" || row.Slug != "roman" || row.WikidataQID == nil || *row.WikidataQID != "Q8261" {
		t.Fatalf("only status must change: %+v", row)
	}
	if links := f.links(book); len(links) != 1 {
		t.Fatalf("reject must keep book_genres, got %v", links)
	}

	found, err = f.repo.SetGenreStatus(ctx, alias, catalogDomain.GenreStatusPublished)
	if err != nil || found {
		t.Fatalf("alias must be not found: found=%v err=%v", found, err)
	}
	if f.load(alias).Status != "rejected" {
		t.Fatal("alias status must stay rejected")
	}
	if found, err := f.repo.SetGenreStatus(ctx, uuid.New(), "published"); err != nil || found {
		t.Fatalf("unknown id: found=%v err=%v", found, err)
	}
}

// TestUpdateGenre_EditableColumnsOnlyAndAliasNotFound: the PATCH write never
// touches merged_into_id, clears wikidata_qid with a nil pointer, and treats
// an alias (e.g. one merged after the service's read) as not found.
func TestUpdateGenre_EditableColumnsOnlyAndAliasNotFound(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	target := f.genre("роман", "roman", "published", nil)
	g := f.genre("повесть", "povest", "pending", strPtr("Q49084"))
	alias := f.alias("романы", "romany", target, nil)

	// A stale in-memory copy whose MergedIntoID is set must not leak into
	// the write (only name/slug/status/wikidata_qid are written).
	stale := uuid.New()
	err := f.repo.UpdateGenre(ctx, &catalogDomain.Genre{ID: g, Name: "Повесть", Slug: "povest-2", Status: "published", WikidataQID: nil, MergedIntoID: &stale})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	row := f.load(g)
	if row.Name != "Повесть" || row.Slug != "povest-2" || row.Status != "published" || row.WikidataQID != nil || row.MergedIntoID != nil {
		t.Fatalf("unexpected row after update: %+v", row)
	}

	// Updating an alias (stale copy with MergedIntoID nil, as a racing PATCH
	// would hold) matches no row and leaves the alias intact.
	err = f.repo.UpdateGenre(ctx, &catalogDomain.Genre{ID: alias, Name: "романы", Slug: "romany", Status: "published"})
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("alias update: expected ErrRecordNotFound, got %v", err)
	}
	if row := f.load(alias); row.Status != "rejected" || row.MergedIntoID == nil || *row.MergedIntoID != target.String() {
		t.Fatalf("alias must stay merged and rejected: %+v", row)
	}

	if err := f.repo.UpdateGenre(ctx, &catalogDomain.Genre{ID: uuid.New(), Name: "x", Slug: "x", Status: "pending"}); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unknown id: expected ErrRecordNotFound, got %v", err)
	}
}

func TestMergeGenres_MovesLinksAndBecomesAlias(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	target := f.genre("роман", "roman", "published", nil)
	source := f.genre("романы", "romany", "pending", strPtr("Q123"))
	oldAlias := f.alias("романа", "romana", source, nil)
	onlySource := f.book("Только источник", false, source)
	both := f.book("Оба", false, source, target)
	onlyTarget := f.book("Только цель", false, target)

	res, err := f.repo.MergeGenres(ctx, source, target)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if res.BooksReassigned != 1 || res.SourcesMerged != 1 || res.CollectionsSwitched != 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	for _, b := range []uuid.UUID{onlySource, both, onlyTarget} {
		if links := f.links(b); len(links) != 1 || links[0] != target.String() {
			t.Fatalf("book %s: expected exactly one link to the target, got %v", b, links)
		}
	}

	src := f.load(source)
	if src.Status != "rejected" || src.MergedIntoID == nil || *src.MergedIntoID != target.String() || src.Name != "романы" || src.Slug != "romany" {
		t.Fatalf("source must be a rejected alias keeping name/slug: %+v", src)
	}
	// QID moved to the target, which had none.
	if src.WikidataQID != nil {
		t.Fatalf("source QID must move, got %v", *src.WikidataQID)
	}
	tgt := f.load(target)
	if tgt.WikidataQID == nil || *tgt.WikidataQID != "Q123" || tgt.Status != "published" || tgt.Name != "роман" || tgt.MergedIntoID != nil {
		t.Fatalf("unexpected target: %+v", tgt)
	}
	// The existing alias of the source now points at the target.
	if a := f.load(oldAlias); a.MergedIntoID == nil || *a.MergedIntoID != target.String() {
		t.Fatalf("existing alias must be re-pointed, got %+v", a)
	}

	// The alias is now hidden from admin reads, but slug/QID lookups see it.
	if _, err := f.repo.GetGenreByID(ctx, source); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetGenreByID(alias): expected not found, got %v", err)
	}
	if _, err := f.repo.MergeGenres(ctx, source, target); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("merging an alias again: expected not found, got %v", err)
	}
}

func TestMergeGenres_BothHaveQID(t *testing.T) {
	f := newGenreModRepo(t)
	target := f.genre("роман", "roman", "published", strPtr("Q8261"))
	source := f.genre("романы", "romany", "pending", strPtr("Q123"))

	if _, err := f.repo.MergeGenres(context.Background(), source, target); err != nil {
		t.Fatalf("merge: %v", err)
	}
	if q := f.load(source).WikidataQID; q == nil || *q != "Q123" {
		t.Fatalf("source must keep its QID, got %v", q)
	}
	if q := f.load(target).WikidataQID; q == nil || *q != "Q8261" {
		t.Fatalf("target must keep its QID, got %v", q)
	}
}

func TestMergeGenres_MissingTargetChangesNothing(t *testing.T) {
	f := newGenreModRepo(t)
	source := f.genre("романы", "romany", "pending", nil)
	book := f.book("Книга", false, source)

	if _, err := f.repo.MergeGenres(context.Background(), source, uuid.New()); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected not found, got %v", err)
	}
	if row := f.load(source); row.MergedIntoID != nil || row.Status != "pending" {
		t.Fatalf("source must be untouched: %+v", row)
	}
	if links := f.links(book); len(links) != 1 || links[0] != source.String() {
		t.Fatalf("links must be untouched: %v", links)
	}
}

// TestMergeGenres_Atomic forces the last step (turning the source into an
// alias) to fail with a trigger: the moved links, the re-pointed alias and
// the QID transfer must all roll back.
func TestMergeGenres_Atomic(t *testing.T) {
	f := newGenreModRepo(t)
	target := f.genre("роман", "roman", "published", nil)
	source := f.genre("романы", "romany", "pending", strPtr("Q123"))
	oldAlias := f.alias("романа", "romana", source, nil)
	book := f.book("Книга", false, source)

	if err := f.db.Exec(`CREATE TRIGGER fail_alias BEFORE UPDATE OF status ON genres
		WHEN NEW.id = '` + source.String() + `' AND NEW.merged_into_id IS NOT NULL
		BEGIN SELECT RAISE(ABORT, 'forced failure'); END;`).Error; err != nil {
		t.Fatalf("create trigger: %v", err)
	}

	if _, err := f.repo.MergeGenres(context.Background(), source, target); err == nil {
		t.Fatal("expected the forced failure")
	}
	if links := f.links(book); len(links) != 1 || links[0] != source.String() {
		t.Fatalf("links must roll back: %v", links)
	}
	if a := f.load(oldAlias); a.MergedIntoID == nil || *a.MergedIntoID != source.String() {
		t.Fatalf("alias re-point must roll back: %+v", a)
	}
	if q := f.load(source).WikidataQID; q == nil || *q != "Q123" {
		t.Fatalf("source QID must roll back, got %v", q)
	}
	if q := f.load(target).WikidataQID; q != nil {
		t.Fatalf("target QID must roll back, got %v", *q)
	}
	if row := f.load(source); row.MergedIntoID != nil || row.Status != "pending" {
		t.Fatalf("source must be untouched: %+v", row)
	}
}

func TestGenreAliasVisibility(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()
	target := f.genre("роман", "roman", "pending", nil)
	alias := f.alias("романы", "romany", target, strPtr("Q123"))

	// Slug/QID lookups and the slug uniqueness check see the alias.
	if g, err := f.repo.GetGenreBySlug(ctx, "romany"); err != nil || g.ID != alias || g.MergedIntoID == nil || *g.MergedIntoID != target {
		t.Fatalf("GetGenreBySlug(alias): %+v err=%v", g, err)
	}
	if g, err := f.repo.GetGenreByQID(ctx, "Q123"); err != nil || g.ID != alias || !g.IsAlias() {
		t.Fatalf("GetGenreByQID(alias): %+v err=%v", g, err)
	}
	if exists, err := f.repo.GenreSlugExists(ctx, "romany", nil); err != nil || !exists {
		t.Fatalf("GenreSlugExists(alias slug): %v err=%v", exists, err)
	}

	// Admin reads hide it.
	list, total, err := f.repo.ListGenres(ctx, "", "", 20, 0)
	if err != nil || total != 1 || len(list) != 1 || list[0].ID != target {
		t.Fatalf("ListGenres: %+v total=%d err=%v", list, total, err)
	}
	rejected, total, err := f.repo.ListGenres(ctx, "", "rejected", 20, 0)
	if err != nil || total != 0 || len(rejected) != 0 {
		t.Fatalf("ListGenres(rejected): %+v total=%d err=%v", rejected, total, err)
	}
	if _, err := f.repo.GetGenreByID(ctx, alias); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetGenreByID(alias): expected not found, got %v", err)
	}
	if g, err := f.repo.GetGenreByID(ctx, target); err != nil || g.ID != target {
		t.Fatalf("GetGenreByID(target): %+v err=%v", g, err)
	}
	if ok, err := f.repo.GenresExist(ctx, []uuid.UUID{target, alias}); err != nil || ok {
		t.Fatalf("GenresExist with an alias id must be false: %v err=%v", ok, err)
	}
	if ok, err := f.repo.GenresExist(ctx, []uuid.UUID{target}); err != nil || !ok {
		t.Fatalf("GenresExist(target): %v err=%v", ok, err)
	}
	stats, err := f.repo.DashboardCounts(ctx)
	if err != nil || stats.Genres != 1 || stats.PendingGenres != 1 {
		t.Fatalf("DashboardCounts: %+v err=%v", stats, err)
	}

	// The DB CHECK pins aliases to rejected.
	if err := f.db.Exec("UPDATE genres SET status = 'published' WHERE id = ?", alias).Error; err == nil {
		t.Fatal("expected the alias => rejected CHECK to fail")
	}
}
