package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
)

func TestListGenreAnalysisCandidates_ExcludesAliasesAndRejected(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()

	pending := f.genre("Фентези", "fentezi", "pending", nil)
	published := f.genre("Фэнтези", "fentezi-2", "published", strPtr("Q132311"))
	f.genre("Мусор", "musor", "rejected", nil)
	f.alias("Fantasy", "fantasy", published, nil)
	f.book("Книга 1", false, published)
	f.book("Книга 2", true, published, pending)

	rows, err := f.repo.ListGenreAnalysisCandidates(ctx)
	if err != nil {
		t.Fatalf("candidates: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 candidates, got %d", len(rows))
	}
	byID := map[uuid.UUID]GenreModerationRow{}
	for _, r := range rows {
		byID[r.Genre.ID] = r
	}
	if byID[published].BooksTotal != 2 || byID[pending].BooksTotal != 1 {
		t.Fatalf("unexpected book counts: %+v", byID)
	}
	if q := byID[published].Genre.WikidataQID; q == nil || *q != "Q132311" || byID[pending].Genre.CreatedAt.IsZero() {
		t.Fatalf("unexpected mapping: %+v", byID[published])
	}
}

func TestGenresByIDs_SeesAliasesAndReportsMissingByAbsence(t *testing.T) {
	f := newGenreModRepo(t)
	ctx := context.Background()

	target := f.genre("Фэнтези", "fentezi", "published", nil)
	alias := f.alias("Фентези", "fentezi-alias", target, nil)
	rejected := f.genre("Мусор", "musor", "rejected", nil)
	missing := uuid.New()

	rows, err := f.repo.GenresByIDs(ctx, []uuid.UUID{target, alias, rejected, missing})
	if err != nil {
		t.Fatalf("by ids: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (missing absent), got %d", len(rows))
	}
	for _, r := range rows {
		switch r.ID {
		case alias:
			if r.MergedIntoID == nil || *r.MergedIntoID != target || r.Status != "rejected" {
				t.Fatalf("alias state: %+v", r)
			}
		case target:
			if r.MergedIntoID != nil || r.Status != "published" {
				t.Fatalf("target state: %+v", r)
			}
		case rejected:
			if r.Status != "rejected" {
				t.Fatalf("rejected state: %+v", r)
			}
		default:
			t.Fatalf("unexpected row %+v", r)
		}
	}
	if none, err := f.repo.GenresByIDs(ctx, nil); err != nil || len(none) != 0 {
		t.Fatalf("empty ids: %v %v", none, err)
	}
}
