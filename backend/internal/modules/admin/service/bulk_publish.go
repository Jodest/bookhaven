package service

import (
	"context"
	"errors"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
)

// bulkPublishCap bounds the number of pending-review books one
// BulkPublishBooks call will process, matching maxImportRows
// (backend/internal/modules/admin/transport/http/import_parse.go) - the
// same "bound a synchronous admin batch HTTP call" precedent, applied here
// to the "publish every row matching the current pending_review (+search)
// filter" action (docs/features/harvest-bulk-publish.md).
const bulkPublishCap = 500

// ErrBulkPublishTooLarge is returned by BulkPublishBooks, without
// publishing anything, when more than bulkPublishCap books currently match
// the pending_review predicate (optionally narrowed by search). The caller
// (admin/transport/http.Handler.BulkPublishBooks) maps it to a 422 telling
// the admin to narrow the search.
var ErrBulkPublishTooLarge = errors.New("too many pending-review books match; narrow the search and try again, or review them in smaller batches")

// BulkPublishReport is the JSON report returned by
// POST /admin/books/bulk-publish, mirroring ImportReport's (import.go)
// "aggregate outcome of independent per-record processing" shape. Returned
// with HTTP 200 regardless of per-row outcomes.
type BulkPublishReport struct {
	Processed int                   `json:"processed"`
	Published int                   `json:"published"`
	Errors    []BulkPublishRowError `json:"errors"`
}

// BulkPublishRowError describes one book that failed to publish.
type BulkPublishRowError struct {
	BookID  string `json:"book_id"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// BulkPublishBooks publishes (sets is_hidden = false on) every book
// currently matching the pending_review predicate (is_hidden = true AND
// external_source IS NOT NULL), optionally narrowed by search - the exact
// same predicate/search ListBooks already applies for the admin books
// list's pending_review filter. The predicate is hardcoded here
// (Visibility: "pending_review") and is never derived from caller input, so
// this can never be used to mass-publish books that are not harvested-
// and-pending, regardless of what else a caller sends.
//
// If more than bulkPublishCap books match, nothing is published and
// ErrBulkPublishTooLarge is returned (all-or-nothing only for this "is the
// batch too big to run" guard). Otherwise each matching row is processed
// independently by building a BookInput from the row's already-loaded
// fields and calling the existing UpdateBook method/validation path
// unchanged: a failure on one row is recorded in the returned report and
// does not stop the rest of the batch, mirroring ImportBooks' established
// "continue on row error" control flow (import.go) exactly.
func (s *Service) BulkPublishBooks(ctx context.Context, search string) (BulkPublishReport, error) {
	books, total, err := s.repo.ListBooks(ctx, repository.BookFilter{
		Visibility: "pending_review",
		Search:     search,
		Limit:      bulkPublishCap + 1,
	})
	if err != nil {
		return BulkPublishReport{}, err
	}
	if total > int64(bulkPublishCap) {
		return BulkPublishReport{}, ErrBulkPublishTooLarge
	}

	report := BulkPublishReport{Processed: len(books), Errors: []BulkPublishRowError{}}
	for _, book := range books {
		if err := s.publishHarvestedBook(ctx, book); err != nil {
			report.Errors = append(report.Errors, BulkPublishRowError{
				BookID:  book.ID.String(),
				Title:   book.Title,
				Message: err.Error(),
			})
			continue
		}
		report.Published++
	}

	return report, nil
}

// publishHarvestedBook builds a BookInput from a pending-review book's
// already-loaded fields (unchanged except IsHidden: false) and calls the
// existing UpdateBook method - the same method/validation path the
// single-row frontend "Опубликовать" action already drives via
// PATCH /admin/books/:id. No new validation logic is introduced here.
func (s *Service) publishHarvestedBook(ctx context.Context, book catalogDomain.Book) error {
	authorIDs := make([]string, 0, len(book.Authors))
	for _, a := range book.Authors {
		authorIDs = append(authorIDs, a.ID.String())
	}
	genreIDs := make([]string, 0, len(book.Genres))
	for _, g := range book.Genres {
		genreIDs = append(genreIDs, g.ID.String())
	}

	input := BookInput{
		Title:         book.Title,
		Slug:          book.Slug,
		Description:   book.Description,
		CoverURL:      book.CoverURL,
		ISBN:          book.ISBN,
		PublishedYear: book.PublishedYear,
		PageCount:     book.PageCount,
		Language:      book.Language,
		AuthorIDs:     authorIDs,
		GenreIDs:      genreIDs,
		IsHidden:      false,
	}

	_, err := s.UpdateBook(ctx, book.ID, input)
	return err
}
