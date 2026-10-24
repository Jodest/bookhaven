package repository

import (
	"context"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/google/uuid"
)

// Votes (openspec change add-adaptation-verdict-votes). adaptation_votes rows
// belong to a book_screen_works link and are removed by the composite foreign
// key when the link or the book is deleted. A hidden link keeps its rows; the
// reads below skip it.
//
// NOTE: there is no book merge or screen-work merge today. Any future one MUST
// move adaptation_votes to the surviving book or work (keeping the row with
// the latest updated_at on a primary key conflict); otherwise the cascade
// silently deletes the votes.

// VoteRepository is the persistence port of the adaptation votes.
type VoteRepository interface {
	// LinkVisible reports whether the work has a non-hidden link to the book.
	LinkVisible(ctx context.Context, bookID, workID uuid.UUID) (bool, error)
	// IsReader reports whether the user has at least one finished reading of the
	// book in the reading journal (book_reads).
	IsReader(ctx context.Context, userID, bookID uuid.UUID) (bool, error)
	// UpsertVote stores the vote; updated_at moves only when the value changes.
	UpsertVote(ctx context.Context, userID, bookID, workID uuid.UUID, value string) error
	// DeleteVote removes the caller's vote (no error without one).
	DeleteVote(ctx context.Context, userID, bookID, workID uuid.UUID) error
	// ListVerdicts returns the counts of every visible link of the book in the
	// public order; userID (optional) fills MyVote.
	ListVerdicts(ctx context.Context, bookID uuid.UUID, userID *uuid.UUID) ([]domain.VerdictCounts, error)
}

func (r *GormRepository) LinkVisible(ctx context.Context, bookID, workID uuid.UUID) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("book_screen_works").
		Where("book_id = ? AND screen_work_id = ? AND hidden = ?", bookID, workID, false).
		Count(&n).Error
	return n > 0, err
}

func (r *GormRepository) IsReader(ctx context.Context, userID, bookID uuid.UUID) (bool, error) {
	var n int64
	err := r.db.WithContext(ctx).Table("book_reads").
		Where("user_id = ? AND book_id = ? AND finished_at IS NOT NULL", userID, bookID).
		Count(&n).Error
	return n > 0, err
}

func (r *GormRepository) UpsertVote(ctx context.Context, userID, bookID, workID uuid.UUID, value string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Exec(`INSERT INTO adaptation_votes (user_id, book_id, screen_work_id, value, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
ON CONFLICT (user_id, book_id, screen_work_id) DO UPDATE SET value = EXCLUDED.value, updated_at = EXCLUDED.updated_at
WHERE adaptation_votes.value <> EXCLUDED.value`,
		userID, bookID, workID, value, now, now).Error
}

func (r *GormRepository) DeleteVote(ctx context.Context, userID, bookID, workID uuid.UUID) error {
	return r.db.WithContext(ctx).Exec(
		"DELETE FROM adaptation_votes WHERE user_id = ? AND book_id = ? AND screen_work_id = ?",
		userID, bookID, workID).Error
}

type verdictRow struct {
	ScreenWorkID     uuid.UUID `gorm:"column:screen_work_id"`
	Votes            int       `gorm:"column:votes"`
	BookBetter       int       `gorm:"column:book_better"`
	AdaptationBetter int       `gorm:"column:adaptation_better"`
	Same             int       `gorm:"column:same"`
}

func (r *GormRepository) ListVerdicts(ctx context.Context, bookID uuid.UUID, userID *uuid.UUID) ([]domain.VerdictCounts, error) {
	var rows []verdictRow
	err := r.db.WithContext(ctx).Table("book_screen_works AS l").
		Joins("JOIN screen_works w ON w.id = l.screen_work_id").
		Joins("LEFT JOIN adaptation_votes v ON v.book_id = l.book_id AND v.screen_work_id = l.screen_work_id").
		Select(`l.screen_work_id AS screen_work_id, COUNT(v.user_id) AS votes,
COUNT(CASE WHEN v.value = 'book' THEN 1 END) AS book_better,
COUNT(CASE WHEN v.value = 'adaptation' THEN 1 END) AS adaptation_better,
COUNT(CASE WHEN v.value = 'same' THEN 1 END) AS same`).
		Where("l.book_id = ? AND l.hidden = ?", bookID, false).
		Group("l.screen_work_id, w.year, w.title, w.id").
		Order(linkedOrder).Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	mine := map[uuid.UUID]string{}
	if userID != nil {
		var own []struct {
			ScreenWorkID uuid.UUID `gorm:"column:screen_work_id"`
			Value        string    `gorm:"column:value"`
		}
		err := r.db.WithContext(ctx).Table("adaptation_votes").Select("screen_work_id, value").
			Where("user_id = ? AND book_id = ?", *userID, bookID).Scan(&own).Error
		if err != nil {
			return nil, err
		}
		for _, o := range own {
			mine[o.ScreenWorkID] = o.Value
		}
	}

	out := make([]domain.VerdictCounts, 0, len(rows))
	for _, row := range rows {
		c := domain.VerdictCounts{
			ScreenWorkID: row.ScreenWorkID, Votes: row.Votes,
			BookBetter: row.BookBetter, AdaptationBetter: row.AdaptationBetter, Same: row.Same,
		}
		if v, ok := mine[row.ScreenWorkID]; ok {
			c.MyVote = &v
		}
		out = append(out, c)
	}
	return out, nil
}
