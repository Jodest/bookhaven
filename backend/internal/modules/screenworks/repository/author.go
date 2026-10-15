package repository

import (
	"context"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Author film links (openspec change add-author-screen-works).

type authorRow struct {
	ID             uuid.UUID `gorm:"column:id"`
	Name           string    `gorm:"column:name"`
	Status         string    `gorm:"column:status"`
	ExternalSource *string   `gorm:"column:external_source"`
	ExternalID     *string   `gorm:"column:external_id"`
}

func (r *GormRepository) GetAuthorsForSync(ctx context.Context, ids []uuid.UUID) ([]domain.AuthorRef, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var rows []authorRow
	err := r.db.WithContext(ctx).Table("authors").
		Select("id, name, status, external_source, external_id").
		Where("id IN ?", ids).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]domain.AuthorRef, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.AuthorRef{
			ID: row.ID, Name: row.Name, Status: row.Status,
			ExternalSource: row.ExternalSource, ExternalID: row.ExternalID,
		})
	}
	return out, nil
}

func (r *GormRepository) AuthorStatus(ctx context.Context, id uuid.UUID) (string, error) {
	var rows []authorRow
	err := r.db.WithContext(ctx).Table("authors").Select("id, name, status").
		Where("id = ?", id).Limit(1).Scan(&rows).Error
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", domain.ErrAuthorNotFound
	}
	return rows[0].Status, nil
}

// SyncAuthor upserts the works and inserts the links; an existing link (hidden
// or not) is never touched, so a hide survives and relation is set on insert
// only.
func (r *GormRepository) SyncAuthor(ctx context.Context, authorID uuid.UUID, films []domain.AuthorFilmInput) (int, error) {
	added := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, f := range films {
			workID, err := upsertWikidataWork(tx, f.WorkInput)
			if err != nil {
				return err
			}
			now := time.Now()
			res := tx.Exec(`INSERT INTO author_screen_works (author_id, screen_work_id, relation, hidden, source, created_at, updated_at)
VALUES (?, ?, ?, FALSE, ?, ?, ?) ON CONFLICT (author_id, screen_work_id) DO NOTHING`,
				authorID, workID, f.Relation, domain.SourceWikidata, now, now)
			if res.Error != nil {
				return res.Error
			}
			added += int(res.RowsAffected)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return added, nil
}

const authorLinkedSelect = linkedSelect + ", l.relation"

func (r *GormRepository) listAuthor(ctx context.Context, authorID uuid.UUID, onlyVisible bool) ([]domain.LinkedWork, error) {
	q := r.db.WithContext(ctx).Table("author_screen_works AS l").
		Joins("JOIN screen_works w ON w.id = l.screen_work_id").
		Select(authorLinkedSelect).Where("l.author_id = ?", authorID)
	if onlyVisible {
		q = q.Where("l.hidden = ?", false)
	}
	var out []domain.LinkedWork
	if err := q.Order(linkedOrder).Scan(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}

func (r *GormRepository) ListPublicByAuthor(ctx context.Context, authorID uuid.UUID) ([]domain.LinkedWork, error) {
	return r.listAuthor(ctx, authorID, true)
}

func (r *GormRepository) ListAdminByAuthor(ctx context.Context, authorID uuid.UUID) ([]domain.LinkedWork, error) {
	return r.listAuthor(ctx, authorID, false)
}

func (r *GormRepository) SetAuthorLinkHidden(ctx context.Context, authorID, workID uuid.UUID, hidden bool) error {
	res := r.db.WithContext(ctx).Exec(
		"UPDATE author_screen_works SET hidden = ?, updated_at = ? WHERE author_id = ? AND screen_work_id = ?",
		hidden, time.Now(), authorID, workID)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return domain.ErrAuthorLinkNotFound
	}
	return nil
}
