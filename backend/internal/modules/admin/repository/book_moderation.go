package repository

import (
	"context"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Book moderation queue (docs/features/book-moderation-queue.md).

// bookHintAuthorRowCap bounds the rows FindBooksByAuthorIDs reads for one
// page (prolific authors); some same_title_author hints may be dropped
// beyond it.
const bookHintAuthorRowCap = 2000

// BookModerationFilter defines filtering options for the book moderation
// queue. Status is a valid book status or "" (any); Source is "" (any),
// "manual" or "harvested" (same semantics as BookFilter.Source). Search
// uses the same full-text condition as ListBooks (Postgres-only). AI is ""
// (any), "none" (no AI verdict) or a verdict value (publish, reject,
// needs_human; stale or not).
type BookModerationFilter struct {
	Status string
	Source string
	AI     string
	Search string
	Limit  int
	Offset int
}

// BookHintCandidate is a compact book used as a "possibly the same book"
// hint candidate. AuthorIDs is only filled by FindBooksByAuthorIDs (and by
// the service for page rows); FindBooksByNormalizedISBNs leaves it nil.
type BookHintCandidate struct {
	ID             uuid.UUID
	Title          string
	Slug           string
	Status         string
	ISBN           *string
	PublishedYear  *int
	ExternalSource *string
	AuthorIDs      []uuid.UUID
}

// bookHintScanRow is the flat scan target of the hint candidate queries.
type bookHintScanRow struct {
	ID             uuid.UUID
	Title          string
	Slug           string
	Status         string
	ISBN           *string `gorm:"column:isbn"`
	PublishedYear  *int
	ExternalSource *string
	AuthorID       *uuid.UUID
}

func (row bookHintScanRow) candidate() BookHintCandidate {
	return BookHintCandidate{
		ID:             row.ID,
		Title:          row.Title,
		Slug:           row.Slug,
		Status:         row.Status,
		ISBN:           row.ISBN,
		PublishedYear:  row.PublishedYear,
		ExternalSource: row.ExternalSource,
	}
}

func (r *GormRepository) ListBooksForModeration(ctx context.Context, filter BookModerationFilter) ([]catalogDomain.Book, int64, error) {
	query := r.db.WithContext(ctx).Model(&catalogDomain.Book{}).
		Preload("Authors").
		Preload("Genres")

	if filter.Status != "" {
		query = query.Where("books.status = ?", filter.Status)
	}
	switch filter.Source {
	case "manual":
		query = query.Where("books.external_source IS NULL")
	case "harvested":
		query = query.Where("books.external_source IS NOT NULL")
	}
	switch filter.AI {
	case "":
	case "none":
		// 1:1 join (book_id is the verdicts PK), so Count stays exact.
		query = query.Joins("LEFT JOIN book_ai_verdicts v ON v.book_id = books.id").Where("v.book_id IS NULL")
	default:
		query = query.Joins("LEFT JOIN book_ai_verdicts v ON v.book_id = books.id").Where("v.verdict = ?", filter.AI)
	}
	if filter.Search != "" {
		query = query.Where(
			"to_tsvector('russian', title || ' ' || COALESCE(isbn, '') || ' ' || COALESCE(description, '')) @@ plainto_tsquery('russian', ?)",
			filter.Search,
		)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var books []catalogDomain.Book
	if err := query.
		Order("books.created_at DESC, books.title ASC, books.id ASC").
		Limit(limit).
		Offset(offset).
		Find(&books).Error; err != nil {
		return nil, 0, err
	}
	return books, total, nil
}

func (r *GormRepository) FindBooksByNormalizedISBNs(ctx context.Context, isbns []string, excludeIDs []uuid.UUID) ([]BookHintCandidate, error) {
	out := []BookHintCandidate{}
	if len(isbns) == 0 {
		return out, nil
	}

	query := r.db.WithContext(ctx).Table("books").
		Select("id, title, slug, status, isbn, published_year, external_source").
		Where("upper(replace(replace(isbn, '-', ''), ' ', '')) IN ?", isbns)
	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}

	var rows []bookHintScanRow
	if err := query.Order("id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out = append(out, row.candidate())
	}
	return out, nil
}

func (r *GormRepository) FindBooksByAuthorIDs(ctx context.Context, authorIDs, excludeIDs []uuid.UUID) ([]BookHintCandidate, error) {
	out := []BookHintCandidate{}
	if len(authorIDs) == 0 {
		return out, nil
	}

	query := r.db.WithContext(ctx).Table("books AS b").
		Select("b.id, b.title, b.slug, b.status, b.isbn, b.published_year, b.external_source, ba.author_id").
		Joins("JOIN book_authors ba ON ba.book_id = b.id").
		Where("ba.author_id IN ?", authorIDs)
	if len(excludeIDs) > 0 {
		query = query.Where("b.id NOT IN ?", excludeIDs)
	}

	var rows []bookHintScanRow
	if err := query.Order("b.id, ba.author_id").Limit(bookHintAuthorRowCap).Scan(&rows).Error; err != nil {
		return nil, err
	}

	index := make(map[uuid.UUID]int, len(rows))
	for _, row := range rows {
		i, ok := index[row.ID]
		if !ok {
			i = len(out)
			index[row.ID] = i
			out = append(out, row.candidate())
		}
		if row.AuthorID != nil {
			out[i].AuthorIDs = append(out[i].AuthorIDs, *row.AuthorID)
		}
	}
	return out, nil
}
