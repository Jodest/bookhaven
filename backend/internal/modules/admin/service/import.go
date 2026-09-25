package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// defaultImportLanguage is applied to a row when its language field is
// omitted, matching the single-book create form's own default
// (apps/admin/src/features/manage-book/ui/book-form.tsx).
const defaultImportLanguage = "ru"

// errImportNoAuthors is the one import-specific validation rule this layer
// adds on top of CreateBook's own rules (E-02 in
// docs/features/catalog-manual-import.md): CreateBook itself does not
// require at least one author (the single-book form allows saving without
// one), but an imported row with zero resolvable author names is rejected
// as a row error.
var errImportNoAuthors = errors.New("row has no resolvable author names")

// ImportRowInput is one row of a bulk book import (CSV or JSON), already
// parsed into a structured shape by the transport layer (see
// admin/transport/http's CSV/JSON parsing). ImportBooks itself never reads
// a file.
type ImportRowInput struct {
	Title         string
	Authors       []string
	Genres        []string
	ISBN          *string
	Description   *string
	CoverURL      *string
	PublishedYear *int
	PageCount     *int
	Language      string
	Slug          string
}

// ImportReport is the JSON report returned by POST /admin/books/import,
// mirroring harvest/service.HarvestResult's shape (processed/created/
// skipped counts + per-row errors) for the same "aggregate outcome of
// independent per-record processing" pattern.
type ImportReport struct {
	Processed       int              `json:"processed"`
	Created         int              `json:"created"`
	SkippedExisting int              `json:"skipped_existing"`
	Errors          []ImportRowError `json:"errors"`
}

// ImportRowError describes a single failed row. Row is 1-based over data
// rows (a CSV header row, if any, is not counted).
type ImportRowError struct {
	Row     int    `json:"row"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

type importOutcome int

const (
	importOutcomeCreated importOutcome = iota
	importOutcomeSkipped
)

// ImportBooks creates one book per row, independently: a failure on one row
// is recorded in the report and does not prevent the rest of the batch from
// being processed (FR-004). It reuses CreateBook for all validation/
// creation rules and ResolveOrCreateAuthorIDsByNames/
// ResolveOrCreateGenreIDsByNames for author/genre matching - no parallel
// logic is implemented here (NFR-001). Imported books never set
// ExternalSource/ExternalID (this is not the harvest path) and default to
// defaultHidden - there is no per-row visibility override in this
// increment.
func (s *Service) ImportBooks(ctx context.Context, rows []ImportRowInput, defaultHidden bool) ImportReport {
	report := ImportReport{Processed: len(rows), Errors: []ImportRowError{}}

	for i, row := range rows {
		outcome, err := s.importRow(ctx, row, defaultHidden)
		if err != nil {
			report.Errors = append(report.Errors, ImportRowError{
				Row:     i + 1,
				Title:   row.Title,
				Message: err.Error(),
			})
			continue
		}
		switch outcome {
		case importOutcomeCreated:
			report.Created++
		case importOutcomeSkipped:
			report.SkippedExisting++
		}
	}

	return report
}

// importRow validates, resolves authors/genres, and creates a single row's
// book. It returns a non-nil error only for *this row's* failure - callers
// record it and continue with the rest of the batch.
func (s *Service) importRow(ctx context.Context, row ImportRowInput, defaultHidden bool) (importOutcome, error) {
	language := strings.TrimSpace(row.Language)
	if language == "" {
		language = defaultImportLanguage
	}

	authorIDs, err := s.ResolveOrCreateAuthorIDsByNames(ctx, row.Authors)
	if err != nil {
		return 0, fmt.Errorf("resolve authors: %w", err)
	}
	if len(authorIDs) == 0 {
		return 0, errImportNoAuthors
	}

	genreIDs, err := s.ResolveOrCreateGenreIDsByNames(ctx, row.Genres)
	if err != nil {
		return 0, fmt.Errorf("resolve genres: %w", err)
	}

	input := BookInput{
		Title:         row.Title,
		Slug:          row.Slug,
		Description:   row.Description,
		CoverURL:      row.CoverURL,
		ISBN:          row.ISBN,
		PublishedYear: row.PublishedYear,
		PageCount:     row.PageCount,
		Language:      language,
		AuthorIDs:     authorIDs,
		GenreIDs:      genreIDs,
		IsHidden:      defaultHidden,
	}

	if _, err := s.CreateBook(ctx, input); err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			return importOutcomeSkipped, nil
		}
		return 0, err
	}
	return importOutcomeCreated, nil
}
