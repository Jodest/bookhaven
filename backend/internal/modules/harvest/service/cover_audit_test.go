package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

type fakeAuditBooks struct {
	books     []catalogDomain.Book
	updates   map[uuid.UUID]adminservice.BookInput
	updateErr error
}

func (f *fakeAuditBooks) ListBooksWithOpenLibraryCover(ctx context.Context) ([]catalogDomain.Book, error) {
	return f.books, nil
}

func (f *fakeAuditBooks) UpdateBook(ctx context.Context, id uuid.UUID, in adminservice.BookInput) (*catalogDomain.Book, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updates == nil {
		f.updates = map[uuid.UUID]adminservice.BookInput{}
	}
	f.updates[id] = in
	return &catalogDomain.Book{ID: id}, nil
}

type fakeAuditMatcher struct {
	byTitle map[string]*bookdata.ExternalBookRecord
	errFor  map[string]error
}

func (f *fakeAuditMatcher) FindMatchByTitleAuthor(ctx context.Context, q bookdata.TitleAuthorQuery) (*bookdata.ExternalBookRecord, error) {
	if err, ok := f.errFor[q.Title]; ok {
		return nil, err
	}
	if rec, ok := f.byTitle[q.Title]; ok {
		return rec, nil
	}
	return nil, bookdata.ErrNotFound
}

func auditBook(title, cover string) catalogDomain.Book {
	src := "wikidata"
	u := "https://covers.openlibrary.org/b/id/" + cover + "-L.jpg"
	isbn := "9780000000002"
	desc := "Описание книги на русском языке"
	return catalogDomain.Book{
		ID: uuid.New(), Title: title, Slug: title, CoverURL: &u, ISBN: &isbn, Description: &desc, Language: "ru",
		Status: catalogDomain.BookStatusPublished, ExternalSource: &src,
		Authors: []catalogDomain.Author{{ID: uuid.New(), Name: "Автор"}},
		Genres:  []catalogDomain.Genre{{ID: uuid.New(), Name: "Жанр"}},
	}
}

func matchRec(cover string) *bookdata.ExternalBookRecord {
	u := "https://covers.openlibrary.org/b/id/" + cover + "-L.jpg"
	return &bookdata.ExternalBookRecord{CoverURL: &u}
}

func TestRunCoverAudit_ReportMode(t *testing.T) {
	a := auditBook("А", "9157583")
	b := auditBook("Б", "9157583")
	c := auditBook("В", "3")
	d := auditBook("Г", "4")
	e := auditBook("Д", "5")
	f := auditBook("Е", "6")
	books := &fakeAuditBooks{books: []catalogDomain.Book{a, b, c, d, e, f}}
	matcher := &fakeAuditMatcher{
		byTitle: map[string]*bookdata.ExternalBookRecord{
			"А": matchRec("9157583"), "Б": matchRec("9157583"), "В": matchRec("3"), "Г": matchRec("999"),
		},
		errFor: map[string]error{"Д": errors.New("boom")},
	}

	rep, err := RunCoverAudit(context.Background(), books, matcher, CoverAuditOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(books.updates) != 0 {
		t.Fatal("report mode must not update")
	}
	reasons := map[string]string{}
	for _, fd := range rep.Findings {
		reasons[fd.Book.Title] = strings.Join(fd.Reasons, ",")
	}
	want := map[string]string{"А": "shared_cover", "Б": "shared_cover", "Г": "other_match", "Д": "check_error", "Е": "no_match"}
	for k, v := range want {
		if reasons[k] != v {
			t.Errorf("%s reasons = %q, want %q", k, reasons[k], v)
		}
	}
	if _, flagged := reasons["В"]; flagged {
		t.Error("В must be ok")
	}
	if rep.Checked != 6 || rep.OK != 1 || rep.Errors != 1 {
		t.Errorf("report = %+v", rep)
	}
	var out bytes.Buffer
	rep.Write(&out)
	if !strings.Contains(out.String(), "checked=6 flagged=5 ok=1 errors=1") {
		t.Errorf("output: %s", out.String())
	}
}

func TestRunCoverAudit_Apply(t *testing.T) {
	a := auditBook("А", "1")
	manual := auditBook("Ручная", "2")
	manual.ExternalSource = nil
	other := auditBook("Б", "3")
	books := &fakeAuditBooks{books: []catalogDomain.Book{a, manual, other}}
	unknown := uuid.New()

	rep, err := RunCoverAudit(context.Background(), books, &fakeAuditMatcher{}, CoverAuditOptions{
		Apply: true, IDs: []uuid.UUID{a.ID, manual.ID, unknown},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(books.updates) != 1 {
		t.Fatalf("updates = %d, want only the listed candidate", len(books.updates))
	}
	in := books.updates[a.ID]
	if in.CoverURL != nil || in.ISBN != nil || in.Status != nil {
		t.Errorf("cover/isbn/status not cleared/unchanged: %+v", in)
	}
	if in.Title != "А" || in.Slug != "А" || in.Language != "ru" || in.Description == nil ||
		len(in.AuthorIDs) != 1 || in.AuthorIDs[0] != a.Authors[0].ID.String() ||
		len(in.GenreIDs) != 1 || in.GenreIDs[0] != a.Genres[0].ID.String() {
		t.Errorf("other fields not preserved: %+v", in)
	}
	got := []string{rep.Applied[0].Result, rep.Applied[1].Result, rep.Applied[2].Result}
	want := []string{"cleared", "skipped: not a candidate", "skipped: unknown id"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("result %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestRunCoverAudit_ApplyUpdateErrorReported(t *testing.T) {
	a := auditBook("А", "1")
	books := &fakeAuditBooks{books: []catalogDomain.Book{a}, updateErr: adminservice.ErrDescriptionNotRussian}
	rep, err := RunCoverAudit(context.Background(), books, &fakeAuditMatcher{}, CoverAuditOptions{Apply: true, IDs: []uuid.UUID{a.ID}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(rep.Applied[0].Result, "error: ") {
		t.Errorf("result = %q", rep.Applied[0].Result)
	}
}
