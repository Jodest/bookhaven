// Package repository is the GORM storage of the AI genre analysis runs and
// suggestions (tables genre_ai_runs, genre_ai_suggestions and
// genre_ai_suggestion_genres, migration 000032). These tables are not genre
// data, so the module owns its repository; it never reads or writes the
// genres, book_genres or collections tables (the service reaches them through
// the admin service). Every timestamp is passed in by the caller (UTC).
package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrNotFound: no such run or suggestion.
var ErrNotFound = errors.New("aigenres: not found")

type runRow struct {
	ID                 uuid.UUID  `gorm:"column:id;primaryKey"`
	Trigger            string     `gorm:"column:trigger"`
	Status             string     `gorm:"column:status"`
	ErrorCode          *string    `gorm:"column:error_code"`
	Model              string     `gorm:"column:model"`
	PromptVersion      int        `gorm:"column:prompt_version"`
	Fingerprint        string     `gorm:"column:fingerprint"`
	BatchesTotal       int        `gorm:"column:batches_total"`
	BatchesDone        int        `gorm:"column:batches_done"`
	SuggestionsCreated int        `gorm:"column:suggestions_created"`
	InputTokens        int        `gorm:"column:input_tokens"`
	OutputTokens       int        `gorm:"column:output_tokens"`
	CreatedBy          *uuid.UUID `gorm:"column:created_by"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime:false"`
	StartedAt          *time.Time `gorm:"column:started_at"`
	FinishedAt         *time.Time `gorm:"column:finished_at"`
}

func (runRow) TableName() string { return "genre_ai_runs" }

type suggestionRow struct {
	ID              uuid.UUID  `gorm:"column:id;primaryKey"`
	RunID           *uuid.UUID `gorm:"column:run_id"`
	Kind            string     `gorm:"column:kind"`
	Status          string     `gorm:"column:status"`
	SetKey          string     `gorm:"column:set_key"`
	ReasonCodes     string     `gorm:"column:reason_codes"`
	Reason          string     `gorm:"column:reason"`
	Confidence      float64    `gorm:"column:confidence"`
	Model           string     `gorm:"column:model"`
	PromptVersion   int        `gorm:"column:prompt_version"`
	AppliedTargetID *uuid.UUID `gorm:"column:applied_target_id"`
	DecidedBy       *uuid.UUID `gorm:"column:decided_by"`
	DecidedAt       *time.Time `gorm:"column:decided_at"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime:false"`
}

func (suggestionRow) TableName() string { return "genre_ai_suggestions" }

type suggestionGenreRow struct {
	SuggestionID uuid.UUID `gorm:"column:suggestion_id;primaryKey"`
	GenreID      uuid.UUID `gorm:"column:genre_id;primaryKey"`
	Role         string    `gorm:"column:role"`
	Position     int       `gorm:"column:position"`
	NameSnapshot string    `gorm:"column:name_snapshot"`
}

func (suggestionGenreRow) TableName() string { return "genre_ai_suggestion_genres" }

// GormRepository implements the aigenres storage on GORM.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository builds a GormRepository.
func NewGormRepository(db *gorm.DB) *GormRepository {
	return &GormRepository{db: db}
}

// ---- runs ----

func toRun(r runRow) domain.Run {
	return domain.Run{
		ID: r.ID, Trigger: r.Trigger, Status: r.Status, ErrorCode: r.ErrorCode, Model: r.Model,
		PromptVersion: r.PromptVersion, Fingerprint: r.Fingerprint, BatchesTotal: r.BatchesTotal,
		BatchesDone: r.BatchesDone, SuggestionsCreated: r.SuggestionsCreated,
		InputTokens: r.InputTokens, OutputTokens: r.OutputTokens, CreatedBy: r.CreatedBy,
		CreatedAt: r.CreatedAt.UTC(), StartedAt: r.StartedAt, FinishedAt: r.FinishedAt,
	}
}

// CreateRun inserts run. A zero ID is replaced by a new UUID; run is updated
// with the stored id.
func (r *GormRepository) CreateRun(ctx context.Context, run *domain.Run) error {
	if run.ID == uuid.Nil {
		run.ID = uuid.New()
	}
	row := runRow{
		ID: run.ID, Trigger: run.Trigger, Status: run.Status, ErrorCode: run.ErrorCode, Model: run.Model,
		PromptVersion: run.PromptVersion, Fingerprint: run.Fingerprint, CreatedBy: run.CreatedBy,
		CreatedAt: run.CreatedAt,
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return fmt.Errorf("create genre ai run: %w", err)
	}
	return nil
}

// GetRun returns the run; ErrNotFound when absent.
func (r *GormRepository) GetRun(ctx context.Context, id uuid.UUID) (*domain.Run, error) {
	var rows []runRow
	if err := r.db.WithContext(ctx).Where("id = ?", id).Limit(1).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("get genre ai run: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	run := toRun(rows[0])
	return &run, nil
}

// LatestRun returns the newest run, or nil when none exists.
func (r *GormRepository) LatestRun(ctx context.Context) (*domain.Run, error) {
	var rows []runRow
	if err := r.db.WithContext(ctx).Order("created_at DESC").Limit(1).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("latest genre ai run: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	run := toRun(rows[0])
	return &run, nil
}

// ActiveRun returns a queued or running run, or nil.
func (r *GormRepository) ActiveRun(ctx context.Context) (*domain.Run, error) {
	var rows []runRow
	err := r.db.WithContext(ctx).
		Where("status IN ?", []string{domain.RunQueued, domain.RunRunning}).
		Order("created_at DESC").Limit(1).Find(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("active genre ai run: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	run := toRun(rows[0])
	return &run, nil
}

// InterruptStaleRuns marks queued/running runs created before cutoff as
// failed with code interrupted and returns how many were changed.
func (r *GormRepository) InterruptStaleRuns(ctx context.Context, cutoff, now time.Time) (int64, error) {
	res := r.db.WithContext(ctx).Model(&runRow{}).
		Where("status IN ? AND created_at < ?", []string{domain.RunQueued, domain.RunRunning}, cutoff).
		Updates(map[string]any{"status": domain.RunFailed, "error_code": domain.ErrorInterrupted, "finished_at": now})
	if res.Error != nil {
		return 0, fmt.Errorf("interrupt genre ai runs: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// StartRun sets a queued run to running with its plan.
func (r *GormRepository) StartRun(ctx context.Context, id uuid.UUID, model, fingerprint string, batchesTotal int, now time.Time) error {
	err := r.db.WithContext(ctx).Model(&runRow{}).
		Where("id = ? AND status IN ?", id, []string{domain.RunQueued, domain.RunRunning}).
		Updates(map[string]any{
			"status": domain.RunRunning, "model": model, "fingerprint": fingerprint,
			"batches_total": batchesTotal, "started_at": now,
		}).Error
	if err != nil {
		return fmt.Errorf("start genre ai run: %w", err)
	}
	return nil
}

// BatchResult is the bookkeeping of one finished batch.
type BatchResult struct {
	SuggestionsCreated int
	InputTokens        int
	OutputTokens       int
	// ErrorCode, when non-nil, becomes the run's last failure code.
	ErrorCode *string
}

// AddBatchResult records one finished batch on the run.
func (r *GormRepository) AddBatchResult(ctx context.Context, id uuid.UUID, b BatchResult) error {
	updates := map[string]any{
		"batches_done":        gorm.Expr("batches_done + 1"),
		"suggestions_created": gorm.Expr("suggestions_created + ?", b.SuggestionsCreated),
		"input_tokens":        gorm.Expr("input_tokens + ?", b.InputTokens),
		"output_tokens":       gorm.Expr("output_tokens + ?", b.OutputTokens),
	}
	if b.ErrorCode != nil {
		updates["error_code"] = *b.ErrorCode
	}
	if err := r.db.WithContext(ctx).Model(&runRow{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("add genre ai batch result: %w", err)
	}
	return nil
}

// FinishRun ends the run with status (done or failed). A nil errorCode keeps
// the code recorded so far.
func (r *GormRepository) FinishRun(ctx context.Context, id uuid.UUID, status string, errorCode *string, now time.Time) error {
	updates := map[string]any{"status": status, "finished_at": now}
	if errorCode != nil {
		updates["error_code"] = *errorCode
	}
	if err := r.db.WithContext(ctx).Model(&runRow{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return fmt.Errorf("finish genre ai run: %w", err)
	}
	return nil
}

// LastCleanFingerprint returns the fingerprint of the newest run that ended
// done without any recorded failure code, or "" when there is none. A run
// with failed batches is not clean, so its catalog is analyzed again.
func (r *GormRepository) LastCleanFingerprint(ctx context.Context) (string, error) {
	var rows []runRow
	err := r.db.WithContext(ctx).
		Where("status = ? AND error_code IS NULL", domain.RunDone).
		Order("finished_at DESC").Limit(1).Find(&rows).Error
	if err != nil {
		return "", fmt.Errorf("last genre ai fingerprint: %w", err)
	}
	if len(rows) == 0 {
		return "", nil
	}
	return rows[0].Fingerprint, nil
}

// PurgeRunsOlderThan deletes runs created before t (suggestions keep living:
// their run_id is set to NULL) and returns the number of deleted runs.
func (r *GormRepository) PurgeRunsOlderThan(ctx context.Context, t time.Time) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Explicit, so the purge does not depend on the FK action.
		old := tx.Model(&runRow{}).Select("id").Where("created_at < ?", t)
		if err := tx.Model(&suggestionRow{}).Where("run_id IN (?)", old).Update("run_id", nil).Error; err != nil {
			return fmt.Errorf("detach genre ai suggestions: %w", err)
		}
		res := tx.Where("created_at < ?", t).Delete(&runRow{})
		if res.Error != nil {
			return fmt.Errorf("purge genre ai runs: %w", res.Error)
		}
		n = res.RowsAffected
		return nil
	})
	return n, err
}

// ---- suggestions ----

func toSuggestion(row suggestionRow, genres []suggestionGenreRow) domain.Suggestion {
	var codes []string
	if err := json.Unmarshal([]byte(row.ReasonCodes), &codes); err != nil || codes == nil {
		codes = []string{}
	}
	s := domain.Suggestion{
		ID: row.ID, RunID: row.RunID, Kind: row.Kind, Status: row.Status, SetKey: row.SetKey,
		ReasonCodes: codes, Reason: row.Reason, Confidence: row.Confidence, Model: row.Model,
		PromptVersion: row.PromptVersion, AppliedTargetID: row.AppliedTargetID, DecidedBy: row.DecidedBy,
		DecidedAt: row.DecidedAt, CreatedAt: row.CreatedAt.UTC(),
		Genres: make([]domain.SuggestionGenre, 0, len(genres)),
	}
	for _, g := range genres {
		s.Genres = append(s.Genres, domain.SuggestionGenre{
			GenreID: g.GenreID, Role: g.Role, Position: g.Position, NameSnapshot: g.NameSnapshot,
		})
	}
	return s
}

// toSuggestions maps rows and loads their genres (target first, then by position).
func (r *GormRepository) toSuggestions(ctx context.Context, rows []suggestionRow) ([]domain.Suggestion, error) {
	out := make([]domain.Suggestion, 0, len(rows))
	if len(rows) == 0 {
		return out, nil
	}
	ids := make([]uuid.UUID, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	var genreRows []suggestionGenreRow
	if err := r.db.WithContext(ctx).Where("suggestion_id IN ?", ids).
		Order("position ASC").Find(&genreRows).Error; err != nil {
		return nil, fmt.Errorf("load genre ai suggestion genres: %w", err)
	}
	bySuggestion := make(map[uuid.UUID][]suggestionGenreRow, len(rows))
	for _, g := range genreRows {
		bySuggestion[g.SuggestionID] = append(bySuggestion[g.SuggestionID], g)
	}
	for _, row := range rows {
		out = append(out, toSuggestion(row, bySuggestion[row.ID]))
	}
	return out, nil
}

// InsertSuggestions stores the suggestions of one batch in one transaction and
// returns how many were stored. A suggestion is skipped when an open or
// dismissed suggestion has the same set, or when one of its genres belongs to
// an open suggestion (including one stored earlier in this call). The unique
// index on set_key is the last guard against a concurrent writer.
func (r *GormRepository) InsertSuggestions(ctx context.Context, items []domain.NewSuggestion, runID uuid.UUID, now time.Time) (int, error) {
	created := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created = 0
		for i := range items {
			item := &items[i]
			key := item.SetKeyOf()

			var same int64
			if err := tx.Model(&suggestionRow{}).
				Where("set_key = ? AND status IN ?", key, []string{domain.StatusOpen, domain.StatusDismissed}).
				Count(&same).Error; err != nil {
				return fmt.Errorf("check genre ai set: %w", err)
			}
			if same > 0 {
				continue
			}
			genreIDs := make([]uuid.UUID, 0, len(item.Genres))
			for _, g := range item.Genres {
				genreIDs = append(genreIDs, g.GenreID)
			}
			var overlap int64
			if err := tx.Table("genre_ai_suggestion_genres AS g").
				Joins("JOIN genre_ai_suggestions s ON s.id = g.suggestion_id").
				Where("s.status = ? AND g.genre_id IN ?", domain.StatusOpen, genreIDs).
				Count(&overlap).Error; err != nil {
				return fmt.Errorf("check genre ai overlap: %w", err)
			}
			if overlap > 0 {
				continue
			}

			codes := item.ReasonCodes
			if codes == nil {
				codes = []string{}
			}
			rawCodes, err := json.Marshal(codes)
			if err != nil {
				return fmt.Errorf("marshal genre ai reason codes: %w", err)
			}
			row := suggestionRow{
				ID: uuid.New(), RunID: &runID, Kind: item.Kind, Status: domain.StatusOpen, SetKey: key,
				ReasonCodes: string(rawCodes), Reason: item.Reason, Confidence: item.Confidence,
				Model: item.Model, PromptVersion: item.PromptVersion, CreatedAt: now,
			}
			res := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if res.Error != nil {
				return fmt.Errorf("save genre ai suggestion: %w", res.Error)
			}
			if res.RowsAffected == 0 {
				continue
			}
			genreRows := make([]suggestionGenreRow, 0, len(item.Genres))
			for pos, g := range item.Genres {
				genreRows = append(genreRows, suggestionGenreRow{
					SuggestionID: row.ID, GenreID: g.GenreID, Role: g.Role, Position: pos, NameSnapshot: g.NameSnapshot,
				})
			}
			if err := tx.Create(&genreRows).Error; err != nil {
				return fmt.Errorf("save genre ai suggestion genres: %w", err)
			}
			created++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return created, nil
}

// ListFilter selects suggestions for the list endpoint. Status is required.
type ListFilter struct {
	Status string
	Kind   string
	Limit  int
	Offset int
}

// ListSuggestions returns one page of suggestions and the total. Open ones are
// ordered by confidence (highest first) then newest; the others by their
// decision time, newest first.
func (r *GormRepository) ListSuggestions(ctx context.Context, f ListFilter) ([]domain.Suggestion, int64, error) {
	q := r.db.WithContext(ctx).Model(&suggestionRow{}).Where("status = ?", f.Status)
	if f.Kind != "" {
		q = q.Where("kind = ?", f.Kind)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count genre ai suggestions: %w", err)
	}
	order := "COALESCE(decided_at, created_at) DESC, id ASC"
	if f.Status == domain.StatusOpen {
		order = "confidence DESC, created_at DESC, id ASC"
	}
	var rows []suggestionRow
	if err := q.Order(order).Limit(f.Limit).Offset(f.Offset).Find(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list genre ai suggestions: %w", err)
	}
	items, err := r.toSuggestions(ctx, rows)
	return items, total, err
}

// GetSuggestion returns the suggestion with its genres; ErrNotFound when
// absent.
func (r *GormRepository) GetSuggestion(ctx context.Context, id uuid.UUID) (*domain.Suggestion, error) {
	var rows []suggestionRow
	if err := r.db.WithContext(ctx).Where("id = ?", id).Limit(1).Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("get genre ai suggestion: %w", err)
	}
	if len(rows) == 0 {
		return nil, ErrNotFound
	}
	items, err := r.toSuggestions(ctx, rows)
	if err != nil {
		return nil, err
	}
	return &items[0], nil
}

// ListOpenSuggestions returns every open suggestion with its genres.
func (r *GormRepository) ListOpenSuggestions(ctx context.Context) ([]domain.Suggestion, error) {
	var rows []suggestionRow
	if err := r.db.WithContext(ctx).Where("status = ?", domain.StatusOpen).
		Order("created_at ASC, id ASC").Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list open genre ai suggestions: %w", err)
	}
	return r.toSuggestions(ctx, rows)
}

// CountOpen returns the number of open suggestions.
func (r *GormRepository) CountOpen(ctx context.Context) (int64, error) {
	var n int64
	if err := r.db.WithContext(ctx).Model(&suggestionRow{}).
		Where("status = ?", domain.StatusOpen).Count(&n).Error; err != nil {
		return 0, fmt.Errorf("count open genre ai suggestions: %w", err)
	}
	return n, nil
}

// MarkStale sets the open suggestions among ids to stale and returns how many
// were changed.
func (r *GormRepository) MarkStale(ctx context.Context, ids []uuid.UUID) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res := r.db.WithContext(ctx).Model(&suggestionRow{}).
		Where("id IN ? AND status = ?", ids, domain.StatusOpen).
		Update("status", domain.StatusStale)
	if res.Error != nil {
		return 0, fmt.Errorf("mark genre ai suggestions stale: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// MarkApplied sets an open suggestion to applied. changed is false when the
// suggestion was no longer open (a concurrent decision won).
func (r *GormRepository) MarkApplied(ctx context.Context, id uuid.UUID, by uuid.UUID, appliedTarget *uuid.UUID, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&suggestionRow{}).
		Where("id = ? AND status = ?", id, domain.StatusOpen).
		Updates(map[string]any{
			"status": domain.StatusApplied, "decided_by": by, "decided_at": now, "applied_target_id": appliedTarget,
		})
	if res.Error != nil {
		return false, fmt.Errorf("mark genre ai suggestion applied: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// MarkDismissed sets an open suggestion to dismissed. changed is false when
// the suggestion was no longer open.
func (r *GormRepository) MarkDismissed(ctx context.Context, id uuid.UUID, by uuid.UUID, now time.Time) (bool, error) {
	res := r.db.WithContext(ctx).Model(&suggestionRow{}).
		Where("id = ? AND status = ?", id, domain.StatusOpen).
		Updates(map[string]any{"status": domain.StatusDismissed, "decided_by": by, "decided_at": now})
	if res.Error != nil {
		return false, fmt.Errorf("mark genre ai suggestion dismissed: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}
