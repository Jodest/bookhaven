package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
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
	// CreatedAuthors / CreatedGenres list every author/genre this import
	// created, in creation order, each at most once - including those
	// created while processing a row that then failed or was skipped
	// (they stay in the catalog). Always present, never nil.
	CreatedAuthors []ImportCreatedEntity `json:"created_authors"`
	CreatedGenres  []ImportCreatedEntity `json:"created_genres"`
}

// ImportCreatedEntity is one author or genre created by an import: its ID,
// its stored name, and the 1-based data row whose processing created it.
type ImportCreatedEntity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Row  int    `json:"row"`
}

// importRowCreated holds the entities created while processing one row.
type importRowCreated struct {
	authors []*catalogDomain.Author
	genres  []*catalogDomain.Genre
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
// ExternalSource/ExternalID (this is not the harvest path) and get
// defaultStatus (a valid book status; "" means CreateBook's default,
// published) - there is no per-row status override.
func (s *Service) ImportBooks(ctx context.Context, rows []ImportRowInput, defaultStatus string) ImportReport {
	report := ImportReport{
		Processed:      len(rows),
		Errors:         []ImportRowError{},
		CreatedAuthors: []ImportCreatedEntity{},
		CreatedGenres:  []ImportCreatedEntity{},
	}
	seenAuthors := make(map[string]struct{})
	seenGenres := make(map[string]struct{})

	for i, row := range rows {
		outcome, created, err := s.importRow(ctx, row, defaultStatus)
		for _, a := range created.authors {
			id := a.ID.String()
			if _, ok := seenAuthors[id]; ok {
				continue
			}
			seenAuthors[id] = struct{}{}
			report.CreatedAuthors = append(report.CreatedAuthors, ImportCreatedEntity{ID: id, Name: a.Name, Row: i + 1})
		}
		for _, g := range created.genres {
			id := g.ID.String()
			if _, ok := seenGenres[id]; ok {
				continue
			}
			seenGenres[id] = struct{}{}
			report.CreatedGenres = append(report.CreatedGenres, ImportCreatedEntity{ID: id, Name: g.Name, Row: i + 1})
		}
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
// record it and continue with the rest of the batch. The authors/genres it
// created are returned on every path (success, skip and error), because
// they stay in the catalog whatever the row's outcome.
func (s *Service) importRow(ctx context.Context, row ImportRowInput, defaultStatus string) (importOutcome, importRowCreated, error) {
	var created importRowCreated

	language := strings.TrimSpace(row.Language)
	if language == "" {
		language = defaultImportLanguage
	}

	authorIDs, createdAuthors, err := s.resolveOrCreateAuthorsByNames(ctx, row.Authors)
	created.authors = createdAuthors
	if err != nil {
		return 0, created, fmt.Errorf("resolve authors: %w", err)
	}
	if len(authorIDs) == 0 {
		return 0, created, errImportNoAuthors
	}

	genreIDs, createdGenres, err := s.resolveOrCreateGenresByNames(ctx, row.Genres)
	created.genres = createdGenres
	if err != nil {
		return 0, created, fmt.Errorf("resolve genres: %w", err)
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
	}

	if defaultStatus != "" {
		status := defaultStatus
		input.Status = &status
	}

	if _, err := s.CreateBook(ctx, input); err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			return importOutcomeSkipped, created, nil
		}
		return 0, created, err
	}
	return importOutcomeCreated, created, nil
}
