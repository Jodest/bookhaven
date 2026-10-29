package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/google/uuid"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

// Reasons a harvested book's Open Library cover is flagged by the audit.
const (
	CoverAuditSharedCover = "shared_cover"
	CoverAuditNoMatch     = "no_match"
	CoverAuditOtherMatch  = "other_match"
	CoverAuditCheckError  = "check_error"
)

var openLibraryCoverIDPattern = regexp.MustCompile(`/b/id/(\d+)-[SML]\.jpg`)

// CoverAuditBooks is the book source and updater the audit needs
// (adminservice.Service satisfies it).
type CoverAuditBooks interface {
	ListBooksWithOpenLibraryCover(ctx context.Context) ([]catalogDomain.Book, error)
	UpdateBook(ctx context.Context, id uuid.UUID, input adminservice.BookInput) (*catalogDomain.Book, error)
}

// CoverAuditMatcher is the strict Open Library title search
// (bookdata.OpenLibraryClient satisfies it).
type CoverAuditMatcher = titleAuthorMatcher

// CoverAuditOptions controls a run. Without Apply the audit is read-only.
type CoverAuditOptions struct {
	Apply bool
	IDs   []uuid.UUID
}

// CoverAuditFinding is one flagged candidate.
type CoverAuditFinding struct {
	Book    catalogDomain.Book
	Reasons []string
}

// CoverAuditApplyResult is the outcome for one requested ID.
type CoverAuditApplyResult struct {
	ID     uuid.UUID
	Result string
}

// CoverAuditReport is the result of RunCoverAudit.
type CoverAuditReport struct {
	Checked  int
	OK       int
	Errors   int
	Findings []CoverAuditFinding
	Applied  []CoverAuditApplyResult
}

func coverID(url *string) string {
	if url == nil {
		return ""
	}
	if m := openLibraryCoverIDPattern.FindStringSubmatch(*url); m != nil {
		return m[1]
	}
	return ""
}

func authorNames(b catalogDomain.Book) []string {
	names := make([]string, 0, len(b.Authors))
	for _, a := range b.Authors {
		names = append(names, a.Name)
	}
	return names
}

// RunCoverAudit reports harvested books whose Open Library cover came from a
// title search and looks wrong. With opts.Apply it clears the cover link and
// ISBN of exactly opts.IDs (only harvested Open Library candidates), through
// the normal book save. A per-book failure is recorded and never aborts the run.
func RunCoverAudit(ctx context.Context, books CoverAuditBooks, matcher CoverAuditMatcher, opts CoverAuditOptions) (*CoverAuditReport, error) {
	all, err := books.ListBooksWithOpenLibraryCover(ctx)
	if err != nil {
		return nil, fmt.Errorf("list books: %w", err)
	}

	perCover := make(map[string]int)
	byID := make(map[uuid.UUID]catalogDomain.Book, len(all))
	for _, b := range all {
		byID[b.ID] = b
		if id := coverID(b.CoverURL); id != "" {
			perCover[id]++
		}
	}
	isCandidate := func(b catalogDomain.Book) bool {
		return b.ExternalSource != nil && *b.ExternalSource == "wikidata" && coverID(b.CoverURL) != ""
	}

	report := &CoverAuditReport{}
	for _, b := range all {
		if !isCandidate(b) {
			continue
		}
		if err := ctx.Err(); err != nil {
			return report, err
		}
		report.Checked++
		var reasons []string
		id := coverID(b.CoverURL)
		if perCover[id] > 1 {
			reasons = append(reasons, CoverAuditSharedCover)
		}
		found, err := matcher.FindMatchByTitleAuthor(ctx, bookdata.TitleAuthorQuery{Title: b.Title, Authors: authorNames(b)})
		switch {
		case errors.Is(err, bookdata.ErrNotFound):
			reasons = append(reasons, CoverAuditNoMatch)
		case err != nil:
			report.Errors++
			reasons = append(reasons, CoverAuditCheckError)
		case coverID(found.CoverURL) != id:
			reasons = append(reasons, CoverAuditOtherMatch)
		}
		if len(reasons) == 0 {
			report.OK++
			continue
		}
		report.Findings = append(report.Findings, CoverAuditFinding{Book: b, Reasons: reasons})
	}

	if opts.Apply {
		for _, id := range opts.IDs {
			report.Applied = append(report.Applied, CoverAuditApplyResult{ID: id, Result: clearCover(ctx, books, byID, id, isCandidate)})
		}
	}
	return report, nil
}

func clearCover(ctx context.Context, books CoverAuditBooks, byID map[uuid.UUID]catalogDomain.Book, id uuid.UUID, isCandidate func(catalogDomain.Book) bool) string {
	b, ok := byID[id]
	if !ok {
		return "skipped: unknown id"
	}
	if !isCandidate(b) {
		return "skipped: not a candidate"
	}
	input := adminservice.BookInput{
		Title:         b.Title,
		Slug:          b.Slug,
		Description:   b.Description,
		PublishedYear: b.PublishedYear,
		PageCount:     b.PageCount,
		Language:      b.Language,
		CoverURL:      nil,
		ISBN:          nil,
		Status:        nil,
	}
	for _, a := range b.Authors {
		input.AuthorIDs = append(input.AuthorIDs, a.ID.String())
	}
	for _, g := range b.Genres {
		input.GenreIDs = append(input.GenreIDs, g.ID.String())
	}
	if _, err := books.UpdateBook(ctx, id, input); err != nil {
		return "error: " + err.Error()
	}
	return "cleared"
}

// Write prints the report as plain text, one line per flagged book and apply
// result, then the totals.
func (r *CoverAuditReport) Write(w io.Writer) {
	for _, f := range r.Findings {
		b := f.Book
		cover, isbn := "", ""
		if b.CoverURL != nil {
			cover = *b.CoverURL
		}
		if b.ISBN != nil {
			isbn = *b.ISBN
		}
		fmt.Fprintf(w, "%s %s %s %s | %s | %s | %s | has_description=%t\n",
			b.ID, b.Status, strings.Join(f.Reasons, ","), b.Title, strings.Join(authorNames(b), ", "), cover, isbn,
			b.Description != nil && *b.Description != "")
	}
	for _, a := range r.Applied {
		fmt.Fprintf(w, "%s %s\n", a.ID, a.Result)
	}
	fmt.Fprintf(w, "checked=%d flagged=%d ok=%d errors=%d\n", r.Checked, len(r.Findings), r.OK, r.Errors)
}
