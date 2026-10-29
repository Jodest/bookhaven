package service

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

type fakeFantLabIDSource struct {
	ids     map[string][]string
	failAll bool
	asked   [][]string
}

func (f *fakeFantLabIDSource) FetchFantLabWorkIDs(ctx context.Context, qids []string) (map[string][]string, error) {
	f.asked = append(f.asked, qids)
	if f.failAll {
		return nil, errors.New("boom")
	}
	return f.ids, nil
}

type fakeFantLabFinder struct {
	covers map[string]string // book title -> cover
	errs   map[string]error
	titles []string
}

func (f *fakeFantLabFinder) FindSelfTitledEditionCover(ctx context.Context, title string, workIDs []string) (string, string, error) {
	f.titles = append(f.titles, title)
	if err, ok := f.errs[title]; ok {
		return "", "", err
	}
	if c, ok := f.covers[title]; ok {
		return c, workIDs[0] + "/77", nil
	}
	return "", "", bookdata.ErrNotFound
}

const backfillFantLabCover = "https://fantlab.ru/images/editions/big/77"

func TestRunCoverBackfill_FantLabFindingForBooksWithoutOLFinding(t *testing.T) {
	a, b, c, d := backfillBook("А", "Q1"), backfillBook("Б", "Q2"), backfillBook("В", "Q3"), backfillBook("Г", "Q4")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a, b, c, d}}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}, "Q2": {"OL2W"}}}
	covers := &fakeCoverFinder{covers: map[string]string{"OL1W": backfillCover}} // Q2: OL miss
	flIDs := &fakeFantLabIDSource{ids: map[string][]string{"Q1": {"9"}, "Q2": {"5"}, "Q3": {"6"}}}
	fl := &fakeFantLabFinder{covers: map[string]string{"Б": backfillFantLabCover}}

	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{FantLab: fl, FantLabIDs: flIDs})
	if err != nil {
		t.Fatal(err)
	}
	// OL wins for Q1: FantLab is neither asked for it nor looked up.
	if len(flIDs.asked) != 1 || strings.Join(flIDs.asked[0], ",") != "Q2,Q3,Q4" {
		t.Fatalf("FantLab IDs asked for %v", flIDs.asked)
	}
	if strings.Join(fl.titles, ",") != "Б,В" {
		t.Fatalf("FantLab looked up %v", fl.titles)
	}
	if len(r.Findings) != 2 || r.FoundFantLab != 1 || r.WithFantLabIDs != 2 {
		t.Fatalf("report = %+v", r)
	}
	if got := r.Findings[1]; got.Book.ID != b.ID || got.SourceID != "fantlab:5/77" || got.Cover != backfillFantLabCover {
		t.Fatalf("finding = %+v", got)
	}
	// Q2's earlier OL no-cover line is replaced by the finding; Q3 is a FantLab miss.
	if len(r.NoCover) != 1 || r.NoCover[0].Book.ID != c.ID || r.NoCover[0].OLIDs[0] != "fantlab:6" {
		t.Fatalf("nocover = %+v", r.NoCover)
	}
	var buf bytes.Buffer
	r.Write(&buf)
	if !strings.Contains(buf.String(), "candidates=4 with_ids=2 with_fantlab_ids=2 found=1 found_fantlab=1 nocover=1 errors=0") {
		t.Fatalf("output:\n%s", buf.String())
	}
}

func TestRunCoverBackfill_FantLabReplacesOpenLibraryError(t *testing.T) {
	a := backfillBook("А", "Q1")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a}}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}}}
	covers := &fakeCoverFinder{errs: map[string]error{"OL1W": bookdata.ErrSourceUnavailable}}
	flIDs := &fakeFantLabIDSource{ids: map[string][]string{"Q1": {"9"}}}

	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{
		FantLab: &fakeFantLabFinder{covers: map[string]string{"А": backfillFantLabCover}}, FantLabIDs: flIDs})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) != 0 || len(r.Findings) != 1 {
		t.Fatalf("report = %+v", r)
	}
}

func TestRunCoverBackfill_FantLabErrorKeepsBothLines(t *testing.T) {
	a := backfillBook("А", "Q1")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a}}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}}}
	covers := &fakeCoverFinder{errs: map[string]error{"OL1W": bookdata.ErrSourceUnavailable}}
	flIDs := &fakeFantLabIDSource{ids: map[string][]string{"Q1": {"9"}}}

	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{
		FantLab: &fakeFantLabFinder{errs: map[string]error{"А": bookdata.ErrRateLimited}}, FantLabIDs: flIDs})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) != 2 || !strings.HasPrefix(r.Errors[1].Message, "fantlab: ") || len(r.Findings) != 0 {
		t.Fatalf("report = %+v", r)
	}
}

func TestRunCoverBackfill_FantLabIDsBatchErrorOnlyForBooksWithoutFinding(t *testing.T) {
	a, b := backfillBook("А", "Q1"), backfillBook("Б", "Q2")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a, b}}
	ids := &fakeIDSource{ids: map[string][]string{"Q1": {"OL1W"}}}
	covers := &fakeCoverFinder{covers: map[string]string{"OL1W": backfillCover}}

	r, err := RunCoverBackfill(context.Background(), books, ids, covers, CoverBackfillOptions{
		FantLab: &fakeFantLabFinder{}, FantLabIDs: &fakeFantLabIDSource{failAll: true}})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Errors) != 1 || r.Errors[0].Book.ID != b.ID || !strings.HasPrefix(r.Errors[0].Message, "wikidata: ") || len(r.Findings) != 1 {
		t.Fatalf("report = %+v", r)
	}
}

func TestRunCoverBackfill_ApplyFantLabFinding(t *testing.T) {
	a := backfillBook("А", "Q1")
	books := &fakeBackfillBooks{books: []catalogDomain.Book{a}}
	r, err := RunCoverBackfill(context.Background(), books, &fakeIDSource{}, &fakeCoverFinder{}, CoverBackfillOptions{
		Apply:      true,
		FantLab:    &fakeFantLabFinder{covers: map[string]string{"А": backfillFantLabCover}},
		FantLabIDs: &fakeFantLabIDSource{ids: map[string][]string{"Q1": {"9"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Applied) != 1 || r.Applied[0].Result != "updated" {
		t.Fatalf("applied = %+v", r.Applied)
	}
	if in := books.updates[a.ID]; in.CoverURL == nil || *in.CoverURL != backfillFantLabCover {
		t.Fatalf("update = %+v", in)
	}
}
