package repository

import (
	"context"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
)

// ListWikidataBooksWithoutCover returns Wikidata books that are not rejected
// and have a NULL or empty cover link, oldest first, with authors and genres.
func (r *GormRepository) ListWikidataBooksWithoutCover(ctx context.Context) ([]catalogDomain.Book, error) {
	var books []catalogDomain.Book
	if err := r.db.WithContext(ctx).
		Preload("Authors").
		Preload("Genres").
		Where("external_source = ? AND (cover_url IS NULL OR cover_url = '') AND status <> ?",
			"wikidata", catalogDomain.BookStatusRejected).
		Order("created_at, id").
		Find(&books).Error; err != nil {
		return nil, err
	}
	return books, nil
}
