package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/modules/screenworks/repository"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const ddl = `
CREATE TABLE books (id TEXT PRIMARY KEY, title TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT, created_at DATETIME, updated_at DATETIME);
CREATE TABLE screen_works (id TEXT PRIMARY KEY, title TEXT NOT NULL, original_title TEXT, kind TEXT NOT NULL, year INTEGER, director TEXT, imdb_id TEXT, kinopoisk_id TEXT, external_source TEXT, external_id TEXT, manually_edited_at DATETIME, poster_source TEXT, poster_url TEXT, poster_path TEXT, poster_author TEXT, poster_license TEXT, poster_license_url TEXT, poster_source_url TEXT, poster_updated_at DATETIME, created_at DATETIME, updated_at DATETIME);
CREATE UNIQUE INDEX uq_screen_works_external ON screen_works (external_source, external_id) WHERE external_id IS NOT NULL;
CREATE TABLE book_screen_works (book_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, hidden BOOLEAN NOT NULL DEFAULT 0, source TEXT NOT NULL DEFAULT 'wikidata', created_at DATETIME, updated_at DATETIME, PRIMARY KEY (book_id, screen_work_id));
`

type fakeSource struct {
	works map[string][]bookdata.ExternalScreenWork
	err   error
	calls [][]string
}

func (f *fakeSource) FetchAdaptations(_ context.Context, qids []string) (map[string][]bookdata.ExternalScreenWork, error) {
	f.calls = append(f.calls, qids)
	if f.err != nil {
		return nil, f.err
	}
	return f.works, nil
}

func setup(t *testing.T, src *fakeSource) (*Service, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec(ddl).Error; err != nil {
		t.Fatal(err)
	}
	var source Source
	if src != nil {
		source = src
	}
	return NewService(repository.NewGormRepository(db), source, nil), db
}

func mkBook(t *testing.T, db *gorm.DB, status, qid string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var src, ext any
	if qid != "" {
		src, ext = "wikidata", qid
	}
	now := time.Now()
	if err := db.Exec("INSERT INTO books (id, title, status, external_source, external_id, created_at, updated_at) VALUES (?, 'Книга', ?, ?, ?, ?, ?)",
		id, status, src, ext, now, now).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func year(y int) *int { return &y }

func work(qid, title string, y *int) bookdata.ExternalScreenWork {
	return bookdata.ExternalScreenWork{QID: qid, Title: title, Kind: bookdata.ScreenKindFilm, Year: y}
}

func count(t *testing.T, db *gorm.DB, table string) int {
	t.Helper()
	var n int
	if err := db.Raw("SELECT COUNT(*) FROM " + table).Scan(&n).Error; err != nil {
		t.Fatal(err)
	}
	return n
}

func TestSyncBooks_AddedThenUnchanged(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{
		"Q1": {work("Q10", "Фильм", year(1994)), work("Q11", "Сериал", year(1972))},
	}}
	svc, db := setup(t, src)
	book := mkBook(t, db, "pending", "Q1")

	rep, err := svc.SyncBooks(context.Background(), []uuid.UUID{book, book})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Processed != 1 || rep.Updated != 1 || rep.Added != 2 || rep.Items[0].Outcome != domain.OutcomeAdded || rep.Items[0].Found != 2 {
		t.Fatalf("report = %+v", rep)
	}
	rep, err = svc.SyncBooks(context.Background(), []uuid.UUID{book})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items[0].Outcome != domain.OutcomeUnchanged || rep.Items[0].Found != 2 || rep.Items[0].Added != 0 || rep.Updated != 0 {
		t.Fatalf("re-sync = %+v", rep)
	}
	if count(t, db, "screen_works") != 2 || count(t, db, "book_screen_works") != 2 {
		t.Fatal("duplicates after re-sync")
	}
}

func TestSyncBooks_WorkSharedByTwoBooks(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{
		"Q1": {work("Q10", "Общий", year(2000))}, "Q2": {work("Q10", "Общий", year(2000))},
	}}
	svc, db := setup(t, src)
	a, b := mkBook(t, db, "published", "Q1"), mkBook(t, db, "published", "Q2")
	if _, err := svc.SyncBooks(context.Background(), []uuid.UUID{a, b}); err != nil {
		t.Fatal(err)
	}
	if count(t, db, "screen_works") != 1 || count(t, db, "book_screen_works") != 2 {
		t.Fatal("expected one work linked to both books")
	}
	if len(src.calls) != 1 {
		t.Fatalf("calls = %d, want one SPARQL call", len(src.calls))
	}
}

func TestSyncBooks_HiddenLinkStaysHidden(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {work("Q10", "Ф", year(2000))}}}
	svc, db := setup(t, src)
	book := mkBook(t, db, "published", "Q1")
	ctx := context.Background()
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	links, _ := svc.ListAdmin(ctx, book)
	if _, err := svc.SetHidden(ctx, book, links[0].ID, true); err != nil {
		t.Fatal(err)
	}
	rep, err := svc.SyncBooks(ctx, []uuid.UUID{book})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items[0].Outcome != domain.OutcomeUnchanged || rep.Items[0].Added != 0 {
		t.Fatalf("report = %+v", rep.Items[0])
	}
	if pub, _ := svc.ListPublic(ctx, book); len(pub) != 0 {
		t.Fatalf("hidden link is public: %+v", pub)
	}
	if adm, _ := svc.ListAdmin(ctx, book); len(adm) != 1 || !adm[0].Hidden {
		t.Fatalf("admin list = %+v", adm)
	}
}

func TestSyncBooks_FillsEmptyFieldsOnly(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {work("Q10", "Старое", nil)}}}
	svc, db := setup(t, src)
	book := mkBook(t, db, "published", "Q1")
	ctx := context.Background()
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	w := work("Q10", "Новое название", year(1999))
	w.Kind = bookdata.ScreenKindSeries
	w.Director = "Режиссёр"
	src.works["Q1"] = []bookdata.ExternalScreenWork{w}
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	links, _ := svc.ListAdmin(ctx, book)
	got := links[0]
	if got.Title != "Старое" || got.Kind != domain.KindFilm {
		t.Fatalf("title/kind changed: %+v", got)
	}
	if got.Year == nil || *got.Year != 1999 || got.Director == nil || *got.Director != "Режиссёр" {
		t.Fatalf("empty fields not filled: %+v", got)
	}
	// A filled year is not overwritten.
	w.Year = year(2005)
	src.works["Q1"] = []bookdata.ExternalScreenWork{w}
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	links, _ = svc.ListAdmin(ctx, book)
	if *links[0].Year != 1999 {
		t.Fatalf("year overwritten: %d", *links[0].Year)
	}
}

func TestSyncBooks_ManuallyEditedWorkUntouched(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {work("Q10", "Ф", nil)}}}
	svc, db := setup(t, src)
	book := mkBook(t, db, "published", "Q1")
	ctx := context.Background()
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE screen_works SET manually_edited_at = ?", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	src.works["Q1"] = []bookdata.ExternalScreenWork{work("Q10", "Ф", year(1999))}
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	links, _ := svc.ListAdmin(ctx, book)
	if links[0].Year != nil {
		t.Fatalf("manually edited work was filled: %+v", links[0])
	}
}

func TestSyncBooks_SkippedUnknownAndNoQID(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{}}
	svc, db := setup(t, src)
	noQID := mkBook(t, db, "published", "")
	withQID := mkBook(t, db, "published", "Q1")
	unknown := uuid.New()
	rep, err := svc.SyncBooks(context.Background(), []uuid.UUID{noQID, unknown, withQID})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items[0].Outcome != domain.OutcomeSkipped || rep.Items[0].Notes[0] != domain.NoteNoQID {
		t.Fatalf("no qid = %+v", rep.Items[0])
	}
	if rep.Items[1].Outcome != domain.OutcomeError || rep.Items[1].Message != "book not found" {
		t.Fatalf("unknown = %+v", rep.Items[1])
	}
	if rep.Items[2].Outcome != domain.OutcomeUnchanged || rep.Items[2].Found != 0 {
		t.Fatalf("no adaptations = %+v", rep.Items[2])
	}
	if len(src.calls) != 1 || len(src.calls[0]) != 1 || src.calls[0][0] != "Q1" {
		t.Fatalf("calls = %v", src.calls)
	}
}

func TestSyncBooks_SourceFailure(t *testing.T) {
	src := &fakeSource{err: bookdata.ErrSourceUnavailable}
	svc, db := setup(t, src)
	a, b := mkBook(t, db, "published", "Q1"), mkBook(t, db, "published", "Q2")
	rep, err := svc.SyncBooks(context.Background(), []uuid.UUID{a, b})
	if !errors.Is(err, bookdata.ErrSourceUnavailable) || rep == nil {
		t.Fatalf("rep=%v err=%v", rep, err)
	}
	for _, it := range rep.Items {
		if it.Outcome != domain.OutcomeError || it.Notes[0] != domain.NoteWikidataUnavailable {
			t.Fatalf("item = %+v", it)
		}
	}
	if count(t, db, "book_screen_works") != 0 {
		t.Fatal("links created on failure")
	}
}

func TestSyncBooks_Validation(t *testing.T) {
	svc, _ := setup(t, &fakeSource{})
	ctx := context.Background()
	if _, err := svc.SyncBooks(ctx, nil); !errors.Is(err, ErrSyncEmpty) {
		t.Fatalf("empty: %v", err)
	}
	ids := make([]uuid.UUID, 51)
	for i := range ids {
		ids[i] = uuid.New()
	}
	if _, err := svc.SyncBooks(ctx, ids); !errors.Is(err, ErrSyncTooLarge) {
		t.Fatalf("too large: %v", err)
	}
	noSource, _ := setup(t, nil)
	if _, err := noSource.SyncBooks(ctx, ids[:1]); !errors.Is(err, ErrSyncUnavailable) {
		t.Fatalf("unavailable: %v", err)
	}
}

func TestSyncCreatedBooks_ChunksAndCounts(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{
		"Q1": {work("Q10", "Ф", year(2000)), work("Q11", "Ф2", year(2001))},
	}}
	svc, db := setup(t, src)
	ids := []uuid.UUID{mkBook(t, db, "pending", "Q1")}
	for i := 0; i < SyncCap; i++ {
		ids = append(ids, mkBook(t, db, "pending", "Q"+strconv.Itoa(100+i)))
	}
	added, failed := svc.SyncCreatedBooks(context.Background(), ids)
	if added != 2 || failed != 0 {
		t.Fatalf("added=%d failed=%d", added, failed)
	}
	if len(src.calls) != 2 {
		t.Fatalf("calls = %d, want 2 chunks", len(src.calls))
	}

	src.err = bookdata.ErrSourceUnavailable
	_, failed = svc.SyncCreatedBooks(context.Background(), ids[:3])
	if failed != 3 {
		t.Fatalf("failed = %d", failed)
	}
}

func TestListPublic_Visibility(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {
		work("Q10", "Б", year(1994)), work("Q11", "А", year(1972)), work("Q12", "Без года", nil), work("Q13", "Скрытый", year(1980)),
	}}}
	svc, db := setup(t, src)
	pub := mkBook(t, db, "published", "Q1")
	pending := mkBook(t, db, "pending", "")
	ctx := context.Background()
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{pub}); err != nil {
		t.Fatal(err)
	}
	admin, _ := svc.ListAdmin(ctx, pub)
	for _, l := range admin {
		if l.Title == "Скрытый" {
			if _, err := svc.SetHidden(ctx, pub, l.ID, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	got, err := svc.ListPublic(ctx, pub)
	if err != nil {
		t.Fatal(err)
	}
	var titles []string
	for _, l := range got {
		titles = append(titles, l.Title)
	}
	want := []string{"А", "Б", "Без года"}
	if len(titles) != 3 || titles[0] != want[0] || titles[1] != want[1] || titles[2] != want[2] {
		t.Fatalf("order = %v", titles)
	}
	if _, err := svc.ListPublic(ctx, pending); !errors.Is(err, domain.ErrBookNotFound) {
		t.Fatalf("pending: %v", err)
	}
	if _, err := svc.ListPublic(ctx, uuid.New()); !errors.Is(err, domain.ErrBookNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := svc.SetHidden(ctx, pub, uuid.New(), true); !errors.Is(err, domain.ErrLinkNotFound) {
		t.Fatalf("unknown link: %v", err)
	}
}
