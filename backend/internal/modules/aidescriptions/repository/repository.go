// Package repository is the GORM storage of AI book descriptions (table
// book_ai_descriptions, migration 000024). The table is not catalog data,
// so the module owns its repository instead of growing admin/repository.
// It also runs read-only SQL against books (candidate selection, counts,
// the superseded flag); it never writes any book field. Every timestamp is
// passed in by the caller (UTC).
//
// The "blank own description" condition (blankDescription) uses only
// IS NULL and trim, so it is valid in Postgres and SQLite alike.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/modules/aidescriptions/domain"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Errors of Approve and Discard.
var (
	// ErrNotFound: the book has no AI description.
	ErrNotFound = errors.New("AI description not found")
	// ErrNotReviewable: the record is not a draft or approved.
	ErrNotReviewable = errors.New("AI description is not reviewable")
	// ErrHasOwnDescription: the book has its own description (approve).
	ErrHasOwnDescription = errors.New("book already has a description")
)

// blankDescription is the "own description blank" condition on alias b.
const blankDescription = "(b.description IS NULL OR trim(b.description) = '')"

// descriptionRow is one row of book_ai_descriptions.
type descriptionRow struct {
	BookID        uuid.UUID  `gorm:"column:book_id;primaryKey"`
	Status        string     `gorm:"column:status"`
	Text          string     `gorm:"column:text"`
	ErrorCode     *string    `gorm:"column:error_code"`
	Edited        bool       `gorm:"column:edited"`
	Model         string     `gorm:"column:model"`
	PromptVersion int        `gorm:"column:prompt_version"`
	InputHash     string     `gorm:"column:input_hash"`
	InputTokens   int        `gorm:"column:input_tokens"`
	OutputTokens  int        `gorm:"column:output_tokens"`
	GeneratedAt   time.Time  `gorm:"column:generated_at"`
	ReviewedBy    *uuid.UUID `gorm:"column:reviewed_by"`
	ReviewedAt    *time.Time `gorm:"column:reviewed_at"`
}

func (descriptionRow) TableName() string { return "book_ai_descriptions" }

// itemRow is a description row plus its book card fields (flat: GORM's
// Raw().Scan does not fill embedded structs).
type itemRow struct {
	BookID            uuid.UUID  `gorm:"column:book_id"`
	Status            string     `gorm:"column:status"`
	Text              string     `gorm:"column:text"`
	ErrorCode         *string    `gorm:"column:error_code"`
	Edited            bool       `gorm:"column:edited"`
	Model             string     `gorm:"column:model"`
	PromptVersion     int        `gorm:"column:prompt_version"`
	InputHash         string     `gorm:"column:input_hash"`
	InputTokens       int        `gorm:"column:input_tokens"`
	OutputTokens      int        `gorm:"column:output_tokens"`
	GeneratedAt       time.Time  `gorm:"column:generated_at"`
	ReviewedBy        *uuid.UUID `gorm:"column:reviewed_by"`
	ReviewedAt        *time.Time `gorm:"column:reviewed_at"`
	BookTitle         string     `gorm:"column:book_title"`
	BookPublishedYear *int       `gorm:"column:book_published_year"`
	BookStatus        string     `gorm:"column:book_status"`
	Superseded        int        `gorm:"column:superseded"`
}

// itemSelect selects a description with its book card and the superseded
// flag (1 when the book has its own description).
const itemSelect = `
SELECT d.*, b.title AS book_title, b.published_year AS book_published_year, b.status AS book_status,
       CASE WHEN ` + blankDescription + ` THEN 0 ELSE 1 END AS superseded
FROM book_ai_descriptions d
JOIN books b ON b.id = d.book_id`

// GormRepository implements the aidescriptions storage on GORM.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository builds a GormRepository.
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// Upsert stores d as the book's only AI description, replacing any
// previous one whatever its status. The review columns (reviewed_by,
// reviewed_at, edited) are reset.
func (r *GormRepository) Upsert(ctx context.Context, d domain.Description) error {
	row := descriptionRow{
		BookID:        d.BookID,
		Status:        d.Status,
		Text:          d.Text,
		ErrorCode:     d.ErrorCode,
		Edited:        false,
		Model:         d.Model,
		PromptVersion: d.PromptVersion,
		InputHash:     d.InputHash,
		InputTokens:   d.InputTokens,
		OutputTokens:  d.OutputTokens,
		GeneratedAt:   d.GeneratedAt,
	}
	err := r.db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "book_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"status", "text", "error_code", "edited", "model", "prompt_version", "input_hash",
			"input_tokens", "output_tokens", "generated_at", "reviewed_by", "reviewed_at",
		}),
	}).Create(&row).Error
	if err != nil {
		return fmt.Errorf("upsert AI description: %w", err)
	}
	return nil
}

// Get returns the book's AI description with its book card (authors not
// filled), or nil when there is none.
func (r *GormRepository) Get(ctx context.Context, bookID uuid.UUID) (*domain.Item, error) {
	var rows []itemRow
	if err := r.db.WithContext(ctx).Raw(itemSelect+" WHERE d.book_id = ?", bookID).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get AI description: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	item := toItem(rows[0])
	return &item, nil
}

// List returns one page of AI descriptions in status, newest first, with
// their book cards (authors not filled), and the total in that status.
func (r *GormRepository) List(ctx context.Context, status string, page, limit int) ([]domain.Item, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Model(&descriptionRow{}).Where("status = ?", status).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count AI descriptions: %w", err)
	}
	var rows []itemRow
	err := r.db.WithContext(ctx).
		Raw(itemSelect+" WHERE d.status = ? ORDER BY d.generated_at DESC, d.book_id ASC LIMIT ? OFFSET ?",
			status, limit, (page-1)*limit).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("list AI descriptions: %w", err)
	}
	out := make([]domain.Item, 0, len(rows))
	for _, row := range rows {
		out = append(out, toItem(row))
	}
	return out, total, nil
}

// CountDrafts counts drafts whose book still has no own description.
func (r *GormRepository) CountDrafts(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("book_ai_descriptions AS d").
		Joins("JOIN books b ON b.id = d.book_id").
		Where("d.status = ? AND "+blankDescription, domain.StatusDraft).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count AI description drafts: %w", err)
	}
	return n, nil
}

// missingWhere selects published books with a blank own description and no
// AI description record at all.
const missingWhere = "b.status = ? AND " + blankDescription +
	" AND NOT EXISTS (SELECT 1 FROM book_ai_descriptions d WHERE d.book_id = b.id)"

// CountMissing counts the books an all_missing run would pick (no cap).
func (r *GormRepository) CountMissing(ctx context.Context) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("books AS b").
		Where(missingWhere, catalogDomain.BookStatusPublished).
		Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count books missing a description: %w", err)
	}
	return n, nil
}

// ListMissingCandidateIDs returns up to limit books an all_missing run
// picks, oldest first.
func (r *GormRepository) ListMissingCandidateIDs(ctx context.Context, limit int) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	err := r.db.WithContext(ctx).Table("books AS b").
		Where(missingWhere, catalogDomain.BookStatusPublished).
		Order("b.created_at ASC, b.id ASC").
		Limit(limit).
		Pluck("b.id", &ids).Error
	if err != nil {
		return nil, fmt.Errorf("list books missing a description: %w", err)
	}
	return ids, nil
}

// Approve sets a draft or approved record to approved by userID, in one
// transaction that first re-checks that the book's own description is
// blank. A non-nil text replaces the stored one; edited becomes true when
// it differs (an already edited record stays edited). Errors:
// ErrNotFound, ErrNotReviewable, ErrHasOwnDescription.
func (r *GormRepository) Approve(ctx context.Context, bookID, userID uuid.UUID, text *string, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []descriptionRow
		if err := tx.Where("book_id = ?", bookID).Limit(1).Find(&rows).Error; err != nil {
			return fmt.Errorf("load AI description: %w", err)
		}
		if len(rows) == 0 {
			return ErrNotFound
		}
		cur := rows[0]
		if !domain.IsReviewable(cur.Status) {
			return ErrNotReviewable
		}
		var blank int64
		if err := tx.Table("books AS b").Where("b.id = ? AND "+blankDescription, bookID).Count(&blank).Error; err != nil {
			return fmt.Errorf("check own description: %w", err)
		}
		if blank == 0 {
			return ErrHasOwnDescription
		}
		updates := map[string]any{
			"status":      domain.StatusApproved,
			"reviewed_by": userID,
			"reviewed_at": now,
		}
		if text != nil {
			updates["text"] = *text
			updates["edited"] = cur.Edited || *text != cur.Text
		}
		res := tx.Model(&descriptionRow{}).
			Where("book_id = ? AND status IN ?", bookID, []string{domain.StatusDraft, domain.StatusApproved}).
			Updates(updates)
		if res.Error != nil {
			return fmt.Errorf("approve AI description: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			return ErrNotReviewable
		}
		return nil
	})
}

// Discard sets a draft or approved record to discarded by userID. Errors:
// ErrNotFound, ErrNotReviewable.
func (r *GormRepository) Discard(ctx context.Context, bookID, userID uuid.UUID, now time.Time) error {
	res := r.db.WithContext(ctx).Model(&descriptionRow{}).
		Where("book_id = ? AND status IN ?", bookID, []string{domain.StatusDraft, domain.StatusApproved}).
		Updates(map[string]any{"status": domain.StatusDiscarded, "reviewed_by": userID, "reviewed_at": now})
	if res.Error != nil {
		return fmt.Errorf("discard AI description: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		return nil
	}
	var n int64
	if err := r.db.WithContext(ctx).Model(&descriptionRow{}).Where("book_id = ?", bookID).Count(&n).Error; err != nil {
		return fmt.Errorf("load AI description: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return ErrNotReviewable
}

// ApprovedAIDescription returns the book's approved AI text and its
// generation time; ok is false when the book has no approved record. The
// caller (the public book detail) consults it only when the book's own
// description is blank.
func (r *GormRepository) ApprovedAIDescription(ctx context.Context, bookID uuid.UUID) (string, time.Time, bool, error) {
	var rows []descriptionRow
	err := r.db.WithContext(ctx).
		Where("book_id = ? AND status = ?", bookID, domain.StatusApproved).
		Limit(1).Find(&rows).Error
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("get approved AI description: %w", err)
	}
	if len(rows) == 0 {
		return "", time.Time{}, false, nil
	}
	return rows[0].Text, rows[0].GeneratedAt.UTC(), true, nil
}

func toItem(row itemRow) domain.Item {
	var reviewedAt *time.Time
	if row.ReviewedAt != nil {
		t := row.ReviewedAt.UTC()
		reviewedAt = &t
	}
	return domain.Item{
		Description: domain.Description{
			BookID:        row.BookID,
			Status:        row.Status,
			Text:          row.Text,
			ErrorCode:     row.ErrorCode,
			Edited:        row.Edited,
			Model:         row.Model,
			PromptVersion: row.PromptVersion,
			InputHash:     row.InputHash,
			InputTokens:   row.InputTokens,
			OutputTokens:  row.OutputTokens,
			GeneratedAt:   row.GeneratedAt.UTC(),
			ReviewedBy:    row.ReviewedBy,
			ReviewedAt:    reviewedAt,
		},
		Book: domain.BookCard{
			ID:            row.BookID,
			Title:         row.BookTitle,
			Authors:       []string{},
			PublishedYear: row.BookPublishedYear,
			Status:        row.BookStatus,
		},
		Superseded: row.Superseded != 0,
	}
}
