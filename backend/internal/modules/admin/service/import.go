package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
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
	// created, in creation order, each at most once. Only rows whose book
	// was created create authors/genres (a failed or skipped row creates
	// nothing), so every entry comes from such a row.
	// Always present, never nil.
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
// published) - there is no per-row status override. A row changes the
// catalog only when its book is created: each row is fully checked before
// any write and its writes run in one transaction (see importRow).
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
// record it and continue with the rest of the batch.
//
// The row is fully checked before anything is written: the no-authors rule
// first, then prepareNewBook and the ISBN check (CreateBook's own rules, in
// CreateBook's order, so the slug-existence skip still wins over an invalid
// ISBN). Only then are the authors, the genres and the book written, in one
// transaction, so a failure at any step leaves nothing from this row in the
// catalog. The created authors/genres are therefore returned only on the
// created outcome; on error or skip the returned importRowCreated is empty.
func (s *Service) importRow(ctx context.Context, row ImportRowInput, defaultStatus string) (importOutcome, importRowCreated, error) {
	if !hasNonBlankName(row.Authors) {
		return 0, importRowCreated{}, errImportNoAuthors
	}

	language := strings.TrimSpace(row.Language)
	if language == "" {
		language = defaultImportLanguage
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
	}
	if defaultStatus != "" {
		status := defaultStatus
		input.Status = &status
	}

	// Pre-creation check: nothing has been written yet.
	if _, err := s.prepareNewBook(ctx, input); err != nil {
		if errors.Is(err, ErrDuplicateSlug) {
			return importOutcomeSkipped, importRowCreated{}, nil
		}
		return 0, importRowCreated{}, err
	}
	if _, err := normalizeNewBookISBN(input.ISBN); err != nil {
		return 0, importRowCreated{}, err
	}

	var created importRowCreated
	// bookErr is set only when CreateBook itself fails, so an
	// ErrDuplicateSlug from author/genre creation (e.g. a concurrent import
	// creating the same author) stays a row error instead of a skip.
	var bookErr error
	err := s.repo.WithinTransaction(ctx, func(txRepo repository.Repository) error {
		tx := s.withRepo(txRepo)

		authorIDs, createdAuthors, err := tx.resolveOrCreateAuthorsByNames(ctx, row.Authors)
		if err != nil {
			return fmt.Errorf("resolve authors: %w", err)
		}
		if len(authorIDs) == 0 {
			return errImportNoAuthors
		}
		genreIDs, createdGenres, err := tx.resolveOrCreateGenresByNames(ctx, row.Genres)
		if err != nil {
			return fmt.Errorf("resolve genres: %w", err)
		}

		input.AuthorIDs = authorIDs
		input.GenreIDs = genreIDs
		if _, err := tx.CreateBook(ctx, input); err != nil {
			bookErr = err
			return err
		}
		created = importRowCreated{authors: createdAuthors, genres: createdGenres}
		return nil
	})
	if err != nil {
		if bookErr != nil && errors.Is(bookErr, ErrDuplicateSlug) {
			return importOutcomeSkipped, importRowCreated{}, nil
		}
		return 0, importRowCreated{}, err
	}
	// After the commit (CreateBook inside the tx has no invalidator). Only
	// a published row changes the visible similar-books lists.
	if input.Status == nil || *input.Status == catalogDomain.BookStatusPublished {
		s.invalidateSimilar(ctx)
	}
	return importOutcomeCreated, created, nil
}

// hasNonBlankName reports whether names has at least one non-blank entry.
// A non-blank name always resolves to an author (matched or created) unless
// a write fails, so this is the DB-free form of "has a resolvable author".
func hasNonBlankName(names []string) bool {
	for _, n := range names {
		if strings.TrimSpace(n) != "" {
			return true
		}
	}
	return false
}
