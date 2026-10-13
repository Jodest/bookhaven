package repository

import (
	"context"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func (r *GormRepository) ListAutoHarvestCandidates(ctx context.Context, limit int, notAfter time.Time) ([]catalogDomain.Author, error) {
	var authors []catalogDomain.Author
	if limit <= 0 {
		return authors, nil
	}
	err := r.db.WithContext(ctx).
		Where("auto_harvest = ?", true).
		Where("status = ?", catalogDomain.AuthorStatusPublished).
		Where("external_source = ?", catalogDomain.AuthorExternalSourceWikidata).
		Where("external_id IS NOT NULL AND external_id <> ''").
		Where("last_auto_harvested_at IS NULL OR last_auto_harvested_at <= ?", notAfter).
		Order("last_auto_harvested_at ASC NULLS FIRST").
		Order("name ASC").
		Order("id ASC").
		Limit(limit).
		Find(&authors).Error
	return authors, err
}

func (r *GormRepository) MarkAuthorsAutoHarvested(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).Model(&catalogDomain.Author{}).
		Where("id IN ?", ids).
		UpdateColumn("last_auto_harvested_at", at).Error
}
