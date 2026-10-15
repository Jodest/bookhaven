// Package repository is the GORM storage of AI collection drafts and their
// proposals (tables ai_collection_drafts / ai_collection_proposals,
// migration 000028). These tables are not catalog data, so the module owns
// its repository. Every timestamp is passed in by the caller (UTC), so the
// service clock drives staleness and retention.
package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicollections/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound: no draft with that id.
var ErrNotFound = errors.New("aicollections: draft not found")

type draftRow struct {
	ID            uuid.UUID  `gorm:"column:id;primaryKey"`
	CollectionID  uuid.UUID  `gorm:"column:collection_id"`
	Brief         string     `gorm:"column:brief"`
	Count         int        `gorm:"column:count"`
	Status        string     `gorm:"column:status"`
	ErrorCode     *string    `gorm:"column:error_code"`
	Model         string     `gorm:"column:model"`
	PromptVersion int        `gorm:"column:prompt_version"`
	InputTokens   int        `gorm:"column:input_tokens"`
	OutputTokens  int        `gorm:"column:output_tokens"`
	CreatedBy     *uuid.UUID `gorm:"column:created_by"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime:false"`
	UpdatedAt     time.Time  `gorm:"column:updated_at;autoUpdateTime:false"`
	FinishedAt    *time.Time `gorm:"column:finished_at"`
}

func (draftRow) TableName() string { return "ai_collection_drafts" }

type proposalRow struct {
	DraftID       uuid.UUID `gorm:"column:draft_id;primaryKey"`
	Rank          int       `gorm:"column:rank;primaryKey;autoIncrement:false"`
	Title         string    `gorm:"column:title"`
	Author        string    `gorm:"column:author"`
	OriginalTitle string    `gorm:"column:original_title"`
	Note          string    `gorm:"column:note"`
}

func (proposalRow) TableName() string { return "ai_collection_proposals" }

// summaryRow is a draft row plus its creator and proposal count (flat:
// GORM Raw().Scan does not fill embedded structs).
type summaryRow struct {
	ID             uuid.UUID  `gorm:"column:id"`
	CollectionID   uuid.UUID  `gorm:"column:collection_id"`
	Brief          string     `gorm:"column:brief"`
	Count          int        `gorm:"column:count"`
	Status         string     `gorm:"column:status"`
	ErrorCode      *string    `gorm:"column:error_code"`
	Model          string     `gorm:"column:model"`
	PromptVersion  int        `gorm:"column:prompt_version"`
	InputTokens    int        `gorm:"column:input_tokens"`
	OutputTokens   int        `gorm:"column:output_tokens"`
	CreatedBy      *uuid.UUID `gorm:"column:created_by"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
	FinishedAt     *time.Time `gorm:"column:finished_at"`
	CreatorName    *string    `gorm:"column:creator_name"`
	CreatorEmail   *string    `gorm:"column:creator_email"`
	ProposalsCount int        `gorm:"column:proposals_count"`
}

// GormRepository implements the aicollections storage on GORM.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository builds a GormRepository.
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// summarySelect selects a draft with its creator (LEFT JOIN users) and its
// proposal count (one grouped subquery).
const summarySelect = `
SELECT d.*, u.name AS creator_name, u.email AS creator_email,
       COALESCE(p.total, 0) AS proposals_count
FROM ai_collection_drafts d
LEFT JOIN users u ON u.id = d.created_by
LEFT JOIN (
    SELECT draft_id, COUNT(*) AS total
    FROM ai_collection_proposals
    GROUP BY draft_id
) p ON p.draft_id = d.id`

// CreateDraft inserts d. A zero ID is replaced by a new UUID; d is updated
// with the stored id.
func (r *GormRepository) CreateDraft(ctx context.Context, d *domain.Draft) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	row := draftRow{
		ID: d.ID, CollectionID: d.CollectionID, Brief: d.Brief, Count: d.Count, Status: d.Status,
		ErrorCode: d.ErrorCode, Model: d.Model, PromptVersion: d.PromptVersion, CreatedBy: d.CreatedBy,
		CreatedAt: d.CreatedAt, UpdatedAt: d.UpdatedAt, FinishedAt: d.FinishedAt,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("create collection draft: %w", err)
	}
	return nil
}

// GetDraft returns the draft with its creator and proposal count;
// ErrNotFound when absent.
func (r *GormRepository) GetDraft(ctx context.Context, id uuid.UUID) (*domain.Draft, error) {
	var rows []summaryRow
	if err := r.db.WithContext(ctx).Raw(summarySelect+" WHERE d.id = ?", id).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get collection draft: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	d := toDraft(rows[0])
	return &d, nil
}

// ListDraftsByCollection returns the collection newest limit drafts.
func (r *GormRepository) ListDraftsByCollection(ctx context.Context, collectionID uuid.UUID, limit int) ([]domain.Draft, error) {
	var rows []summaryRow
	err := r.db.WithContext(ctx).
		Raw(summarySelect+" WHERE d.collection_id = ? ORDER BY d.created_at DESC LIMIT ?", collectionID, limit).
		Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list collection drafts: %w", err)
	}
	out := make([]domain.Draft, 0, len(rows))
	for _, row := range rows {
		out = append(out, toDraft(row))
	}
	return out, nil
}

// FindActiveDraft returns the collection newest active draft (queued or
// running with updated_at > staleBefore), or nil.
func (r *GormRepository) FindActiveDraft(ctx context.Context, collectionID uuid.UUID, staleBefore time.Time) (*domain.Draft, error) {
	var rows []draftRow
	err := r.db.WithContext(ctx).
		Where("collection_id = ? AND status IN ? AND updated_at > ?", collectionID,
			[]string{domain.StatusQueued, domain.StatusRunning}, staleBefore).
		Order("created_at DESC").Limit(1).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("find active collection draft: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	d := toDraft(summaryRow{
		ID: row.ID, CollectionID: row.CollectionID, Brief: row.Brief, Count: row.Count, Status: row.Status,
		ErrorCode: row.ErrorCode, Model: row.Model, PromptVersion: row.PromptVersion,
		InputTokens: row.InputTokens, OutputTokens: row.OutputTokens, CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, FinishedAt: row.FinishedAt,
	})
	return &d, nil
}

// CountDraftsSince counts the user drafts created after since.
func (r *GormRepository) CountDraftsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&draftRow{}).
		Where("created_by = ? AND created_at > ?", userID, since).Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count collection drafts: %w", err)
	}
	return n, nil
}

// MarkRunning sets a non-terminal draft to running.
func (r *GormRepository) MarkRunning(ctx context.Context, id uuid.UUID, now time.Time) error {
	err := r.db.WithContext(ctx).Model(&draftRow{}).
		Where("id = ? AND status IN ?", id, []string{domain.StatusQueued, domain.StatusRunning}).
		Updates(map[string]any{"status": domain.StatusRunning, "updated_at": now}).Error
	if err != nil {
		return fmt.Errorf("mark collection draft running: %w", err)
	}
	return nil
}

// MarkFailed ends a draft as failed with code.
func (r *GormRepository) MarkFailed(ctx context.Context, id uuid.UUID, code string, now time.Time) error {
	err := r.db.WithContext(ctx).Model(&draftRow{}).Where("id = ?", id).
		Updates(map[string]any{"status": domain.StatusFailed, "error_code": code, "updated_at": now, "finished_at": now}).Error
	if err != nil {
		return fmt.Errorf("mark collection draft failed: %w", err)
	}
	return nil
}

// MarkDone ends a draft as done; errorCode (may be nil) names a later part
// that failed.
func (r *GormRepository) MarkDone(ctx context.Context, id uuid.UUID, errorCode *string, now time.Time) error {
	err := r.db.WithContext(ctx).Model(&draftRow{}).Where("id = ?", id).
		Updates(map[string]any{"status": domain.StatusDone, "error_code": errorCode, "updated_at": now, "finished_at": now}).Error
	if err != nil {
		return fmt.Errorf("mark collection draft done: %w", err)
	}
	return nil
}

// ListProposals returns the draft proposals by rank.
func (r *GormRepository) ListProposals(ctx context.Context, draftID uuid.UUID) ([]domain.Proposal, error) {
	var rows []proposalRow
	if err := r.db.WithContext(ctx).Where("draft_id = ?", draftID).Order("rank ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list collection proposals: %w", err)
	}
	out := make([]domain.Proposal, 0, len(rows))
	for _, row := range rows {
		out = append(out, domain.Proposal{
			DraftID: row.DraftID, Rank: row.Rank, Title: row.Title, Author: row.Author,
			OriginalTitle: row.OriginalTitle, Note: row.Note,
		})
	}
	return out, nil
}

// AppendProposals stores ps with ranks startRank.., adds the tokens, sets the
// model and bumps updated_at (the progress heartbeat), in one transaction.
func (r *GormRepository) AppendProposals(ctx context.Context, draftID uuid.UUID, startRank int, ps []domain.NewProposal, model string, inputTokens, outputTokens int, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&draftRow{}).Where("id = ?", draftID).Updates(map[string]any{
			"model":         model,
			"input_tokens":  gorm.Expr("input_tokens + ?", inputTokens),
			"output_tokens": gorm.Expr("output_tokens + ?", outputTokens),
			"updated_at":    now,
		}).Error
		if err != nil {
			return fmt.Errorf("update collection draft: %w", err)
		}
		if len(ps) == 0 {
			return nil
		}
		rows := make([]proposalRow, 0, len(ps))
		for i, p := range ps {
			rows = append(rows, proposalRow{
				DraftID: draftID, Rank: startRank + i, Title: p.Title, Author: p.Author,
				OriginalTitle: p.OriginalTitle, Note: p.Note,
			})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("save collection proposals: %w", err)
		}
		return nil
	})
}

// PurgeOlderThan deletes drafts created before t (proposals cascade in
// Postgres; they are deleted explicitly too, so the purge does not depend on
// the FK action) and returns the number of deleted drafts.
func (r *GormRepository) PurgeOlderThan(ctx context.Context, t time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		old := tx.Model(&draftRow{}).Select("id").Where("created_at < ?", t)
		if err := tx.Where("draft_id IN (?)", old).Delete(&proposalRow{}).Error; err != nil {
			return fmt.Errorf("purge collection proposals: %w", err)
		}
		res := tx.Where("created_at < ?", t).Delete(&draftRow{})
		if res.Error != nil {
			return fmt.Errorf("purge collection drafts: %w", res.Error)
		}
		n = res.RowsAffected
		return nil
	})
	return n, err
}

func toDraft(row summaryRow) domain.Draft {
	d := domain.Draft{
		ID: row.ID, CollectionID: row.CollectionID, Brief: row.Brief, Count: row.Count, Status: row.Status,
		ErrorCode: row.ErrorCode, Model: row.Model, PromptVersion: row.PromptVersion,
		InputTokens: row.InputTokens, OutputTokens: row.OutputTokens, CreatedBy: row.CreatedBy,
		CreatedAt: row.CreatedAt.UTC(), UpdatedAt: row.UpdatedAt.UTC(), FinishedAt: row.FinishedAt,
		ProposalsCount: row.ProposalsCount,
	}
	if row.CreatedBy != nil && (row.CreatorName != nil || row.CreatorEmail != nil) {
		username := ""
		if row.CreatorName != nil {
			username = strings.TrimSpace(*row.CreatorName)
		}
		if username == "" && row.CreatorEmail != nil {
			username = *row.CreatorEmail
		}
		d.Creator = &domain.Creator{ID: *row.CreatedBy, Username: username}
	}
	return d
}
