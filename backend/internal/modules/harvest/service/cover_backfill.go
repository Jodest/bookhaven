package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/uuid"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

// CoverBackfillBooks is the book source and updater the backfill needs
// (adminservice.Service satisfies it).
type CoverBackfillBooks interface {
	ListWikidataBooksWithoutCover(ctx context.Context) ([]catalogDomain.Book, error)
	GetBook(ctx context.Context, id uuid.UUID) (*catalogDomain.Book, error)
	UpdateBook(ctx context.Context, id uuid.UUID, input adminservice.BookInput) (*catalogDomain.Book, error)
}

// OpenLibraryIDSource reads Open Library IDs (P648) from Wikidata
// (bookdata.WikidataClient satisfies it).
type OpenLibraryIDSource interface {
	FetchOpenLibraryIDs(ctx context.Context, qids []string) (map[string][]string, error)
}

// CoverByIDFinder finds a cover by Open Library IDs (bookdata.OpenLibraryClient
// satisfies it).
type CoverByIDFinder = openLibraryCoverFinder

// CoverBackfillOptions controls a run. Without Apply the backfill is read-only.
// IDs, when not empty, limits Apply to those book IDs.
type CoverBackfillOptions struct {
	Apply bool
	IDs   []uuid.UUID
}

// CoverBackfillFinding is one candidate for which a cover was found.
type CoverBackfillFinding struct {
	Book  catalogDomain.Book
	QID   string
	OLID  string
	Cover string
}

// CoverBackfillLine is a candidate with no cover found or a failed check.
type CoverBackfillLine struct {
	Book    catalogDomain.Book
	QID     string
	OLIDs   []string
	Message string
}

// CoverBackfillApplyResult is the outcome for one book.
type CoverBackfillApplyResult struct {
	ID     uuid.UUID
	Result string
}

// CoverBackfillReport is the result of RunCoverBackfill.
type CoverBackfillReport struct {
	Candidates int
	WithIDs    int
	Findings   []CoverBackfillFinding
	NoCover    []CoverBackfillLine
	Errors     []CoverBackfillLine
	Skipped    []CoverBackfillLine
	Applied    []CoverBackfillApplyResult
}

// RunCoverBackfill finds Wikidata books without a cover link, reads their Open
// Library IDs from Wikidata in batches and looks for a cover by those IDs.
// With opts.Apply it sets the cover link of the findings (all, or only
// opts.IDs) through the normal book save. A per-book or per-batch failure is
// recorded and never aborts the run.
func RunCoverBackfill(ctx context.Context, books CoverBackfillBooks, ids OpenLibraryIDSource, covers CoverByIDFinder, opts CoverBackfillOptions) (*CoverBackfillReport, error) {
	candidates, err := books.ListWikidataBooksWithoutCover(ctx)
	if err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}
	report := &CoverBackfillReport{Candidates: len(candidates)}
	byID := make(map[uuid.UUID]catalogDomain.Book, len(candidates))

	valid := make([]catalogDomain.Book, 0, len(candidates))
	for _, b := range candidates {
		byID[b.ID] = b
		qid := ""
		if b.ExternalID != nil {
			qid = *b.ExternalID
		}
		if !bookdata.IsWikidataQID(qid) {
			report.Skipped = append(report.Skipped, CoverBackfillLine{Book: b, QID: qid, Message: "invalid qid"})
			continue
		}
		valid = append(valid, b)
	}

	findings := map[uuid.UUID]CoverBackfillFinding{}
	for start := 0; start < len(valid); start += bookdata.MaxOpenLibraryIDBooks {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		end := min(start+bookdata.MaxOpenLibraryIDBooks, len(valid))
		batch := valid[start:end]
		qids := make([]string, len(batch))
		for i, b := range batch {
			qids[i] = *b.ExternalID
		}
		idsByQID, err := ids.FetchOpenLibraryIDs(ctx, qids)
		if err != nil {
			for _, b := range batch {
				report.Errors = append(report.Errors, CoverBackfillLine{Book: b, QID: *b.ExternalID, Message: "wikidata: " + err.Error()})
			}
			continue
		}
		for _, b := range batch {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			qid := *b.ExternalID
			olids := idsByQID[qid]
			if len(olids) == 0 {
				continue
			}
			report.WithIDs++
			cover, used, err := covers.FindCoverByOpenLibraryIDs(ctx, olids)
			switch {
			case errors.Is(err, bookdata.ErrNotFound):
				report.NoCover = append(report.NoCover, CoverBackfillLine{Book: b, QID: qid, OLIDs: olids})
			case err != nil:
				report.Errors = append(report.Errors, CoverBackfillLine{Book: b, QID: qid, OLIDs: olids, Message: err.Error()})
			default:
				f := CoverBackfillFinding{Book: b, QID: qid, OLID: used, Cover: cover}
				report.Findings = append(report.Findings, f)
				findings[b.ID] = f
			}
		}
	}

	if opts.Apply {
		if len(opts.IDs) == 0 {
			for _, f := range report.Findings {
				report.Applied = append(report.Applied, CoverBackfillApplyResult{ID: f.Book.ID, Result: setCover(ctx, books, f)})
			}
		} else {
			for _, id := range opts.IDs {
				report.Applied = append(report.Applied, CoverBackfillApplyResult{ID: id, Result: applyListed(ctx, books, byID, findings, id)})
			}
		}
	}
	return report, nil
}

func applyListed(ctx context.Context, books CoverBackfillBooks, byID map[uuid.UUID]catalogDomain.Book, findings map[uuid.UUID]CoverBackfillFinding, id uuid.UUID) string {
	if _, ok := byID[id]; !ok {
		return "skipped: unknown id"
	}
	f, ok := findings[id]
	if !ok {
		return "skipped: not a candidate"
	}
	return setCover(ctx, books, f)
}

// setCover re-reads the book and, when it still has no cover, saves it with
// the found cover link and everything else as stored.
func setCover(ctx context.Context, books CoverBackfillBooks, f CoverBackfillFinding) string {
	b, err := books.GetBook(ctx, f.Book.ID)
	if err != nil {
		return "error: " + err.Error()
	}
	if b.CoverURL != nil && *b.CoverURL != "" {
		return "skipped: has cover"
	}
	link := f.Cover
	input := adminservice.BookInput{
		Title:         b.Title,
		Slug:          b.Slug,
		Description:   b.Description,
		PublishedYear: b.PublishedYear,
		PageCount:     b.PageCount,
		Language:      b.Language,
		CoverURL:      &link,
		ISBN:          b.ISBN,
		Status:        nil,
	}
	for _, a := range b.Authors {
		input.AuthorIDs = append(input.AuthorIDs, a.ID.String())
	}
	for _, g := range b.Genres {
		input.GenreIDs = append(input.GenreIDs, g.ID.String())
	}
	if _, err := books.UpdateBook(ctx, b.ID, input); err != nil {
		return "error: " + err.Error()
	}
	return "updated"
}

// Write prints the report as plain text, one line per finding, no-cover book,
// error and apply result, then the totals.
func (r *CoverBackfillReport) Write(w io.Writer) {
	for _, f := range r.Findings {
		fmt.Fprintf(w, "found %s %s %s %s %s %s\n", f.Book.ID, f.Book.Status, f.QID, f.OLID, f.Cover, f.Book.Title)
	}
	for _, l := range r.NoCover {
		fmt.Fprintf(w, "nocover %s %s %s %s\n", l.Book.ID, l.QID, strings.Join(l.OLIDs, ","), l.Book.Title)
	}
	for _, l := range r.Errors {
		fmt.Fprintf(w, "error %s %s %s\n", l.Book.ID, l.QID, l.Message)
	}
	for _, l := range r.Skipped {
		fmt.Fprintf(w, "skipped %s %q %s\n", l.Book.ID, l.QID, l.Message)
	}
	for _, a := range r.Applied {
		fmt.Fprintf(w, "%s %s\n", a.ID, a.Result)
	}
	fmt.Fprintf(w, "candidates=%d with_ids=%d found=%d nocover=%d errors=%d\n",
		r.Candidates, r.WithIDs, len(r.Findings), len(r.NoCover), len(r.Errors))
}
