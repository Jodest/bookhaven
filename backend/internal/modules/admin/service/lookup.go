package service

import (
	"context"
	"errors"
	"strings"

	"github.com/bookhaven/backend/internal/pkg/bookdata"
)

// ErrLookupQueryRequired is returned by LookupBook when neither isbn nor a
// title query (q) is provided. GET /admin/books/lookup maps this to 400.
var ErrLookupQueryRequired = errors.New("isbn or q is required")

// ErrLookupUnavailable is returned by LookupBook when no lookup client was
// configured via WithLookupClient. GET /admin/books/lookup maps this to
// 502, the same status used for a configured-but-unreachable source, since
// from the caller's perspective the outcome is identical: retry later or
// enter the data manually.
var ErrLookupUnavailable = errors.New("lookup source is not configured")

// LookupBook performs a single, synchronous external lookup (Open Library,
// in production) by ISBN or by title(+author), returning the normalized
// bookdata.ExternalBookRecord. It is read-only: it never writes to the
// database and never creates authors/genres/books - the caller (the admin,
// via the create-book form) decides whether/what to save via the existing
// CreateBook path.
//
// isbn takes priority when both isbn and query are provided, since an ISBN
// lookup is the more precise match.
func (s *Service) LookupBook(ctx context.Context, isbn, query, author string) (*bookdata.ExternalBookRecord, error) {
	isbn = strings.TrimSpace(isbn)
	query = strings.TrimSpace(query)
	author = strings.TrimSpace(author)

	if isbn == "" && query == "" {
		return nil, ErrLookupQueryRequired
	}
	if s.lookupClient == nil {
		return nil, ErrLookupUnavailable
	}

	if isbn != "" {
		return s.lookupClient.FindByISBN(ctx, isbn)
	}
	return s.lookupClient.FindByTitleAuthor(ctx, query, author)
}
