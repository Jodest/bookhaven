package service

import (
	"context"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// bulkBookStatusCap bounds one BulkSetBookStatus call, matching the author
// and genre bulk-status caps (docs/features/book-status-model.md rule 7).
const bulkBookStatusCap = 100

// BookBulkStatusReport is the JSON report of POST /admin/books/bulk-status.
// Returned with HTTP 200 regardless of per-row outcomes.
type BookBulkStatusReport struct {
	Processed int                      `json:"processed"`
	Updated   int                      `json:"updated"`
	Errors    []BookBulkStatusRowError `json:"errors"`
}

// BookBulkStatusRowError describes one book whose status was not changed.
type BookBulkStatusRowError struct {
	BookID  string `json:"book_id"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

// BulkSetBookStatus sets status on every book in ids (deduplicated, at most
// bulkBookStatusCap). Each id is processed independently: an unknown id
// ("book not found") or a failed update is a row error and the rest
// proceed. Only status (and updated_at) changes; no other field is touched
// and no content re-validation runs.
func (s *Service) BulkSetBookStatus(ctx context.Context, ids []uuid.UUID, status string) (BookBulkStatusReport, error) {
	if !catalogDomain.IsValidBookStatus(status) {
		return BookBulkStatusReport{}, ErrInvalidBookStatus
	}

	seen := make(map[uuid.UUID]struct{}, len(ids))
	unique := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	if len(unique) > bulkBookStatusCap {
		return BookBulkStatusReport{}, ErrBulkStatusTooLarge
	}

	report := BookBulkStatusReport{Processed: len(unique), Errors: []BookBulkStatusRowError{}}
	for _, id := range unique {
		found, err := s.repo.SetBookStatus(ctx, id, status)
		switch {
		case err != nil:
			report.Errors = append(report.Errors, BookBulkStatusRowError{BookID: id.String(), Message: err.Error()})
		case !found:
			report.Errors = append(report.Errors, BookBulkStatusRowError{BookID: id.String(), Message: ErrBookNotFound.Error()})
		default:
			report.Updated++
		}
	}
	return report, nil
}
