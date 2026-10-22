package repository

import (
	"context"
	"errors"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/curated/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Reading-status shelf types that count as activity (shelves.type values).
const (
	shelfRead     = "read"
	shelfWanted   = "want_to_read"
	shelfReading  = "reading"
	insertBatches = 500
)

// aggRow is one candidate book of an aggregate query.
type aggRow struct {
	BookID       uuid.UUID
	Title        string
	RatingsCount int
	RatingSum    int64
	RatingCount  int64
	SignalCount  int64
}

func (a aggRow) toDomain() domain.AutoRow {
	return domain.AutoRow{
		BookID: a.BookID, Title: a.Title, RatingsCount: a.RatingsCount,
		RatingSum: a.RatingSum, RatingCount: a.RatingCount, SignalCount: a.SignalCount,
	}
}

// txSource runs the aggregate queries on a (transaction) handle.
type txSource struct{ db *gorm.DB }

// scope adds the published-book join and the optional genre filter to a
// signal query whose book column is bookCol.
func (s txSource) scope(q *gorm.DB, bookCol string, c domain.Collection) *gorm.DB {
	q = q.Joins("JOIN books b ON b.id = "+bookCol+" AND b.status = ?", catalogDomain.BookStatusPublished)
	if c.GenreID != nil {
		q = q.Joins("JOIN book_genres bg ON bg.book_id = b.id AND bg.genre_id = ?", *c.GenreID)
	}
	return q
}

func (s txSource) ratings(ctx context.Context, c domain.Collection, since *time.Time, signal bool) ([]aggRow, error) {
	sel := "r.book_id AS book_id, b.title AS title, b.ratings_count AS ratings_count, " +
		"SUM(r.value) AS rating_sum, COUNT(*) AS rating_count"
	if signal {
		sel = "r.book_id AS book_id, b.title AS title, b.ratings_count AS ratings_count, COUNT(*) AS signal_count"
	}
	q := s.scope(s.db.WithContext(ctx).Table("ratings AS r").Select(sel), "r.book_id", c)
	if since != nil {
		q = q.Where("r.updated_at >= ?", *since)
	}
	var rows []aggRow
	err := q.Group("r.book_id, b.title, b.ratings_count").Scan(&rows).Error
	return rows, err
}

func (s txSource) shelves(ctx context.Context, c domain.Collection, since *time.Time, types ...string) ([]aggRow, error) {
	q := s.scope(s.db.WithContext(ctx).Table("shelf_items AS si").
		Select("si.book_id AS book_id, b.title AS title, b.ratings_count AS ratings_count, COUNT(*) AS signal_count").
		Joins("JOIN shelves s ON s.id = si.shelf_id AND s.type IN ?", types), "si.book_id", c)
	if since != nil {
		q = q.Where("si.added_at >= ?", *since)
	}
	var rows []aggRow
	err := q.Group("si.book_id, b.title, b.ratings_count").Scan(&rows).Error
	return rows, err
}

// AutoRows returns one aggregate row per candidate book for the collection's
// metric. The caller (domain.RankAuto) scores and filters them.
func (s txSource) AutoRows(ctx context.Context, c domain.Collection, since *time.Time) ([]domain.AutoRow, error) {
	if c.RuleMetric == nil {
		return nil, domain.ErrNotAutomatic
	}
	var rows []aggRow
	var err error
	switch *c.RuleMetric {
	case domain.MetricWeightedRating:
		rows, err = s.ratings(ctx, c, since, false)
	case domain.MetricMostRead:
		rows, err = s.shelves(ctx, c, since, shelfRead)
	case domain.MetricMostWanted:
		rows, err = s.shelves(ctx, c, since, shelfWanted)
	case domain.MetricTrending:
		rows, err = s.trending(ctx, c, since)
	default:
		err = errors.New("unknown metric " + *c.RuleMetric)
	}
	if err != nil {
		return nil, err
	}
	out := make([]domain.AutoRow, len(rows))
	for i, r := range rows {
		out[i] = r.toDomain()
	}
	return out, nil
}

// trending sums the ratings set or changed and the reading-status shelvings.
func (s txSource) trending(ctx context.Context, c domain.Collection, since *time.Time) ([]aggRow, error) {
	rated, err := s.ratings(ctx, c, since, true)
	if err != nil {
		return nil, err
	}
	shelved, err := s.shelves(ctx, c, since, shelfRead, shelfReading, shelfWanted)
	if err != nil {
		return nil, err
	}
	idx := make(map[uuid.UUID]int, len(rated)+len(shelved))
	var merged []aggRow
	for _, list := range [][]aggRow{rated, shelved} {
		for _, r := range list {
			if i, ok := idx[r.BookID]; ok {
				merged[i].SignalCount += r.SignalCount
				continue
			}
			idx[r.BookID] = len(merged)
			merged = append(merged, r)
		}
	}
	return merged, nil
}

// verdictAgg is one visible adaptation of a published book with its votes.
type verdictAgg struct {
	BookID          uuid.UUID
	ScreenWorkID    uuid.UUID
	Title           string
	RatingsCount    int
	WorkTitle       string
	WorkYear        *int
	Votes           int64
	BookVotes       int64
	AdaptationVotes int64
}

// VerdictRows aggregates the counted adaptation votes per (book, adaptation):
// visible links, published books, the genre scope and votes since the cut-off.
func (s txSource) VerdictRows(ctx context.Context, c domain.Collection, since *time.Time) ([]domain.VerdictRow, error) {
	q := s.scope(s.db.WithContext(ctx).Table("adaptation_votes AS v").
		Select("v.book_id AS book_id, v.screen_work_id AS screen_work_id, b.title AS title, "+
			"b.ratings_count AS ratings_count, w.title AS work_title, w.year AS work_year, "+
			"COUNT(*) AS votes, "+
			"SUM(CASE WHEN v.value = 'book' THEN 1 ELSE 0 END) AS book_votes, "+
			"SUM(CASE WHEN v.value = 'adaptation' THEN 1 ELSE 0 END) AS adaptation_votes").
		Joins("JOIN book_screen_works l ON l.book_id = v.book_id AND l.screen_work_id = v.screen_work_id AND NOT l.hidden").
		Joins("JOIN screen_works w ON w.id = v.screen_work_id"), "v.book_id", c)
	if since != nil {
		q = q.Where("v.updated_at >= ?", *since)
	}
	var rows []verdictAgg
	if err := q.Group("v.book_id, v.screen_work_id, b.title, b.ratings_count, w.title, w.year").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]domain.VerdictRow, len(rows))
	for i, r := range rows {
		out[i] = domain.VerdictRow{
			BookID: r.BookID, Title: r.Title, RatingsCount: r.RatingsCount, ScreenWorkID: r.ScreenWorkID,
			WorkTitle: r.WorkTitle, WorkYear: r.WorkYear, Votes: r.Votes, BookVotes: r.BookVotes, AdaptationVotes: r.AdaptationVotes,
		}
	}
	return out, nil
}

func (r *GormRepository) AutoCollectionIDs(ctx context.Context) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.db.WithContext(ctx).Model(&domain.Collection{}).
		Where("kind = ?", domain.KindAuto).Order("created_at ASC").Pluck("id", &ids).Error
	return ids, err
}

func (r *GormRepository) Recompute(ctx context.Context, id uuid.UUID, reset bool, now time.Time, rank RankFunc) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := lock(tx, id); err != nil {
			return err
		}
		// Read under the lock: a concurrent edit has either committed (and is
		// seen here) or is waiting for the lock.
		var c domain.Collection
		if err := tx.Where("id = ?", id).First(&c).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return domain.ErrCollectionNotFound
			}
			return err
		}
		if !c.IsAuto() {
			return domain.ErrNotAutomatic
		}
		ranked, err := rank(ctx, c, txSource{db: tx})
		if err != nil {
			return err
		}

		prev := map[uuid.UUID]int{}
		if !reset {
			var old []domain.CollectionItem
			if err := tx.Select("book_id", "position").Where("collection_id = ?", id).Find(&old).Error; err != nil {
				return err
			}
			for _, it := range old {
				prev[it.BookID] = it.Position
			}
		}
		if err := tx.Where("collection_id = ?", id).Delete(&domain.CollectionItem{}).Error; err != nil {
			return err
		}
		if len(ranked) > 0 {
			items := make([]domain.CollectionItem, len(ranked))
			for i, sb := range ranked {
				score := sb.Score
				it := domain.CollectionItem{CollectionID: id, BookID: sb.BookID, Position: i + 1, AddedAt: now, Score: &score}
				if sb.Source != nil {
					wid, votes, share := sb.Source.ScreenWorkID, sb.Source.Votes, sb.Source.Share
					it.SourceScreenWorkID, it.SourceVotes, it.SourceShare = &wid, &votes, &share
				}
				if p, ok := prev[sb.BookID]; ok {
					p := p
					it.PreviousPosition = &p
				}
				items[i] = it
			}
			if err := tx.CreateInBatches(items, insertBatches).Error; err != nil {
				return err
			}
		}

		var baseline any
		if !reset && c.ComputedAt != nil {
			baseline = *c.ComputedAt
		}
		return tx.Model(&domain.Collection{}).Where("id = ?", id).
			Updates(map[string]any{"computed_at": now, "baseline_at": baseline, "updated_at": now}).Error
	})
}
