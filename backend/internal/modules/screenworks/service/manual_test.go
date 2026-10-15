package service

import (
	"context"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func sptr(s string) *string { return &s }

func newWork(title string) *domain.WorkFields {
	return &domain.WorkFields{Title: title, Kind: domain.KindTVFilm, Year: year(1988)}
}

func scalar(t *testing.T, db *gorm.DB, q string, args ...any) string {
	t.Helper()
	var s string
	if err := db.Raw(q, args...).Scan(&s).Error; err != nil {
		t.Fatal(err)
	}
	return s
}

func TestAddBookAdaptation_ValidationTouchesNothing(t *testing.T) {
	svc, db := setupAuthors(t, nil, nil)
	book := mkBook(t, db, "published", "")
	ctx := context.Background()

	bad := newWork("x")
	bad.Year = year(1850)
	_, err := svc.AddBookAdaptation(ctx, book, nil, bad)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Msg != "year must be between 1880 and 2100" {
		t.Fatalf("err = %v", err)
	}
	id := uuid.New()
	for name, args := range map[string]struct {
		id   *uuid.UUID
		work *domain.WorkFields
	}{"neither": {nil, nil}, "both": {&id, newWork("x")}} {
		if _, err := svc.AddBookAdaptation(ctx, book, args.id, args.work); !errors.Is(err, domain.ErrLinkRefRequired) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := svc.AddBookAdaptation(ctx, uuid.New(), nil, newWork("x")); !errors.Is(err, domain.ErrBookNotFound) {
		t.Errorf("unknown book: %v", err)
	}
	if _, err := svc.AddBookAdaptation(ctx, book, &id, nil); !errors.Is(err, domain.ErrWorkNotFound) {
		t.Errorf("unknown work: %v", err)
	}
	if count(t, db, "screen_works") != 0 || count(t, db, "book_screen_works") != 0 {
		t.Error("rows were created")
	}
}

func TestAddBookAdaptation_NewWorkIsManualAndLocked(t *testing.T) {
	svc, db := setupAuthors(t, nil, nil)
	book := mkBook(t, db, "pending", "")
	res, err := svc.AddBookAdaptation(context.Background(), book, nil, newWork("  Собачье сердце "))
	if err != nil {
		t.Fatal(err)
	}
	if !res.WorkCreated || res.Item.Source != domain.SourceManual || res.Item.Hidden || res.Item.Title != "Собачье сердце" {
		t.Errorf("%+v", res)
	}
	if scalar(t, db, "SELECT manually_edited_at IS NOT NULL FROM screen_works") != "1" {
		t.Error("manually_edited_at not set")
	}
}

func TestAddBookAdaptation_QIDMatchLinksExistingUnchanged(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {work("Q10", "Wikidata title", year(1994))}}}
	svc, db := setupAuthors(t, nil, src)
	first := mkBook(t, db, "published", "Q1")
	second := mkBook(t, db, "published", "")
	ctx := context.Background()
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{first}); err != nil {
		t.Fatal(err)
	}
	w := newWork("Other title")
	w.QID = sptr("q10")
	res, err := svc.AddBookAdaptation(ctx, second, nil, w)
	if err != nil {
		t.Fatal(err)
	}
	if res.WorkCreated || res.Item.Title != "Wikidata title" || count(t, db, "screen_works") != 1 {
		t.Errorf("%+v works=%d", res, count(t, db, "screen_works"))
	}
	if scalar(t, db, "SELECT manually_edited_at IS NULL FROM screen_works") != "1" {
		t.Error("reused work was marked edited")
	}
}

func TestAddBookAdaptation_NewWorkWithQIDIsStoredAsWikidata(t *testing.T) {
	svc, db := setupAuthors(t, nil, nil)
	book := mkBook(t, db, "published", "")
	w := newWork("Солярис")
	w.QID = sptr("Q181069")
	if _, err := svc.AddBookAdaptation(context.Background(), book, nil, w); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, db, "SELECT external_source || ':' || external_id FROM screen_works"); got != "wikidata:Q181069" {
		t.Errorf("got %q", got)
	}
}

func TestAddBookAdaptation_DuplicateExternalID(t *testing.T) {
	svc, db := setupAuthors(t, nil, nil)
	b1, b2 := mkBook(t, db, "published", ""), mkBook(t, db, "published", "")
	ctx := context.Background()
	w := newWork("A")
	w.IMDbID = sptr("tt0069293")
	w.KinopoiskID = sptr("43911")
	first, err := svc.AddBookAdaptation(ctx, b1, nil, w)
	if err != nil {
		t.Fatal(err)
	}
	for _, mk := range []func() *domain.WorkFields{
		func() *domain.WorkFields { x := newWork("B"); x.IMDbID = sptr("tt0069293"); return x },
		func() *domain.WorkFields { x := newWork("B"); x.KinopoiskID = sptr("43911"); return x },
	} {
		_, err := svc.AddBookAdaptation(ctx, b2, nil, mk())
		var dup *domain.ErrDuplicateExternalID
		if !errors.As(err, &dup) || dup.WorkID != first.Item.ID {
			t.Fatalf("err = %v", err)
		}
	}
	if count(t, db, "screen_works") != 1 {
		t.Error("duplicate created")
	}
}

func TestAddBookAdaptation_AlreadyLinkedKeepsHiddenWikidataLink(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {work("Q10", "Фильм", year(1994))}}}
	svc, db := setupAuthors(t, nil, src)
	book := mkBook(t, db, "published", "Q1")
	ctx := context.Background()
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	links, _ := svc.ListAdmin(ctx, book)
	if _, err := svc.SetHidden(ctx, book, links[0].ID, true); err != nil {
		t.Fatal(err)
	}
	id := links[0].ID
	if _, err := svc.AddBookAdaptation(ctx, book, &id, nil); !errors.Is(err, domain.ErrAlreadyLinked) {
		t.Fatalf("err = %v", err)
	}
	if got := scalar(t, db, "SELECT source || hidden FROM book_screen_works"); got != "wikidata1" {
		t.Errorf("link changed: %q", got)
	}
}

func TestDeleteBookAdaptation(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {work("Q10", "Фильм", year(1994))}}}
	svc, db := setupAuthors(t, nil, src)
	book := mkBook(t, db, "published", "Q1")
	ctx := context.Background()
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	links, _ := svc.ListAdmin(ctx, book)
	if err := svc.DeleteBookAdaptation(ctx, book, links[0].ID); !errors.Is(err, domain.ErrLinkNotManual) {
		t.Errorf("wikidata: %v", err)
	}
	if err := svc.DeleteBookAdaptation(ctx, book, uuid.New()); !errors.Is(err, domain.ErrLinkNotFound) {
		t.Errorf("missing: %v", err)
	}
	res, err := svc.AddBookAdaptation(ctx, book, nil, newWork("Ручной"))
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteBookAdaptation(ctx, book, res.Item.ID); err != nil {
		t.Fatal(err)
	}
	if count(t, db, "book_screen_works") != 1 || count(t, db, "screen_works") != 2 {
		t.Error("link deleted but work must stay")
	}
}

func TestDeleteWork(t *testing.T) {
	svc, db := setupAuthors(t, nil, nil)
	book := mkBook(t, db, "published", "")
	ctx := context.Background()
	res, _ := svc.AddBookAdaptation(ctx, book, nil, newWork("Ручной"))
	if err := svc.DeleteWork(ctx, res.Item.ID); !errors.Is(err, domain.ErrWorkHasLinks) {
		t.Errorf("linked: %v", err)
	}
	if count(t, db, "screen_works") != 1 || count(t, db, "book_screen_works") != 1 {
		t.Error("a refused delete must keep the work and its link")
	}
	if err := svc.DeleteWork(ctx, uuid.New()); !errors.Is(err, domain.ErrWorkNotFound) {
		t.Errorf("unknown: %v", err)
	}
	if err := svc.DeleteBookAdaptation(ctx, book, res.Item.ID); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteWork(ctx, res.Item.ID); err != nil {
		t.Fatal(err)
	}
	if count(t, db, "screen_works") != 0 {
		t.Error("work not deleted")
	}
}

func TestUpdateWork_QIDRules(t *testing.T) {
	svc, db := setupAuthors(t, nil, nil)
	book := mkBook(t, db, "published", "")
	ctx := context.Background()
	manual, _ := svc.AddBookAdaptation(ctx, book, nil, newWork("Ручной"))
	withQID := newWork("С QID")
	withQID.QID = sptr("Q5")
	other, _ := svc.AddBookAdaptation(ctx, book, nil, withQID)

	// A QID taken by another work.
	f := *newWork("Ручной")
	f.QID = sptr("Q5")
	if _, err := svc.UpdateWork(ctx, manual.Item.ID, f); !errors.Is(err, domain.ErrQIDTaken) {
		t.Errorf("taken: %v", err)
	}
	// Add a free QID.
	f.QID = sptr("q6")
	d, err := svc.UpdateWork(ctx, manual.Item.ID, f)
	if err != nil || d.QID == nil || *d.QID != "Q6" || d.ManuallyEditedAt == nil {
		t.Fatalf("add qid: %v %+v", err, d)
	}
	// Change and clear are rejected; same or omitted keeps it.
	f.QID = sptr("Q7")
	if _, err := svc.UpdateWork(ctx, manual.Item.ID, f); !errors.Is(err, domain.ErrQIDImmutable) {
		t.Errorf("change: %v", err)
	}
	f.QID = nil
	f.Title = "Новое"
	d, err = svc.UpdateWork(ctx, manual.Item.ID, f)
	if err != nil || d.QID == nil || *d.QID != "Q6" || d.Title != "Новое" {
		t.Errorf("omitted: %v %+v", err, d)
	}
	if _, err := svc.UpdateWork(ctx, uuid.New(), f); !errors.Is(err, domain.ErrWorkNotFound) {
		t.Errorf("unknown: %v", err)
	}
	_ = other
}

func TestUpdateWork_FullReplaceAndSyncLock(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {work("Q10", "Wikidata title", year(1994))}}}
	svc, db := setupAuthors(t, nil, src)
	book := mkBook(t, db, "published", "Q1")
	ctx := context.Background()
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	links, _ := svc.ListAdmin(ctx, book)
	d, err := svc.UpdateWork(ctx, links[0].ID, domain.WorkFields{Title: "Мой заголовок", Kind: domain.KindFilm})
	if err != nil || d.Year != nil {
		t.Fatalf("%v %+v", err, d)
	}
	if _, err := svc.SyncBooks(ctx, []uuid.UUID{book}); err != nil {
		t.Fatal(err)
	}
	d, _ = svc.GetWork(ctx, links[0].ID)
	if d.Title != "Мой заголовок" || d.Year != nil {
		t.Errorf("sync overwrote the edit: %+v", d)
	}
	if len(d.Books) != 1 || d.Books[0].Source != domain.SourceWikidata {
		t.Errorf("links: %+v", d.Books)
	}
}

func TestSyncBooks_ManualLinkWithQIDStaysManual(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{"Q1": {work("Q10", "Фильм", year(1994))}}}
	svc, db := setupAuthors(t, nil, src)
	book := mkBook(t, db, "published", "Q1")
	ctx := context.Background()
	w := newWork("Фильм")
	w.QID = sptr("Q10")
	if _, err := svc.AddBookAdaptation(ctx, book, nil, w); err != nil {
		t.Fatal(err)
	}
	rep, err := svc.SyncBooks(ctx, []uuid.UUID{book})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Added != 0 || rep.Items[0].Added != 0 {
		t.Errorf("report: %+v", rep)
	}
	if count(t, db, "book_screen_works") != 1 || scalar(t, db, "SELECT source FROM book_screen_works") != domain.SourceManual {
		t.Error("manual link changed")
	}
}

func TestAddAuthorFilm_RelationAndPatch(t *testing.T) {
	src := &fakeAuthorSource{films: map[string][]bookdata.ExternalAuthorFilm{
		"Q9": {film("Q20", "Док", domain.RelationBiopic, year(2011))},
	}}
	svc, db := setupAuthors(t, src, nil)
	author := mkAuthor(t, db, "pending", "Q9")
	ctx := context.Background()

	if _, err := svc.AddAuthorFilm(ctx, author, nil, newWork("X"), ""); !errors.Is(err, domain.ErrInvalidRelation) {
		t.Errorf("relation: %v", err)
	}
	if _, err := svc.AddAuthorFilm(ctx, uuid.New(), nil, newWork("X"), domain.RelationBiopic); !errors.Is(err, domain.ErrAuthorNotFound) {
		t.Errorf("unknown author: %v", err)
	}
	res, err := svc.AddAuthorFilm(ctx, author, nil, newWork("Ручной"), domain.RelationDocumentary)
	if err != nil || res.Item.Relation != domain.RelationDocumentary || res.Item.Source != domain.SourceManual || !res.WorkCreated {
		t.Fatalf("%v %+v", err, res)
	}
	id := res.Item.ID
	if _, err := svc.AddAuthorFilm(ctx, author, &id, nil, domain.RelationBiopic); !errors.Is(err, domain.ErrAlreadyLinked) {
		t.Errorf("already: %v", err)
	}

	// Patch: empty, bad relation, relation only keeps hidden.
	if _, err := svc.UpdateAuthorFilm(ctx, author, id, nil, nil); !errors.Is(err, domain.ErrLinkPatchEmpty) {
		t.Errorf("empty: %v", err)
	}
	bad := "x"
	if _, err := svc.UpdateAuthorFilm(ctx, author, id, nil, &bad); !errors.Is(err, domain.ErrInvalidRelation) {
		t.Errorf("bad: %v", err)
	}
	hide := true
	if _, err := svc.UpdateAuthorFilm(ctx, author, id, &hide, nil); err != nil {
		t.Fatal(err)
	}
	rel := domain.RelationBiopic
	item, err := svc.UpdateAuthorFilm(ctx, author, id, nil, &rel)
	if err != nil || item.Relation != domain.RelationBiopic || !item.Hidden {
		t.Errorf("relation only: %v %+v", err, item)
	}
	if _, err := svc.UpdateAuthorFilm(ctx, author, uuid.New(), &hide, nil); !errors.Is(err, domain.ErrAuthorLinkNotFound) {
		t.Errorf("missing link: %v", err)
	}

	// A relation set by an admin survives a sync.
	if _, err := svc.SyncAuthors(ctx, []uuid.UUID{author}); err != nil {
		t.Fatal(err)
	}
	links, _ := svc.ListAdminByAuthor(ctx, author)
	var wd *domain.LinkedWork
	for i := range links {
		if links[i].Title == "Док" {
			wd = &links[i]
		}
	}
	if wd == nil {
		t.Fatal("wikidata film missing")
	}
	doc := domain.RelationDocumentary
	if _, err := svc.UpdateAuthorFilm(ctx, author, wd.ID, nil, &doc); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.SyncAuthors(ctx, []uuid.UUID{author}); err != nil {
		t.Fatal(err)
	}
	if got := scalar(t, db, "SELECT relation FROM author_screen_works WHERE screen_work_id = ?", wd.ID); got != doc {
		t.Errorf("relation after sync = %q", got)
	}
	// Deleting: wikidata 409, manual ok.
	if err := svc.DeleteAuthorFilm(ctx, author, wd.ID); !errors.Is(err, domain.ErrLinkNotManual) {
		t.Errorf("wikidata delete: %v", err)
	}
	if err := svc.DeleteAuthorFilm(ctx, author, id); err != nil {
		t.Errorf("manual delete: %v", err)
	}
	if err := svc.DeleteAuthorFilm(ctx, author, id); !errors.Is(err, domain.ErrAuthorLinkNotFound) {
		t.Errorf("again: %v", err)
	}
}

func TestListWorks_SearchUnlinkedPaging(t *testing.T) {
	svc, db := setupAuthors(t, nil, nil)
	book := mkBook(t, db, "published", "")
	ctx := context.Background()
	a := newWork("Master and Margarita")
	a.IMDbID = sptr("tt0120737")
	if _, err := svc.AddBookAdaptation(ctx, book, nil, a); err != nil {
		t.Fatal(err)
	}
	b := newWork("100% orphan_work")
	b.QID = sptr("Q77")
	res, _ := svc.AddBookAdaptation(ctx, book, nil, b)
	if err := svc.DeleteBookAdaptation(ctx, book, res.Item.ID); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		f    domain.WorkFilter
		want int64
	}{
		{"all", domain.WorkFilter{}, 2},
		{"title ci", domain.WorkFilter{Search: "MARGAR"}, 1},
		{"imdb", domain.WorkFilter{Search: "tt0120737"}, 1},
		{"qid", domain.WorkFilter{Search: "q77"}, 1},
		{"escaped percent", domain.WorkFilter{Search: "100%"}, 1},
		{"escaped underscore", domain.WorkFilter{Search: "p_a"}, 0},
		{"unlinked", domain.WorkFilter{Unlinked: true}, 1},
	}
	for _, c := range cases {
		l, err := svc.ListWorks(ctx, c.f)
		if err != nil || l.Total != c.want || int64(len(l.Items)) != c.want {
			t.Errorf("%s: %v total=%v", c.name, err, l)
		}
	}
	l, _ := svc.ListWorks(ctx, domain.WorkFilter{Page: -3, Limit: 5000})
	if l.Page != 1 || l.Limit != MaxWorkLimit {
		t.Errorf("clamp: %+v", l)
	}
	l, _ = svc.ListWorks(ctx, domain.WorkFilter{Search: "margar"})
	if l.Items[0].BookCount != 1 || l.Items[0].AuthorCount != 0 {
		t.Errorf("counts: %+v", l.Items[0])
	}
}
