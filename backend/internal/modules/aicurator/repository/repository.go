// Package repository is the GORM storage of AI curation requests and their
// candidates (tables ai_curation_requests / ai_curation_candidates,
// migration 000023). These tables are not catalog data, so the module owns
// its repository instead of growing admin/repository. Every timestamp is
// passed in by the caller (UTC), so the service's clock drives staleness
// and retention.
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/bookhaven/backend/internal/modules/aicurator/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrNotFound: no request with that id.
var ErrNotFound = errors.New("aicurator: request not found")

// requestRow is one row of ai_curation_requests.
type requestRow struct {
	ID            uuid.UUID  `gorm:"column:id;primaryKey"`
	Prompt        string     `gorm:"column:prompt"`
	MaxNames      int        `gorm:"column:max_names"`
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

func (requestRow) TableName() string { return "ai_curation_requests" }

// candidateRow is one row of ai_curation_candidates. Options holds a JSON
// array of domain.Option.
type candidateRow struct {
	ID          uuid.UUID `gorm:"column:id;primaryKey"`
	RequestID   uuid.UUID `gorm:"column:request_id"`
	Position    int       `gorm:"column:position"`
	Name        string    `gorm:"column:name"`
	Note        string    `gorm:"column:note"`
	MatchStatus string    `gorm:"column:match_status"`
	Options     string    `gorm:"column:options"`
	UpdatedAt   time.Time `gorm:"column:updated_at;autoUpdateTime:false"`
}

func (candidateRow) TableName() string { return "ai_curation_candidates" }

// summaryRow is a request row plus its creator and candidate counts (flat:
// GORM's Raw().Scan does not fill embedded structs).
type summaryRow struct {
	ID                 uuid.UUID  `gorm:"column:id"`
	Prompt             string     `gorm:"column:prompt"`
	MaxNames           int        `gorm:"column:max_names"`
	Status             string     `gorm:"column:status"`
	ErrorCode          *string    `gorm:"column:error_code"`
	Model              string     `gorm:"column:model"`
	PromptVersion      int        `gorm:"column:prompt_version"`
	InputTokens        int        `gorm:"column:input_tokens"`
	OutputTokens       int        `gorm:"column:output_tokens"`
	CreatedBy          *uuid.UUID `gorm:"column:created_by"`
	CreatedAt          time.Time  `gorm:"column:created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at"`
	FinishedAt         *time.Time `gorm:"column:finished_at"`
	CreatorName        *string    `gorm:"column:creator_name"`
	CreatorEmail       *string    `gorm:"column:creator_email"`
	CandidatesTotal    int        `gorm:"column:candidates_total"`
	CandidatesResolved int        `gorm:"column:candidates_resolved"`
}

// GormRepository implements the aicurator storage on GORM.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository builds a GormRepository.
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// summarySelect selects a request with its creator (LEFT JOIN users) and
// candidate counts (one grouped subquery).
const summarySelect = `
SELECT r.*, u.name AS creator_name, u.email AS creator_email,
       COALESCE(c.total, 0) AS candidates_total,
       COALESCE(c.resolved, 0) AS candidates_resolved
FROM ai_curation_requests r
LEFT JOIN users u ON u.id = r.created_by
LEFT JOIN (
    SELECT request_id, COUNT(*) AS total,
           SUM(CASE WHEN match_status <> 'pending' THEN 1 ELSE 0 END) AS resolved
    FROM ai_curation_candidates
    GROUP BY request_id
) c ON c.request_id = r.id`

// CreateRequest inserts req. A zero ID is replaced by a new UUID; req is
// updated with the stored id.
func (r *GormRepository) CreateRequest(ctx context.Context, req *domain.Request) error {
	if req.ID == uuid.Nil {
		req.ID = uuid.New()
	}
	row := requestRow{
		ID:            req.ID,
		Prompt:        req.Prompt,
		MaxNames:      req.MaxNames,
		Status:        req.Status,
		ErrorCode:     req.ErrorCode,
		Model:         req.Model,
		PromptVersion: req.PromptVersion,
		CreatedBy:     req.CreatedBy,
		CreatedAt:     req.CreatedAt,
		UpdatedAt:     req.UpdatedAt,
		FinishedAt:    req.FinishedAt,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("create curation request: %w", err)
	}
	return nil
}

// GetRequest returns the request with its creator, counts and candidates
// (by position); ErrNotFound when absent.
func (r *GormRepository) GetRequest(ctx context.Context, id uuid.UUID) (*domain.Request, error) {
	var rows []summaryRow
	if err := r.db.WithContext(ctx).Raw(summarySelect+" WHERE r.id = ?", id).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("get curation request: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	req := toRequest(rows[0])

	var cands []candidateRow
	if err := r.db.WithContext(ctx).Where("request_id = ?", id).Order("position ASC").Find(&cands).Error; err != nil {
		return nil, fmt.Errorf("list curation candidates: %w", err)
	}
	req.Candidates = make([]domain.Candidate, 0, len(cands))
	for _, c := range cands {
		req.Candidates = append(req.Candidates, toCandidate(c))
	}
	return &req, nil
}

// ListRecentRequests returns the newest limit requests of all admins.
func (r *GormRepository) ListRecentRequests(ctx context.Context, limit int) ([]domain.Request, error) {
	var rows []summaryRow
	if err := r.db.WithContext(ctx).Raw(summarySelect+" ORDER BY r.created_at DESC LIMIT ?", limit).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list curation requests: %w", err)
	}
	out := make([]domain.Request, 0, len(rows))
	for _, row := range rows {
		out = append(out, toRequest(row))
	}
	return out, nil
}

// FindActiveRequest returns the user's newest active request (queued or
// running with updated_at > staleBefore), or nil.
func (r *GormRepository) FindActiveRequest(ctx context.Context, userID uuid.UUID, staleBefore time.Time) (*domain.Request, error) {
	var rows []requestRow
	err := r.db.WithContext(ctx).
		Where("created_by = ? AND status IN ? AND updated_at > ?", userID, []string{domain.StatusQueued, domain.StatusRunning}, staleBefore).
		Order("created_at DESC").Limit(1).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("find active curation request: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	row := rows[0]
	req := toRequest(summaryRow{
		ID: row.ID, Prompt: row.Prompt, MaxNames: row.MaxNames, Status: row.Status, ErrorCode: row.ErrorCode,
		Model: row.Model, PromptVersion: row.PromptVersion, InputTokens: row.InputTokens, OutputTokens: row.OutputTokens,
		CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt, FinishedAt: row.FinishedAt,
	})
	return &req, nil
}

// CountRequestsSince counts the user's requests created after since.
func (r *GormRepository) CountRequestsSince(ctx context.Context, userID uuid.UUID, since time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Model(&requestRow{}).
		Where("created_by = ? AND created_at > ?", userID, since).Count(&n).Error
	if err != nil {
		return 0, fmt.Errorf("count curation requests: %w", err)
	}
	return n, nil
}

// MarkRunning sets a non-terminal request to running.
func (r *GormRepository) MarkRunning(ctx context.Context, id uuid.UUID, now time.Time) error {
	err := r.db.WithContext(ctx).Model(&requestRow{}).
		Where("id = ? AND status IN ?", id, []string{domain.StatusQueued, domain.StatusRunning}).
		Updates(map[string]any{"status": domain.StatusRunning, "updated_at": now}).Error
	if err != nil {
		return fmt.Errorf("mark curation request running: %w", err)
	}
	return nil
}

// MarkFailed ends a request as failed with code.
func (r *GormRepository) MarkFailed(ctx context.Context, id uuid.UUID, code string, now time.Time) error {
	err := r.db.WithContext(ctx).Model(&requestRow{}).Where("id = ?", id).
		Updates(map[string]any{"status": domain.StatusFailed, "error_code": code, "updated_at": now, "finished_at": now}).Error
	if err != nil {
		return fmt.Errorf("mark curation request failed: %w", err)
	}
	return nil
}

// MarkDone ends a request as done.
func (r *GormRepository) MarkDone(ctx context.Context, id uuid.UUID, now time.Time) error {
	err := r.db.WithContext(ctx).Model(&requestRow{}).Where("id = ?", id).
		Updates(map[string]any{"status": domain.StatusDone, "updated_at": now, "finished_at": now}).Error
	if err != nil {
		return fmt.Errorf("mark curation request done: %w", err)
	}
	return nil
}

// SaveModelResult stores the model id, the token counts and the cleaned
// candidates (match_status pending, positions 0..n-1) in one transaction.
func (r *GormRepository) SaveModelResult(ctx context.Context, id uuid.UUID, model string, inputTokens, outputTokens int, cands []domain.NewCandidate, now time.Time) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&requestRow{}).Where("id = ?", id).Updates(map[string]any{
			"model": model, "input_tokens": inputTokens, "output_tokens": outputTokens, "updated_at": now,
		}).Error; err != nil {
			return fmt.Errorf("save curation model result: %w", err)
		}
		if len(cands) == 0 {
			return nil
		}
		rows := make([]candidateRow, 0, len(cands))
		for i, c := range cands {
			rows = append(rows, candidateRow{
				ID: uuid.New(), RequestID: id, Position: i, Name: c.Name, Note: c.Note,
				MatchStatus: domain.MatchPending, Options: "[]", UpdatedAt: now,
			})
		}
		if err := tx.Create(&rows).Error; err != nil {
			return fmt.Errorf("save curation candidates: %w", err)
		}
		return nil
	})
}

// ListPendingCandidates returns the request's pending candidates by
// position.
func (r *GormRepository) ListPendingCandidates(ctx context.Context, requestID uuid.UUID) ([]domain.Candidate, error) {
	var rows []candidateRow
	err := r.db.WithContext(ctx).Where("request_id = ? AND match_status = ?", requestID, domain.MatchPending).
		Order("position ASC").Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("list pending curation candidates: %w", err)
	}
	out := make([]domain.Candidate, 0, len(rows))
	for _, row := range rows {
		out = append(out, toCandidate(row))
	}
	return out, nil
}

// SaveCandidateResolution stores a candidate's match status and options and
// bumps its request's updated_at (the progress heartbeat), in one
// transaction.
func (r *GormRepository) SaveCandidateResolution(ctx context.Context, requestID, candidateID uuid.UUID, status string, options []domain.Option, now time.Time) error {
	if options == nil {
		options = []domain.Option{}
	}
	raw, err := json.Marshal(options)
	if err != nil {
		return fmt.Errorf("marshal curation options: %w", err)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&candidateRow{}).Where("id = ? AND request_id = ?", candidateID, requestID).
			Updates(map[string]any{"match_status": status, "options": string(raw), "updated_at": now}).Error; err != nil {
			return fmt.Errorf("save curation candidate: %w", err)
		}
		if err := tx.Model(&requestRow{}).Where("id = ?", requestID).
			Update("updated_at", now).Error; err != nil {
			return fmt.Errorf("bump curation request: %w", err)
		}
		return nil
	})
}

// PurgeOlderThan deletes requests created before t (candidates cascade in
// Postgres; they are deleted explicitly too, so the purge does not depend
// on the FK action) and returns the number of deleted requests.
func (r *GormRepository) PurgeOlderThan(ctx context.Context, t time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		old := tx.Model(&requestRow{}).Select("id").Where("created_at < ?", t)
		if err := tx.Where("request_id IN (?)", old).Delete(&candidateRow{}).Error; err != nil {
			return fmt.Errorf("purge curation candidates: %w", err)
		}
		res := tx.Where("created_at < ?", t).Delete(&requestRow{})
		if res.Error != nil {
			return fmt.Errorf("purge curation requests: %w", res.Error)
		}
		n = res.RowsAffected
		return nil
	})
	return n, err
}

func toRequest(row summaryRow) domain.Request {
	req := domain.Request{
		ID:                 row.ID,
		Prompt:             row.Prompt,
		MaxNames:           row.MaxNames,
		Status:             row.Status,
		ErrorCode:          row.ErrorCode,
		Model:              row.Model,
		PromptVersion:      row.PromptVersion,
		InputTokens:        row.InputTokens,
		OutputTokens:       row.OutputTokens,
		CreatedBy:          row.CreatedBy,
		CreatedAt:          row.CreatedAt.UTC(),
		UpdatedAt:          row.UpdatedAt.UTC(),
		FinishedAt:         row.FinishedAt,
		CandidatesTotal:    row.CandidatesTotal,
		CandidatesResolved: row.CandidatesResolved,
	}
	if row.CreatedBy != nil && (row.CreatorName != nil || row.CreatorEmail != nil) {
		username := ""
		if row.CreatorName != nil {
			username = strings.TrimSpace(*row.CreatorName)
		}
		if username == "" && row.CreatorEmail != nil {
			username = *row.CreatorEmail
		}
		req.Creator = &domain.Creator{ID: *row.CreatedBy, Username: username}
	}
	return req
}

func toCandidate(row candidateRow) domain.Candidate {
	opts := []domain.Option{}
	if row.Options != "" {
		var parsed []domain.Option
		if err := json.Unmarshal([]byte(row.Options), &parsed); err == nil && parsed != nil {
			opts = parsed
		}
	}
	return domain.Candidate{
		ID:          row.ID,
		RequestID:   row.RequestID,
		Position:    row.Position,
		Name:        row.Name,
		Note:        row.Note,
		MatchStatus: row.MatchStatus,
		Options:     opts,
		UpdatedAt:   row.UpdatedAt.UTC(),
	}
}
