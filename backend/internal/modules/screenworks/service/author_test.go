package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const authorDDL = `
CREATE TABLE authors (id TEXT PRIMARY KEY, name TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'pending', external_source TEXT, external_id TEXT, created_at DATETIME, updated_at DATETIME);
CREATE TABLE author_screen_works (author_id TEXT NOT NULL, screen_work_id TEXT NOT NULL, relation TEXT NOT NULL, hidden BOOLEAN NOT NULL DEFAULT 0, source TEXT NOT NULL DEFAULT 'wikidata', created_at DATETIME, updated_at DATETIME, PRIMARY KEY (author_id, screen_work_id));
`

type fakeAuthorSource struct {
	films map[string][]bookdata.ExternalAuthorFilm
	err   error
	calls [][]string
}

func (f *fakeAuthorSource) FetchAuthorFilms(_ context.Context, qids []string) (map[string][]bookdata.ExternalAuthorFilm, error) {
	f.calls = append(f.calls, qids)
	if f.err != nil {
		return nil, f.err
	}
	return f.films, nil
}

func setupAuthors(t *testing.T, src *fakeAuthorSource, bookSrc *fakeSource) (*Service, *gorm.DB) {
	t.Helper()
	svc, db := setup(t, bookSrc)
	if err := db.Exec(authorDDL).Error; err != nil {
		t.Fatal(err)
	}
	if src != nil {
		svc.WithAuthorSource(src)
	}
	return svc, db
}

func mkAuthor(t *testing.T, db *gorm.DB, status, qid string) uuid.UUID {
	t.Helper()
	id := uuid.New()
	var src, ext any
	if qid != "" {
		src, ext = "wikidata", qid
	}
	now := time.Now()
	if err := db.Exec("INSERT INTO authors (id, name, status, external_source, external_id, created_at, updated_at) VALUES (?, 'Автор', ?, ?, ?, ?, ?)",
		id, status, src, ext, now, now).Error; err != nil {
		t.Fatal(err)
	}
	return id
}

func film(qid, title, relation string, y *int) bookdata.ExternalAuthorFilm {
	return bookdata.ExternalAuthorFilm{
		ExternalScreenWork: bookdata.ExternalScreenWork{QID: qid, Title: title, Kind: bookdata.ScreenKindFilm, Year: y},
		Relation:           relation,
	}
}

func TestSyncAuthors_AddedThenUnchanged(t *testing.T) {
	src := &fakeAuthorSource{films: map[string][]bookdata.ExternalAuthorFilm{
		"Q1": {film("Q10", "Толкин", domain.RelationBiopic, year(2019)), film("Q11", "Док", domain.RelationDocumentary, year(2011))},
	}}
	svc, db := setupAuthors(t, src, nil)
	author := mkAuthor(t, db, "pending", "Q1")
	ctx := context.Background()

	rep, err := svc.SyncAuthors(ctx, []uuid.UUID{author, author})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Processed != 1 || rep.Updated != 1 || rep.Added != 2 || rep.Items[0].Outcome != domain.OutcomeAdded || rep.Items[0].Found != 2 {
		t.Fatalf("report = %+v", rep)
	}
	rep, err = svc.SyncAuthors(ctx, []uuid.UUID{author})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items[0].Outcome != domain.OutcomeUnchanged || rep.Items[0].Found != 2 || rep.Items[0].Added != 0 || rep.Updated != 0 {
		t.Fatalf("re-sync = %+v", rep)
	}
	if count(t, db, "screen_works") != 2 || count(t, db, "author_screen_works") != 2 {
		t.Fatal("duplicates after re-sync")
	}
}

func TestSyncAuthors_SharedWorks(t *testing.T) {
	src := &fakeAuthorSource{films: map[string][]bookdata.ExternalAuthorFilm{
		"Q1": {film("Q10", "Общий", domain.RelationDocumentary, year(2000))},
		"Q2": {film("Q10", "Общий", domain.RelationDocumentary, year(2000))},
	}}
	bookSrc := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q99": {work("Q10", "Общий", year(2000))}}}
	svc, db := setupAuthors(t, src, bookSrc)
	ctx := context.Background()
	book := mkBook(t, db, "published", "Q99")
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	a, b := mkAuthor(t, db, "published", "Q1"), mkAuthor(t, db, "published", "Q2")
	if _, err := svc.SyncAuthors(ctx, []uuid.UUID{a, b}); err != nil {
		t.Fatal(err)
	}
	if count(t, db, "screen_works") != 1 || count(t, db, "author_screen_works") != 2 || count(t, db, "book_screen_works") != 1 {
		t.Fatal("expected one work linked to the book and both authors")
	}
	if len(src.calls) != 1 {
		t.Fatalf("calls = %d, want one SPARQL call", len(src.calls))
	}

	// Hiding the author link does not touch the book link.
	links, _ := svc.ListAdminByAuthor(ctx, a)
	if _, err := svc.SetAuthorFilmHidden(ctx, a, links[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if pub, _ := svc.ListPublic(ctx, book); len(pub) != 1 {
		t.Fatalf("book adaptation affected: %+v", pub)
	}
}

func TestSyncAuthors_HiddenStaysHiddenAndRelationFrozen(t *testing.T) {
	src := &fakeAuthorSource{films: map[string][]bookdata.ExternalAuthorFilm{"Q1": {film("Q10", "Ф", domain.RelationBiopic, year(2000))}}}
	svc, db := setupAuthors(t, src, nil)
	author := mkAuthor(t, db, "published", "Q1")
	ctx := context.Background()
	if _, err := svc.SyncAuthors(ctx, []uuid.UUID{author}); err != nil {
		t.Fatal(err)
	}
	links, _ := svc.ListAdminByAuthor(ctx, author)
	if _, err := svc.SetAuthorFilmHidden(ctx, author, links[0].ID, true); err != nil {
		t.Fatal(err)
	}
	src.films["Q1"] = []bookdata.ExternalAuthorFilm{film("Q10", "Ф", domain.RelationDocumentary, year(2000))}
	rep, err := svc.SyncAuthors(ctx, []uuid.UUID{author})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items[0].Outcome != domain.OutcomeUnchanged || rep.Items[0].Added != 0 {
		t.Fatalf("report = %+v", rep.Items[0])
	}
	if pub, _ := svc.ListPublicByAuthor(ctx, author); len(pub) != 0 {
		t.Fatalf("hidden link is public: %+v", pub)
	}
	adm, _ := svc.ListAdminByAuthor(ctx, author)
	if len(adm) != 1 || !adm[0].Hidden || adm[0].Relation != domain.RelationBiopic {
		t.Fatalf("admin list = %+v", adm)
	}
	if _, err := svc.SetAuthorFilmHidden(ctx, author, links[0].ID, false); err != nil {
		t.Fatal(err)
	}
	if pub, _ := svc.ListPublicByAuthor(ctx, author); len(pub) != 1 {
		t.Fatalf("shown link is not public: %+v", pub)
	}
}

func TestSyncAuthors_FillsEmptyFieldsAndRespectsManualEdit(t *testing.T) {
	src := &fakeAuthorSource{films: map[string][]bookdata.ExternalAuthorFilm{"Q1": {film("Q10", "Старое", domain.RelationBiopic, nil), film("Q11", "Ручное", domain.RelationBiopic, nil)}}}
	svc, db := setupAuthors(t, src, nil)
	author := mkAuthor(t, db, "published", "Q1")
	ctx := context.Background()
	if _, err := svc.SyncAuthors(ctx, []uuid.UUID{author}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE screen_works SET manually_edited_at = ? WHERE external_id = 'Q11'", time.Now()).Error; err != nil {
		t.Fatal(err)
	}
	n := film("Q10", "Новое", domain.RelationBiopic, year(1999))
	n.Kind = bookdata.ScreenKindSeries
	src.films["Q1"] = []bookdata.ExternalAuthorFilm{n, film("Q11", "Ручное", domain.RelationBiopic, year(1999))}
	if _, err := svc.SyncAuthors(ctx, []uuid.UUID{author}); err != nil {
		t.Fatal(err)
	}
	links, _ := svc.ListAdminByAuthor(ctx, author)
	for _, l := range links {
		switch l.Title {
		case "Старое":
			if l.Kind != domain.KindFilm || l.Year == nil || *l.Year != 1999 {
				t.Fatalf("Q10 = %+v", l)
			}
		case "Ручное":
			if l.Year != nil {
				t.Fatalf("manually edited work was filled: %+v", l)
			}
		default:
			t.Fatalf("unexpected %+v", l)
		}
	}
}

func TestSyncAuthors_SkippedUnknownFailureAndValidation(t *testing.T) {
	src := &fakeAuthorSource{films: map[string][]bookdata.ExternalAuthorFilm{}}
	svc, db := setupAuthors(t, src, nil)
	ctx := context.Background()
	noQID, withQID := mkAuthor(t, db, "published", ""), mkAuthor(t, db, "published", "Q1")
	rep, err := svc.SyncAuthors(ctx, []uuid.UUID{noQID, uuid.New(), withQID})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Items[0].Outcome != domain.OutcomeSkipped || rep.Items[0].Notes[0] != domain.NoteNoQID {
		t.Fatalf("no qid = %+v", rep.Items[0])
	}
	if rep.Items[1].Outcome != domain.OutcomeError || rep.Items[1].Message != "author not found" {
		t.Fatalf("unknown = %+v", rep.Items[1])
	}
	if rep.Items[2].Outcome != domain.OutcomeUnchanged || rep.Items[2].Found != 0 {
		t.Fatalf("no films = %+v", rep.Items[2])
	}
	if len(src.calls) != 1 || len(src.calls[0]) != 1 || src.calls[0][0] != "Q1" {
		t.Fatalf("calls = %v", src.calls)
	}

	src.err = bookdata.ErrSourceUnavailable
	rep, err = svc.SyncAuthors(ctx, []uuid.UUID{withQID})
	if !errors.Is(err, bookdata.ErrSourceUnavailable) || rep == nil || rep.Items[0].Notes[0] != domain.NoteWikidataUnavailable {
		t.Fatalf("rep=%+v err=%v", rep, err)
	}

	if _, err := svc.SyncAuthors(ctx, nil); !errors.Is(err, ErrSyncEmpty) {
		t.Fatalf("empty: %v", err)
	}
	ids := make([]uuid.UUID, 51)
	for i := range ids {
		ids[i] = uuid.New()
	}
	if _, err := svc.SyncAuthors(ctx, ids); !errors.Is(err, ErrAuthorSyncTooLarge) {
		t.Fatalf("too large: %v", err)
	}
	noSource, _ := setupAuthors(t, nil, nil)
	if _, err := noSource.SyncAuthors(ctx, ids[:1]); !errors.Is(err, ErrAuthorSyncUnavailable) {
		t.Fatalf("unavailable: %v", err)
	}
	if noSource.HasAuthorSyncSource() || !svc.HasAuthorSyncSource() {
		t.Fatal("HasAuthorSyncSource")
	}
}

func TestSyncCreatedAuthors(t *testing.T) {
	src := &fakeAuthorSource{films: map[string][]bookdata.ExternalAuthorFilm{
		"Q1": {film("Q10", "Ф", domain.RelationBiopic, year(2000))},
	}}
	svc, db := setupAuthors(t, src, nil)
	ids := []uuid.UUID{mkAuthor(t, db, "pending", "Q1")}
	added, failed := svc.SyncCreatedAuthors(context.Background(), ids)
	if added != 1 || failed != 0 {
		t.Fatalf("added=%d failed=%d", added, failed)
	}
	src.err = bookdata.ErrSourceUnavailable
	if _, failed = svc.SyncCreatedAuthors(context.Background(), ids); failed != 1 {
		t.Fatalf("failed = %d", failed)
	}
	if a, f := svc.SyncCreatedAuthors(context.Background(), nil); a != 0 || f != 0 {
		t.Fatalf("empty: %d %d", a, f)
	}
}

func TestListPublicByAuthor_Visibility(t *testing.T) {
	src := &fakeAuthorSource{films: map[string][]bookdata.ExternalAuthorFilm{"Q1": {
		film("Q10", "Б", domain.RelationBiopic, year(2019)), film("Q11", "А", domain.RelationDocumentary, year(2011)), film("Q12", "Без года", domain.RelationBiopic, nil),
	}}}
	svc, db := setupAuthors(t, src, nil)
	pub, pending := mkAuthor(t, db, "published", "Q1"), mkAuthor(t, db, "pending", "Q1")
	ctx := context.Background()
	if _, err := svc.SyncAuthors(ctx, []uuid.UUID{pub, pending}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.ListPublicByAuthor(ctx, pub)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0].Title != "А" || got[0].Relation != domain.RelationDocumentary || got[1].Title != "Б" || got[2].Title != "Без года" {
		t.Fatalf("order = %+v", got)
	}
	if _, err := svc.ListPublicByAuthor(ctx, pending); !errors.Is(err, domain.ErrAuthorNotFound) {
		t.Fatalf("pending: %v", err)
	}
	if _, err := svc.ListPublicByAuthor(ctx, uuid.New()); !errors.Is(err, domain.ErrAuthorNotFound) {
		t.Fatalf("unknown: %v", err)
	}
	if _, err := svc.SetAuthorFilmHidden(ctx, pub, uuid.New(), true); !errors.Is(err, domain.ErrAuthorLinkNotFound) {
		t.Fatalf("unknown link: %v", err)
	}
	if _, err := svc.SetAuthorFilmHidden(ctx, uuid.New(), uuid.New(), true); !errors.Is(err, domain.ErrAuthorLinkNotFound) {
		t.Fatalf("unknown author: %v", err)
	}
}
