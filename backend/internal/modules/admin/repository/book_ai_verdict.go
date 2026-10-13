package repository

import (
	"context"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// AI pre-moderation verdicts (openspec change add-ai-book-premoderation,
// capability catalog-moderation/ai-verdicts). The table is owned by the
// admin module; the aimoderation module reaches it through the admin
// service only.

// BookAIVerdict is one row of book_ai_verdicts. ReasonCodes holds a JSON
// array of strings (text, not TEXT[], so SQLite tests keep working).
type BookAIVerdict struct {
	BookID        uuid.UUID `gorm:"column:book_id;primaryKey"`
	Verdict       string    `gorm:"column:verdict"`
	ReasonCodes   string    `gorm:"column:reason_codes"`
	Reason        string    `gorm:"column:reason"`
	Confidence    float64   `gorm:"column:confidence"`
	Model         string    `gorm:"column:model"`
	PromptVersion int       `gorm:"column:prompt_version"`
	InputHash     string    `gorm:"column:input_hash"`
	InputTokens   int       `gorm:"column:input_tokens"`
	OutputTokens  int       `gorm:"column:output_tokens"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime:false"`
}

// TableName binds BookAIVerdict to book_ai_verdicts.
func (BookAIVerdict) TableName() string { return "book_ai_verdicts" }

// aiReviewCandidateWhere selects pending books the periodic sweep should
// (re)check: no verdict, an older prompt version, or a book updated after
// its verdict. The single bind parameter is the current prompt version.
const aiReviewCandidateWhere = "b.status = 'pending' AND (v.book_id IS NULL OR v.prompt_version <> ? OR b.updated_at > v.created_at)"

func (r *GormRepository) UpsertBookAIVerdict(ctx context.Context, v BookAIVerdict) error {
	// CURRENT_TIMESTAMP (not Go time) so created_at is DB time on insert
	// and on replace; works on Postgres and SQLite.
	return r.db.WithContext(ctx).Exec(`
INSERT INTO book_ai_verdicts
    (book_id, verdict, reason_codes, reason, confidence, model, prompt_version, input_hash, input_tokens, output_tokens, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
ON CONFLICT (book_id) DO UPDATE SET
    verdict = excluded.verdict,
    reason_codes = excluded.reason_codes,
    reason = excluded.reason,
    confidence = excluded.confidence,
    model = excluded.model,
    prompt_version = excluded.prompt_version,
    input_hash = excluded.input_hash,
    input_tokens = excluded.input_tokens,
    output_tokens = excluded.output_tokens,
    created_at = CURRENT_TIMESTAMP`,
		v.BookID, v.Verdict, v.ReasonCodes, v.Reason, v.Confidence, v.Model, v.PromptVersion, v.InputHash, v.InputTokens, v.OutputTokens,
	).Error
}

func (r *GormRepository) GetBookAIVerdicts(ctx context.Context, bookIDs []uuid.UUID) (map[uuid.UUID]BookAIVerdict, error) {
	out := make(map[uuid.UUID]BookAIVerdict, len(bookIDs))
	if len(bookIDs) == 0 {
		return out, nil
	}
	var rows []BookAIVerdict
	if err := r.db.WithContext(ctx).Where("book_id IN ?", bookIDs).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[row.BookID] = row
	}
	return out, nil
}

func (r *GormRepository) ListAIReviewCandidateIDs(ctx context.Context, limit, promptVersion int) ([]uuid.UUID, error) {
	ids := []uuid.UUID{}
	if limit <= 0 {
		return ids, nil
	}
	err := r.db.WithContext(ctx).Table("books AS b").
		Select("b.id").
		Joins("LEFT JOIN book_ai_verdicts v ON v.book_id = b.id").
		Where(aiReviewCandidateWhere, promptVersion).
		Order("(v.book_id IS NULL) DESC, b.created_at ASC, b.id ASC").
		Limit(limit).
		Pluck("b.id", &ids).Error
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (r *GormRepository) CountAIReviewCandidates(ctx context.Context, promptVersion int) (int64, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("books AS b").
		Joins("LEFT JOIN book_ai_verdicts v ON v.book_id = b.id").
		Where(aiReviewCandidateWhere, promptVersion).
		Count(&n).Error
	return n, err
}

func (r *GormRepository) LoadBooksForReview(ctx context.Context, ids []uuid.UUID) ([]catalogDomain.Book, error) {
	books := []catalogDomain.Book{}
	if len(ids) == 0 {
		return books, nil
	}
	if err := r.db.WithContext(ctx).
		Preload("Authors").
		Preload("Genres").
		Where("id IN ?", ids).
		Find(&books).Error; err != nil {
		return nil, err
	}
	return books, nil
}
