package service

import (
	"bytes"
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/google/uuid"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

type fakeBackfillBooks struct {
	books     []catalogDomain.Book
	reread    map[uuid.UUID]catalogDomain.Book // overrides GetBook
	updates   map[uuid.UUID]adminservice.BookInput
	updateErr error
}

func (f *fakeBackfillBooks) ListWikidataBooksWithoutCover(ctx context.Context) ([]catalogDomain.Book, error) {
	return f.books, nil
}

func (f *fakeBackfillBooks) GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error) {
	if b, ok := f.reread[id]; ok {
		return &b, nil
	}
	for _, b := range f.books {
		if b.ID == id {
			return &b, nil
		}
	}
	return nil, errors.New("not found")
}

func (f *fakeBackfillBooks) UpdateBook(ctx context.Context, id uuid.UUID, in adminservice.BookInput) (*catalogDomain.Book, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	if f.updates == nil {
		f.updates = map[uuid.UUID]adminservice.BookInput{}
	}
	f.updates[id] = in
	return &catalogDomain.Book{ID: id}, nil
}

type fakeIDSource struct {
	ids      map[string][]string
	failFor  map[string]bool // fail any batch containing this QID
	batchLen []int
}

func (f *fakeIDSource) FetchOpenLibraryIDs(ctx context.Context, qids []string) (map[string][]string, error) {
	f.batchLen = append(f.batchLen, len(qids))
	for _, q := range qids {
		if f.failFor[q] {
			return nil, errors.New("boom")
		}
	}
	return f.ids, nil
}

type fakeCoverFinder struct {
	covers map[string]string // first ID -> cover
	errs   map[string]error
	calls  int
}

func (f *fakeCoverFinder) FindCoverByOpenLibraryIDs(ctx context.Context, ids []string) (string, string, error) {
	f.calls++
	if err, ok := f.errs[ids[0]]; ok {
		return "", "", err
	}
	if c, ok := f.covers[ids[0]]; ok {
		return c, ids[0], nil
	}
	return "", "", bookdata.ErrNotFound
}

func backfillBook(title, qid string) catalogDomain.Book {
	src := "wikidata"
	isbn := "9780000000002"
	desc := "Описание книги на русском языке"
	b := catalogDomain.Book{
		ID: uuid.New(), Title: title, Slug: title, ISBN: &isbn, Description: &desc, Language: "ru",
		Status: catalogDomain.BookStatusPending, ExternalSource: &src,
		Authors: []catalogDomain.Author{{ID: uuid.New(), Name: "Автор"}},
		Genres:  []catalogDomain.Genre{{ID: uuid.New(), Name: "Жанр"}},
	}
	b.ExternalID = &qid
	return b
}

const backfillCover = "https://covers.openlibrary.org/b/id/42-L.jpg"

func TestRunCoverBackfill_ReportOnlyChangesNothing(t *testing.T) {
	a, b, c := backfillBook("А", "Q1"), backfillBook("Б", "Q2"), backfillBook("В", "Q3")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a, b, c}}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}, "Q2": {"OL2W"}}}
	covers := &fakeCoverFinder{covers: map[string]string{"OL1W": backfillCover}}

	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if r.Candidates != 3 || r.WithIDs != 2 || len(r.Findings) != 1 || len(r.NoCover) != 1 || len(r.Errors) != 0 {
		t.Fatalf("report = %+v", r)
	}
	if len(books.updates) != 0 || len(r.Applied) != 0 {
		t.Fatalf("report-only wrote: %v %v", books.updates, r.Applied)
	}
	var buf bytes.Buffer
	r.Write(&buf)
	out := buf.String()
	if !strings.Contains(out, "found "+a.ID.String()) || !strings.Contains(out, "nocover "+b.ID.String()) ||
		!strings.Contains(out, "candidates=3 with_ids=2 with_fantlab_ids=0 found=1 found_fantlab=0 nocover=1 errors=0") {
		t.Fatalf("output:\n%s", out)
	}
}

func TestRunCoverBackfill_ApplyUpdatesFindingsKeepingInput(t *testing.T) {
	a, b := backfillBook("А", "Q1"), backfillBook("Б", "Q2")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a, b}}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}, "Q2": {"OL2W"}}}
	covers := &fakeCoverFinder{covers: map[string]string{"OL1W": backfillCover}}

	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(books.updates) != 1 || len(r.Applied) != 1 || r.Applied[0].Result != "updated" {
		t.Fatalf("updates=%v applied=%v", books.updates, r.Applied)
	}
	in := books.updates[a.ID]
	if in.CoverURL == nil || *in.CoverURL != backfillCover {
		t.Fatalf("CoverURL = %v", in.CoverURL)
	}
	if in.ISBN == nil || *in.ISBN != *a.ISBN || in.Description == nil || *in.Description != *a.Description ||
		in.Status != nil || in.Title != a.Title || in.Slug != a.Slug || in.Language != "ru" {
		t.Fatalf("input changed more than the cover: %+v", in)
	}
	if len(in.AuthorIDs) != 1 || in.AuthorIDs[0] != a.Authors[0].ID.String() ||
		len(in.GenreIDs) != 1 || in.GenreIDs[0] != a.Genres[0].ID.String() {
		t.Fatalf("authors/genres lost: %+v", in)
	}
}

func TestRunCoverBackfill_ApplyOnlyListedIDs(t *testing.T) {
	a, b, c := backfillBook("А", "Q1"), backfillBook("Б", "Q2"), backfillBook("В", "Q3")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a, b, c}}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}, "Q2": {"OL2W"}}}
	covers := &fakeCoverFinder{covers: map[string]string{"OL1W": backfillCover, "OL2W": backfillCover}}
	unknown := uuid.New()

	r, err := RunCoverBackfill(context.Background(), books, ids, covers,
		CoverBackfillOptions{Apply: true, IDs: []uuid.UUID{b.ID, c.ID, unknown}})
	if err != nil {
		t.Fatal(err)
	}
	if len(books.updates) != 1 {
		t.Fatalf("updates = %v", books.updates)
	}
	if _, ok := books.updates[b.ID]; !ok {
		t.Fatal("listed finding not updated")
	}
	want := []string{"updated", "skipped: not a candidate", "skipped: unknown id"}
	for i, w := range want {
		if r.Applied[i].Result != w {
			t.Errorf("applied[%d] = %q, want %q", i, r.Applied[i].Result, w)
		}
	}
}

func TestRunCoverBackfill_SkipsBookThatGotACover(t *testing.T) {
	a := backfillBook("А", "Q1")
	now := "https://example.com/new.jpg"
	withCover := a
	withCover.CoverURL = &now
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a}, reread: map[uuid.UUID]catalogDomain.Book{a.ID: withCover}}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}}}
	covers := &fakeCoverFinder{covers: map[string]string{"OL1W": backfillCover}}

	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(books.updates) != 0 || r.Applied[0].Result != "skipped: has cover" {
		t.Fatalf("updates=%v applied=%v", books.updates, r.Applied)
	}
}

func TestRunCoverBackfill_WikidataBatchErrorContinues(t *testing.T) {
	var list []catalogDomain.Book
	for i := 0; i < bookdata.MaxOpenLibraryIDBooks+1; i++ {
		list = append(list, backfillBook("Т", "Q"+strconv.Itoa(1000+i)))
	}
	last := *list[len(list)-1].ExternalID
	books := &fakeBackfillBooks{books: list}
	ids := &fakeIDSource{ids: map[string][]string{last: {"OL1W"}}, failFor: map[string]bool{*list[0].ExternalID: true}}
	covers := &fakeCoverFinder{covers: map[string]string{"OL1W": backfillCover}}

	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ids.batchLen) != 2 || ids.batchLen[0] != bookdata.MaxOpenLibraryIDBooks || ids.batchLen[1] != 1 {
		t.Fatalf("batches = %v", ids.batchLen)
	}
	if len(r.Errors) != bookdata.MaxOpenLibraryIDBooks || !strings.HasPrefix(r.Errors[0].Message, "wikidata: ") {
		t.Fatalf("errors = %d %v", len(r.Errors), r.Errors[0].Message)
	}
	if len(r.Findings) != 1 {
		t.Fatalf("second batch not processed: %+v", r.Findings)
	}
}

func TestRunCoverBackfill_OpenLibraryErrorIsPerBook(t *testing.T) {
	a, b := backfillBook("А", "Q1"), backfillBook("Б", "Q2")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a, b}}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}, "Q2": {"OL2W"}}}
	covers := &fakeCoverFinder{
		covers: map[string]string{"OL2W": backfillCover},
		errs:   map[string]error{"OL1W": bookdata.ErrSourceUnavailable},
	}
	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) != 1 || r.Errors[0].Book.ID != a.ID || len(r.Findings) != 1 || r.Findings[0].Book.ID != b.ID {
		t.Fatalf("report = %+v", r)
	}
}

func TestRunCoverBackfill_InvalidQIDSkipped(t *testing.T) {
	a := backfillBook("А", "not-a-qid")
	b := backfillBook("Б", "Q2")
	b.ExternalID = nil
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a, b}}
	ids := &fakeIDSource{}
	covers := &fakeCoverFinder{}
	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Skipped) != 2 || len(ids.batchLen) != 0 || covers.calls != 0 {
		t.Fatalf("skipped=%d batches=%v calls=%d", len(r.Skipped), ids.batchLen, covers.calls)
	}
}

func TestRunCoverBackfill_UpdateErrorIsReported(t *testing.T) {
	a := backfillBook("А", "Q1")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a}, updateErr: errors.New("description invalid")}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}}}
	covers := &fakeCoverFinder{covers: map[string]string{"OL1W": backfillCover}}
	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{Apply: true})
	if err != nil {
		t.Fatal(err)
	}
	if r.Applied[0].Result != "error: description invalid" {
		t.Fatalf("applied = %v", r.Applied)
	}
}
