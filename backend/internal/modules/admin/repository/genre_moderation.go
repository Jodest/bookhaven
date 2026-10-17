package repository

import (
	"context"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Genre moderation queue and merge (docs/features/genre-moderation-queue.md).

// GenreModerationFilter defines filtering options for the genre moderation
// queue. Status is a valid genre status or "" (any); QID is "" (any),
// "with" or "without". Search uses the same name ILIKE condition as
// ListGenres. Alias rows are always excluded.
type GenreModerationFilter struct {
	Status string
	QID    string
	Search string
	Limit  int
	Offset int
}

// GenreModerationRow is one genre of the moderation queue with its
// linked-books counts (books_visible = linked books with status = 'published').
type GenreModerationRow struct {
	Genre        catalogDomain.Genre
	BooksTotal   int64
	BooksVisible int64
}

// genreBookStatsJoin attaches per-genre linked-books counts. Portable SQL
// (SUM(CASE ...), not FILTER) so the SQLite repository tests run it.
const genreBookStatsJoin = `LEFT JOIN (
	SELECT bg.genre_id,
		COUNT(*) AS books_total,
		SUM(CASE WHEN b.status = 'published' THEN 1 ELSE 0 END) AS books_visible
	FROM book_genres bg
	JOIN books b ON b.id = bg.book_id
	GROUP BY bg.genre_id
) s ON s.genre_id = genres.id`

// genreModerationScanRow is the flat scan target of ListGenresForModeration.
type genreModerationScanRow struct {
	ID           uuid.UUID
	Name         string
	Slug         string
	Status       string
	WikidataQID  *string `gorm:"column:wikidata_qid"`
	CreatedAt    time.Time
	BooksTotal   int64
	BooksVisible int64
}

func (r *GormRepository) ListGenresForModeration(ctx context.Context, filter GenreModerationFilter) ([]GenreModerationRow, int64, error) {
	query := r.db.WithContext(ctx).Table("genres").Joins(genreBookStatsJoin).
		Where("genres.merged_into_id IS NULL")

	if filter.Status != "" {
		query = query.Where("genres.status = ?", filter.Status)
	}
	switch filter.QID {
	case "with":
		query = query.Where("genres.wikidata_qid IS NOT NULL")
	case "without":
		query = query.Where("genres.wikidata_qid IS NULL")
	}
	if filter.Search != "" {
		query = query.Where("genres.name ILIKE ?", "%"+filter.Search+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var scanned []genreModerationScanRow
	if err := query.
		Select("genres.id, genres.name, genres.slug, genres.status, genres.wikidata_qid, genres.created_at, " +
			"COALESCE(s.books_total, 0) AS books_total, COALESCE(s.books_visible, 0) AS books_visible").
		Order("genres.created_at DESC, genres.name ASC, genres.id ASC").
		Limit(limit).
		Offset(offset).
		Scan(&scanned).Error; err != nil {
		return nil, 0, err
	}

	rows := make([]GenreModerationRow, 0, len(scanned))
	for _, s := range scanned {
		rows = append(rows, GenreModerationRow{
			Genre: catalogDomain.Genre{
				ID:          s.ID,
				Name:        s.Name,
				Slug:        s.Slug,
				Status:      s.Status,
				WikidataQID: s.WikidataQID,
				CreatedAt:   s.CreatedAt,
			},
			BooksTotal:   s.BooksTotal,
			BooksVisible: s.BooksVisible,
		})
	}
	return rows, total, nil
}

func (r *GormRepository) ListGenreSampleBooks(ctx context.Context, genreIDs []uuid.UUID) (map[uuid.UUID][]SampleBook, error) {
	out := make(map[uuid.UUID][]SampleBook, len(genreIDs))
	if len(genreIDs) == 0 {
		return out, nil
	}

	var rows []struct {
		GenreID uuid.UUID
		ID      uuid.UUID
		Title   string
		Status  string
	}
	if err := r.db.WithContext(ctx).Raw(`
		SELECT genre_id, id, title, status FROM (
			SELECT bg.genre_id, b.id, b.title, b.status,
				ROW_NUMBER() OVER (PARTITION BY bg.genre_id ORDER BY b.title ASC, b.id ASC) AS rn
			FROM book_genres bg
			JOIN books b ON b.id = bg.book_id
			WHERE bg.genre_id IN ?
		) t
		WHERE rn <= ?
		ORDER BY genre_id, title ASC, id ASC
	`, genreIDs, sampleBooksPerAuthor).Scan(&rows).Error; err != nil {
		return nil, err
	}

	for _, row := range rows {
		if len(out[row.GenreID]) >= sampleBooksPerAuthor {
			continue
		}
		out[row.GenreID] = append(out[row.GenreID], SampleBook{ID: row.ID, Title: row.Title, Status: row.Status})
	}
	return out, nil
}

func (r *GormRepository) ListGenreHintCandidates(ctx context.Context) ([]catalogDomain.Genre, error) {
	var genres []catalogDomain.Genre
	if err := r.db.WithContext(ctx).
		Select("id, name, slug, status, wikidata_qid").
		Where("status <> ? AND merged_into_id IS NULL", catalogDomain.GenreStatusRejected).
		Order("name ASC, id ASC").
		Find(&genres).Error; err != nil {
		return nil, err
	}
	return genres, nil
}

func (r *GormRepository) SetGenreStatus(ctx context.Context, id uuid.UUID, status string) (bool, error) {
	// Updates with an explicit column map (not Save): only status is
	// written (genres has no updated_at); name, slug, QID and book_genres
	// stay. Alias rows are not found (their status is pinned to rejected).
	result := r.db.WithContext(ctx).Model(&catalogDomain.Genre{}).
		Where("id = ? AND merged_into_id IS NULL", id).
		Updates(map[string]interface{}{"status": status})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// GenreMergeResult is the outcome of MergeGenres / MergeGenresBatch.
type GenreMergeResult struct {
	BooksReassigned     int64
	SourcesMerged       int
	CollectionsSwitched int
	// AutoCollectionIDs are the switched collections with kind = 'auto'.
	AutoCollectionIDs []uuid.UUID
	// CarriedQIDFrom is the source whose QID moved to the target, if any.
	CarriedQIDFrom *uuid.UUID
	// KeptQIDs maps a source id to the QID it kept on its alias because the
	// target already had one.
	KeptQIDs map[uuid.UUID]string
	// MergeID is the merge journal entry written for this merge.
	MergeID uuid.UUID
}

// GenreMergeCollection is a curated collection a merge would switch.
type GenreMergeCollection struct {
	ID              uuid.UUID
	Title           string
	Slug            string
	Kind            string
	IsPublished     bool
	SourceGenreID   uuid.UUID
	SourceGenreName string
}

// GenreMergePreview is the read-only summary of a (batch) genre merge.
type GenreMergePreview struct {
	BooksReassigned int64
	Collections     []GenreMergeCollection
}

type genreMergeLockRow struct {
	ID          uuid.UUID
	WikidataQID *string `gorm:"column:wikidata_qid"`
}

type switchedCollection struct {
	ID   uuid.UUID
	Kind string
}

// lockGenresForMerge re-reads the non-alias genres among ids (locked FOR
// UPDATE on Postgres, sorted by id so concurrent merges lock in the same
// order). Returns gorm.ErrRecordNotFound unless every id is found.
func lockGenresForMerge(tx *gorm.DB, ids []uuid.UUID) (map[uuid.UUID]*string, error) {
	lock := ""
	if tx.Dialector.Name() == "postgres" {
		lock = " FOR UPDATE"
	}
	var rows []genreMergeLockRow
	if err := tx.Raw("SELECT id, wikidata_qid FROM genres WHERE id IN ? AND merged_into_id IS NULL ORDER BY id"+lock,
		ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	qids := make(map[uuid.UUID]*string, len(rows))
	for i := range rows {
		qids[rows[i].ID] = rows[i].WikidataQID
	}
	if len(qids) != countDistinct(ids) {
		return nil, gorm.ErrRecordNotFound
	}
	return qids, nil
}

func countDistinct(ids []uuid.UUID) int {
	seen := make(map[uuid.UUID]struct{}, len(ids))
	for _, id := range ids {
		seen[id] = struct{}{}
	}
	return len(seen)
}

// MergeGenres implements Repository.MergeGenres as a single transaction
// (genre-moderation-queue.md rule 4), see mergeGenreInTx for the steps.
//
// Both rows are re-read inside the transaction (locked FOR UPDATE on
// Postgres); a missing or alias row returns gorm.ErrRecordNotFound with
// nothing changed.
func (r *GormRepository) MergeGenres(ctx context.Context, sourceID, targetID uuid.UUID, actorID *uuid.UUID) (GenreMergeResult, error) {
	return r.mergeGenres(ctx, []uuid.UUID{sourceID}, targetID, actorID)
}

// MergeGenresBatch implements Repository.MergeGenresBatch: all rows are
// locked up front, then each source is merged in request order with the
// running target QID, so the first source with a QID wins the carry-over.
func (r *GormRepository) MergeGenresBatch(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID, actorID *uuid.UUID) (GenreMergeResult, error) {
	return r.mergeGenres(ctx, sourceIDs, targetID, actorID)
}

func (r *GormRepository) mergeGenres(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID, actorID *uuid.UUID) (GenreMergeResult, error) {
	for _, id := range sourceIDs {
		if id == targetID {
			return GenreMergeResult{}, gorm.ErrRecordNotFound
		}
	}
	var res GenreMergeResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res = GenreMergeResult{}
		ids := append(append([]uuid.UUID{}, sourceIDs...), targetID)
		qids, err := lockGenresForMerge(tx, ids)
		if err != nil {
			return err
		}
		targetQID := qids[targetID]
		var journal []*genreMergeSnapshot
		for i, sourceID := range sourceIDs {
			sourceQID := qids[sourceID]
			snap, err := snapshotGenreForMerge(tx, sourceID, targetID, i)
			if err != nil {
				return err
			}
			books, carried, collections, err := mergeGenreInTx(tx, sourceID, targetID, targetQID, sourceQID)
			if err != nil {
				return err
			}
			res.BooksReassigned += books
			res.SourcesMerged++
			if carried {
				id := sourceID
				res.CarriedQIDFrom = &id
				targetQID = sourceQID
			} else if sourceQID != nil {
				if res.KeptQIDs == nil {
					res.KeptQIDs = map[uuid.UUID]string{}
				}
				res.KeptQIDs[sourceID] = *sourceQID
			}
			res.CollectionsSwitched += len(collections)
			for _, c := range collections {
				if c.Kind == "auto" {
					res.AutoCollectionIDs = append(res.AutoCollectionIDs, c.ID)
				}
			}
			snap.finish(carried, collections)
			journal = append(journal, snap)
		}
		res.MergeID, err = writeGenreMergeJournal(tx, targetID, actorID, res, journal)
		return err
	})
	if err != nil {
		return GenreMergeResult{}, err
	}
	return res, nil
}

// mergeGenreInTx merges one source into the target inside an open
// transaction whose rows are already locked:
//  1. Move book_genres rows the target does not already have to the
//     target (their count is booksReassigned).
//  2. Delete the remaining source rows (books already linked to both).
//  3. Re-point the source's existing aliases to the target, so alias
//     chains stay one hop deep.
//  4. When the source has a QID and the target has none, clear it on the
//     source, then set it on the target (this order keeps the unique
//     wikidata_qid index happy). Otherwise the source keeps its QID and
//     the alias resolves it.
//  5. Re-point curated collections linked to the source to the target.
//  6. Turn the source into an alias: merged_into_id = target, status =
//     rejected. Name, slug and any remaining QID stay.
func mergeGenreInTx(tx *gorm.DB, sourceID, targetID uuid.UUID, targetQID, sourceQID *string) (int64, bool, []switchedCollection, error) {
	result := tx.Exec(`
		UPDATE book_genres
		SET genre_id = ?
		WHERE genre_id = ?
		AND book_id NOT IN (SELECT book_id FROM book_genres WHERE genre_id = ?)
	`, targetID, sourceID, targetID)
	if result.Error != nil {
		return 0, false, nil, result.Error
	}
	booksReassigned := result.RowsAffected

	if err := tx.Exec("DELETE FROM book_genres WHERE genre_id = ?", sourceID).Error; err != nil {
		return 0, false, nil, err
	}

	if err := tx.Exec("UPDATE genres SET merged_into_id = ? WHERE merged_into_id = ?", targetID, sourceID).Error; err != nil {
		return 0, false, nil, err
	}

	carried := false
	if sourceQID != nil && targetQID == nil {
		if err := tx.Exec("UPDATE genres SET wikidata_qid = NULL WHERE id = ?", sourceID).Error; err != nil {
			return 0, false, nil, err
		}
		if err := tx.Exec("UPDATE genres SET wikidata_qid = ? WHERE id = ?", *sourceQID, targetID).Error; err != nil {
			return 0, false, nil, err
		}
		carried = true
	}

	var collections []switchedCollection
	if err := tx.Raw("SELECT id, kind FROM curated_collections WHERE genre_id = ?", sourceID).Scan(&collections).Error; err != nil {
		return 0, false, nil, err
	}
	if len(collections) > 0 {
		if err := tx.Exec("UPDATE curated_collections SET genre_id = ?, updated_at = ? WHERE genre_id = ?",
			targetID, time.Now().UTC(), sourceID).Error; err != nil {
			return 0, false, nil, err
		}
	}

	if err := tx.Exec("UPDATE genres SET merged_into_id = ?, status = ? WHERE id = ?",
		targetID, catalogDomain.GenreStatusRejected, sourceID).Error; err != nil {
		return 0, false, nil, err
	}
	return booksReassigned, carried, collections, nil
}

// PreviewGenreMerge implements Repository.PreviewGenreMerge; it only reads.
func (r *GormRepository) PreviewGenreMerge(ctx context.Context, sourceIDs []uuid.UUID, targetID uuid.UUID) (GenreMergePreview, error) {
	preview := GenreMergePreview{Collections: []GenreMergeCollection{}}
	for _, id := range sourceIDs {
		if id == targetID {
			return GenreMergePreview{}, gorm.ErrRecordNotFound
		}
	}
	db := r.db.WithContext(ctx)

	ids := append(append([]uuid.UUID{}, sourceIDs...), targetID)
	var found []uuid.UUID
	if err := db.Raw("SELECT id FROM genres WHERE id IN ? AND merged_into_id IS NULL", ids).Scan(&found).Error; err != nil {
		return GenreMergePreview{}, err
	}
	if len(found) != countDistinct(ids) {
		return GenreMergePreview{}, gorm.ErrRecordNotFound
	}

	if err := db.Raw(`
		SELECT COUNT(DISTINCT book_id) FROM book_genres
		WHERE genre_id IN ?
		AND book_id NOT IN (SELECT book_id FROM book_genres WHERE genre_id = ?)
	`, sourceIDs, targetID).Scan(&preview.BooksReassigned).Error; err != nil {
		return GenreMergePreview{}, err
	}

	var rows []struct {
		ID          uuid.UUID
		Title       string
		Slug        string
		Kind        string
		IsPublished bool
		GenreID     uuid.UUID
		GenreName   string
	}
	if err := db.Raw(`
		SELECT c.id, c.title, c.slug, c.kind, c.is_published, g.id AS genre_id, g.name AS genre_name
		FROM curated_collections c
		JOIN genres g ON g.id = c.genre_id
		WHERE c.genre_id IN ?
		ORDER BY c.title, c.id
	`, sourceIDs).Scan(&rows).Error; err != nil {
		return GenreMergePreview{}, err
	}
	for _, row := range rows {
		preview.Collections = append(preview.Collections, GenreMergeCollection{
			ID: row.ID, Title: row.Title, Slug: row.Slug, Kind: row.Kind,
			IsPublished: row.IsPublished, SourceGenreID: row.GenreID, SourceGenreName: row.GenreName,
		})
	}
	return preview, nil
}

// GenreStateRow is the current state of one genre (alias rows included), read
// by the AI genre suggestions to decide staleness and to show current values.
type GenreStateRow struct {
	ID           uuid.UUID
	Name         string
	Status       string
	MergedIntoID *uuid.UUID
	WikidataQID  *string
	BooksTotal   int64
}

type genreStateScanRow struct {
	ID           uuid.UUID
	Name         string
	Status       string
	MergedIntoID *uuid.UUID `gorm:"column:merged_into_id"`
	WikidataQID  *string    `gorm:"column:wikidata_qid"`
	BooksTotal   int64
}

// ListGenreAnalysisCandidates returns every non-alias genre whose status is
// pending or published with its linked-books count (the AI genre analysis
// pool), ordered by name then id. Rejected genres and aliases never appear.
func (r *GormRepository) ListGenreAnalysisCandidates(ctx context.Context) ([]GenreModerationRow, error) {
	var scanned []genreModerationScanRow
	if err := r.db.WithContext(ctx).Table("genres").Joins(genreBookStatsJoin).
		Where("genres.merged_into_id IS NULL AND genres.status IN ?",
			[]string{catalogDomain.GenreStatusPending, catalogDomain.GenreStatusPublished}).
		Select("genres.id, genres.name, genres.slug, genres.status, genres.wikidata_qid, genres.created_at, " +
			"COALESCE(s.books_total, 0) AS books_total, COALESCE(s.books_visible, 0) AS books_visible").
		Order("genres.name ASC, genres.id ASC").
		Scan(&scanned).Error; err != nil {
		return nil, err
	}
	rows := make([]GenreModerationRow, 0, len(scanned))
	for _, s := range scanned {
		rows = append(rows, GenreModerationRow{
			Genre: catalogDomain.Genre{
				ID: s.ID, Name: s.Name, Slug: s.Slug, Status: s.Status,
				WikidataQID: s.WikidataQID, CreatedAt: s.CreatedAt,
			},
			BooksTotal:   s.BooksTotal,
			BooksVisible: s.BooksVisible,
		})
	}
	return rows, nil
}

// GenresByIDs returns the current state of the genres among ids, aliases and
// rejected ones included. Ids that do not exist are simply absent.
func (r *GormRepository) GenresByIDs(ctx context.Context, ids []uuid.UUID) ([]GenreStateRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var scanned []genreStateScanRow
	if err := r.db.WithContext(ctx).Table("genres").Joins(genreBookStatsJoin).
		Where("genres.id IN ?", ids).
		Select("genres.id, genres.name, genres.status, genres.merged_into_id, genres.wikidata_qid, " +
			"COALESCE(s.books_total, 0) AS books_total").
		Scan(&scanned).Error; err != nil {
		return nil, err
	}
	rows := make([]GenreStateRow, 0, len(scanned))
	for _, s := range scanned {
		rows = append(rows, GenreStateRow(s))
	}
	return rows, nil
}
