package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	aimodDomain "github.com/bookhaven/backend/internal/modules/aimoderation/domain"
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
	if report.Updated > 0 {
		s.invalidateSimilar(ctx)
	}
	return report, nil
}

// Book moderation queue (docs/features/book-moderation-queue.md).

const (
	bookQueueDefaultLimit = 20
	bookQueueMaxLimit     = 50
	// maxBookHints bounds the "possibly the same book" hints per queue row.
	maxBookHints = 5
)

// Book hint match kinds, strongest first.
const (
	BookMatchSameISBN        = "same_isbn"
	BookMatchSameTitleAuthor = "same_title_author"
)

// BookHint is a display-only "possibly the same book" suggestion.
type BookHint struct {
	Book  repository.BookHintCandidate
	Match string
}

// BookQueueItem is one row of the book moderation queue.
type BookQueueItem struct {
	Book    catalogDomain.Book
	Similar []BookHint
	// AIVerdict is the book's AI pre-moderation verdict, nil when it has
	// none.
	AIVerdict *BookAIVerdictView
}

// ListBookModerationQueue returns one page of the book moderation queue
// (default status pending, newest first) with authors/genres preloaded and
// "possibly the same book" hints and AI verdicts. Hints use at most two
// extra queries per page (ISBN candidates, shared-author candidates),
// consider books of any status and are recomputed on every call; verdicts
// add one query.
func (s *Service) ListBookModerationQueue(ctx context.Context, filter repository.BookModerationFilter, page, limit int) ([]BookQueueItem, int64, error) {
	if filter.Status == "" {
		filter.Status = catalogDomain.BookStatusPending
	}
	if !catalogDomain.IsValidBookStatus(filter.Status) {
		return nil, 0, fmt.Errorf("%w: status must be pending, published or rejected", ErrInvalidModerationFilter)
	}
	switch filter.Source {
	case "", "manual", "harvested":
	default:
		return nil, 0, fmt.Errorf("%w: source must be manual or harvested", ErrInvalidModerationFilter)
	}
	switch filter.AI {
	case "", AIFilterNone, aimodDomain.VerdictPublish, aimodDomain.VerdictReject, aimodDomain.VerdictNeedsHuman:
	default:
		return nil, 0, fmt.Errorf("%w: ai must be publish, reject, needs_human or none", ErrInvalidModerationFilter)
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = bookQueueDefaultLimit
	}
	if limit > bookQueueMaxLimit {
		limit = bookQueueMaxLimit
	}
	filter.Limit = limit
	filter.Offset = (page - 1) * limit

	books, total, err := s.repo.ListBooksForModeration(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list books for moderation: %w", err)
	}
	items := make([]BookQueueItem, 0, len(books))
	if len(books) == 0 {
		return items, total, nil
	}

	hints, err := s.bookHintsFor(ctx, books)
	if err != nil {
		return nil, 0, err
	}
	verdicts, err := s.aiVerdictViews(ctx, books)
	if err != nil {
		return nil, 0, err
	}
	for i, b := range books {
		items = append(items, BookQueueItem{
			Book:      b,
			Similar:   hints[i],
			AIVerdict: verdicts[b.ID],
		})
	}
	return items, total, nil
}

// bookHintsFor computes the "possibly the same book" hints of books (with
// authors preloaded), index-aligned with books. It uses at most two
// queries (ISBN candidates, shared-author candidates), considers books of
// any status, and is shared by the queue and the AI review input so both
// see the same hints.
func (s *Service) bookHintsFor(ctx context.Context, books []catalogDomain.Book) ([][]BookHint, error) {
	out := make([][]BookHint, len(books))
	if len(books) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, 0, len(books))
	isbnSet := map[string]struct{}{}
	isbns := []string{}
	authorSet := map[uuid.UUID]struct{}{}
	authorIDs := []uuid.UUID{}
	pageRows := make([]repository.BookHintCandidate, 0, len(books))
	for _, b := range books {
		ids = append(ids, b.ID)
		if b.ISBN != nil {
			for _, v := range isbnVariants(normalizeISBN(*b.ISBN)) {
				if _, ok := isbnSet[v]; !ok {
					isbnSet[v] = struct{}{}
					isbns = append(isbns, v)
				}
			}
		}
		for _, a := range b.Authors {
			if _, ok := authorSet[a.ID]; !ok {
				authorSet[a.ID] = struct{}{}
				authorIDs = append(authorIDs, a.ID)
			}
		}
		pageRows = append(pageRows, bookHintCandidateOf(b))
	}

	byISBN, err := s.repo.FindBooksByNormalizedISBNs(ctx, isbns, ids)
	if err != nil {
		return nil, fmt.Errorf("find books by isbn: %w", err)
	}
	byAuthor, err := s.repo.FindBooksByAuthorIDs(ctx, authorIDs, ids)
	if err != nil {
		return nil, fmt.Errorf("find books by author: %w", err)
	}

	// Candidate pool: both query results plus the page's own rows, so two
	// duplicates on the same page hint at each other. Rows returned by
	// both queries are merged (author ids unioned).
	pool := mergeBookCandidates(byISBN, byAuthor, pageRows)
	for i := range books {
		out[i] = buildBookHints(pageRows[i], pool)
	}
	return out, nil
}

// bookHintCandidateOf converts a page book (with preloaded authors) into a
// hint candidate.
func bookHintCandidateOf(b catalogDomain.Book) repository.BookHintCandidate {
	authorIDs := make([]uuid.UUID, 0, len(b.Authors))
	for _, a := range b.Authors {
		authorIDs = append(authorIDs, a.ID)
	}
	return repository.BookHintCandidate{
		ID:             b.ID,
		Title:          b.Title,
		Slug:           b.Slug,
		Status:         b.Status,
		ISBN:           b.ISBN,
		PublishedYear:  b.PublishedYear,
		ExternalSource: b.ExternalSource,
		AuthorIDs:      authorIDs,
	}
}

// mergeBookCandidates concatenates candidate lists, keeping one entry per
// book id with the union of their author ids, in first-seen order.
func mergeBookCandidates(lists ...[]repository.BookHintCandidate) []repository.BookHintCandidate {
	out := []repository.BookHintCandidate{}
	index := map[uuid.UUID]int{}
	for _, list := range lists {
		for _, c := range list {
			i, ok := index[c.ID]
			if !ok {
				index[c.ID] = len(out)
				c.AuthorIDs = append([]uuid.UUID(nil), c.AuthorIDs...)
				out = append(out, c)
				continue
			}
			for _, id := range c.AuthorIDs {
				if !containsUUID(out[i].AuthorIDs, id) {
					out[i].AuthorIDs = append(out[i].AuthorIDs, id)
				}
			}
		}
	}
	return out
}

func containsUUID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// normalizeISBN strips hyphens and spaces and uppercases isbn. Only
// results of length 10 or 13 are returned; anything else yields "".
func normalizeISBN(isbn string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(isbn) {
		if r == '-' || r == ' ' {
			continue
		}
		b.WriteRune(r)
	}
	n := b.String()
	if len(n) != 10 && len(n) != 13 {
		return ""
	}
	return n
}

// isbnVariants returns the normalized ISBN together with its other form:
// an ISBN-10 gains its 978-prefixed ISBN-13, a 978 ISBN-13 gains its
// ISBN-10 (check digits recomputed). 979 ISBN-13s and values without a
// digit body have no other form. Empty input returns nil.
func isbnVariants(norm string) []string {
	if norm == "" {
		return nil
	}
	out := []string{norm}
	switch len(norm) {
	case 10:
		if body := norm[:9]; allDigits(body) {
			out = append(out, "978"+body+isbn13CheckDigit("978"+body))
		}
	case 13:
		if strings.HasPrefix(norm, "978") {
			if body := norm[3:12]; allDigits(body) {
				out = append(out, body+isbn10CheckDigit(body))
			}
		}
	}
	return out
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// isbn13CheckDigit computes the ISBN-13 check digit of 12 digits.
func isbn13CheckDigit(digits12 string) string {
	sum := 0
	for i := 0; i < 12; i++ {
		d := int(digits12[i] - '0')
		if i%2 == 1 {
			d *= 3
		}
		sum += d
	}
	return string(rune('0' + (10-sum%10)%10))
}

// isbn10CheckDigit computes the ISBN-10 check digit (0-9 or X) of 9 digits.
func isbn10CheckDigit(digits9 string) string {
	sum := 0
	for i := 0; i < 9; i++ {
		sum += int(digits9[i]-'0') * (10 - i)
	}
	check := (11 - sum%11) % 11
	if check == 10 {
		return "X"
	}
	return string(rune('0' + check))
}

// normalizeBookTitle normalizes a title like author/genre names: lowercase,
// ё folded to е, non-letter/non-digit runes turned into spaces, spaces
// collapsed.
func normalizeBookTitle(title string) string {
	return normalizeGenreName(title)
}

// bookMatchKind classifies how candidate matches book, or "" when it does
// not: same_isbn when their normalized ISBNs are equal (an ISBN-10 also
// matching its 978 ISBN-13), else same_title_author when the normalized
// titles are equal and they share an author.
func bookMatchKind(book, candidate repository.BookHintCandidate) string {
	if book.ISBN != nil && candidate.ISBN != nil {
		if other := normalizeISBN(*candidate.ISBN); other != "" {
			for _, v := range isbnVariants(normalizeISBN(*book.ISBN)) {
				if v == other {
					return BookMatchSameISBN
				}
			}
		}
	}
	title := normalizeBookTitle(book.Title)
	if title == "" || title != normalizeBookTitle(candidate.Title) {
		return ""
	}
	for _, id := range book.AuthorIDs {
		if containsUUID(candidate.AuthorIDs, id) {
			return BookMatchSameTitleAuthor
		}
	}
	return ""
}

func bookMatchRank(kind string) int {
	if kind == BookMatchSameISBN {
		return 0
	}
	return 1
}

func bookStatusRank(status string) int {
	switch status {
	case catalogDomain.BookStatusPublished:
		return 0
	case catalogDomain.BookStatusPending:
		return 1
	default:
		return 2
	}
}

// buildBookHints returns at most maxBookHints candidates matching book
// (self excluded, any status), each reported once with its strongest
// match, ordered by match kind, then published < pending < rejected, then
// title, then id. Never nil.
func buildBookHints(book repository.BookHintCandidate, candidates []repository.BookHintCandidate) []BookHint {
	hints := []BookHint{}
	index := map[uuid.UUID]int{}
	for _, c := range candidates {
		if c.ID == book.ID {
			continue
		}
		kind := bookMatchKind(book, c)
		if kind == "" {
			continue
		}
		if i, dup := index[c.ID]; dup {
			if bookMatchRank(kind) < bookMatchRank(hints[i].Match) {
				hints[i].Match = kind
			}
			continue
		}
		index[c.ID] = len(hints)
		hints = append(hints, BookHint{Book: c, Match: kind})
	}
	sort.SliceStable(hints, func(i, j int) bool {
		a, b := hints[i], hints[j]
		if ra, rb := bookMatchRank(a.Match), bookMatchRank(b.Match); ra != rb {
			return ra < rb
		}
		if sa, sb := bookStatusRank(a.Book.Status), bookStatusRank(b.Book.Status); sa != sb {
			return sa < sb
		}
		if a.Book.Title != b.Book.Title {
			return a.Book.Title < b.Book.Title
		}
		return a.Book.ID.String() < b.Book.ID.String()
	})
	if len(hints) > maxBookHints {
		hints = hints[:maxBookHints]
	}
	return hints
}
