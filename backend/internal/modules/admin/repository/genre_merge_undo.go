package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Block codes of GenreMergeUndoBlockedError, in check order.
const (
	GenreMergeUndoAlreadyUndone  = "already_undone"
	GenreMergeUndoTargetMissing  = "target_missing"
	GenreMergeUndoTargetMerged   = "target_merged"
	GenreMergeUndoSourceMissing  = "source_missing"
	GenreMergeUndoSourceNotAlias = "source_not_alias"
	GenreMergeUndoQIDConflict    = "qid_conflict"
)

// GenreMergeUndoBlockedError is returned, with nothing changed, when a merge
// cannot be undone safely.
type GenreMergeUndoBlockedError struct{ Code string }

func (e *GenreMergeUndoBlockedError) Error() string {
	return "genre merge cannot be undone: " + e.Code
}

// GenreMergeUserRef names a user in the merge history.
type GenreMergeUserRef struct {
	ID   uuid.UUID
	Name string
}

// GenreMergeGenreRef names a genre as recorded at merge time.
type GenreMergeGenreRef struct {
	ID   uuid.UUID
	Name string
}

// GenreMergeEntry is one merge journal entry.
type GenreMergeEntry struct {
	ID                  uuid.UUID
	CreatedAt           time.Time
	PerformedBy         *GenreMergeUserRef
	Target              GenreMergeGenreRef
	Sources             []GenreMergeGenreRef
	BooksReassigned     int
	CollectionsSwitched int
	UndoneAt            *time.Time
	UndoneBy            *GenreMergeUserRef
}

// GenreMergeUndoPreview is the read-only summary of an undo.
type GenreMergeUndoPreview struct {
	Merge         GenreMergeEntry
	BooksReturned int64
	BooksRelinked int64
	// Collections are the recorded collections still linked to the target;
	// SourceGenre* is the genre each one returns to.
	Collections []GenreMergeCollection
}

// GenreMergeUndoResult is the outcome of UndoGenreMerge.
type GenreMergeUndoResult struct {
	TargetID                uuid.UUID
	SourcesRestored         int
	BooksReturned           int64
	BooksRelinked           int64
	CollectionsSwitchedBack int
	// AutoCollectionIDs are the switched-back collections with kind = 'auto'.
	AutoCollectionIDs []uuid.UUID
}

type genreMergeHeaderRow struct {
	ID                  uuid.UUID
	TargetID            uuid.UUID
	TargetName          string
	BooksReassigned     int
	CollectionsSwitched int
	CreatedAt           time.Time
	UndoneAt            *time.Time
	PerformedBy         *uuid.UUID
	PerformedByName     *string
	UndoneBy            *uuid.UUID
	UndoneByName        *string
}

const genreMergeHeaderSelect = `
	SELECT m.id, m.target_id, m.target_name, m.books_reassigned, m.collections_switched,
		m.created_at, m.undone_at,
		m.performed_by, COALESCE(NULLIF(pb.name, ''), pb.email) AS performed_by_name,
		m.undone_by, COALESCE(NULLIF(ub.name, ''), ub.email) AS undone_by_name
	FROM genre_merges m
	LEFT JOIN users pb ON pb.id = m.performed_by
	LEFT JOIN users ub ON ub.id = m.undone_by`

func userRef(id *uuid.UUID, name *string) *GenreMergeUserRef {
	if id == nil {
		return nil
	}
	ref := &GenreMergeUserRef{ID: *id}
	if name != nil {
		ref.Name = *name
	}
	return ref
}

func (h genreMergeHeaderRow) entry(sources []genreMergeSourceRecord) GenreMergeEntry {
	e := GenreMergeEntry{
		ID:                  h.ID,
		CreatedAt:           h.CreatedAt,
		PerformedBy:         userRef(h.PerformedBy, h.PerformedByName),
		Target:              GenreMergeGenreRef{ID: h.TargetID, Name: h.TargetName},
		Sources:             make([]GenreMergeGenreRef, 0, len(sources)),
		BooksReassigned:     h.BooksReassigned,
		CollectionsSwitched: h.CollectionsSwitched,
		UndoneAt:            h.UndoneAt,
		UndoneBy:            userRef(h.UndoneBy, h.UndoneByName),
	}
	for _, s := range sources {
		e.Sources = append(e.Sources, GenreMergeGenreRef{ID: s.SourceID, Name: s.SourceName})
	}
	return e
}

// ListGenreMerges implements Repository.ListGenreMerges: newest first.
func (r *GormRepository) ListGenreMerges(ctx context.Context, page, limit int) ([]GenreMergeEntry, int64, error) {
	db := r.db.WithContext(ctx)
	var total int64
	if err := db.Raw("SELECT COUNT(*) FROM genre_merges").Scan(&total).Error; err != nil {
		return nil, 0, err
	}
	var headers []genreMergeHeaderRow
	if err := db.Raw(genreMergeHeaderSelect+" ORDER BY m.created_at DESC, m.id DESC LIMIT ? OFFSET ?",
		limit, (page-1)*limit).Scan(&headers).Error; err != nil {
		return nil, 0, err
	}
	entries := make([]GenreMergeEntry, 0, len(headers))
	if len(headers) == 0 {
		return entries, total, nil
	}
	ids := make([]uuid.UUID, 0, len(headers))
	for _, h := range headers {
		ids = append(ids, h.ID)
	}
	var sources []genreMergeSourceRecord
	if err := db.Where("merge_id IN ?", ids).Order("merge_id, position").Find(&sources).Error; err != nil {
		return nil, 0, err
	}
	byMerge := make(map[uuid.UUID][]genreMergeSourceRecord, len(headers))
	for _, s := range sources {
		byMerge[s.MergeID] = append(byMerge[s.MergeID], s)
	}
	for _, h := range headers {
		entries = append(entries, h.entry(byMerge[h.ID]))
	}
	return entries, total, nil
}

// loadGenreMerge reads the entry and its sources; gorm.ErrRecordNotFound for
// an unknown id. With lock, the header row is locked first (Postgres; the
// lock cannot share a query with the outer joins of the header select).
func loadGenreMerge(db *gorm.DB, id uuid.UUID, lock bool) (genreMergeHeaderRow, []genreMergeSourceRecord, error) {
	if lock {
		suffix := ""
		if db.Dialector.Name() == "postgres" {
			suffix = " FOR UPDATE"
		}
		var locked []uuid.UUID
		if err := db.Raw("SELECT id FROM genre_merges WHERE id = ?"+suffix, id).Scan(&locked).Error; err != nil {
			return genreMergeHeaderRow{}, nil, err
		}
		if len(locked) == 0 {
			return genreMergeHeaderRow{}, nil, gorm.ErrRecordNotFound
		}
	}
	var headers []genreMergeHeaderRow
	if err := db.Raw(genreMergeHeaderSelect+" WHERE m.id = ?", id).Scan(&headers).Error; err != nil {
		return genreMergeHeaderRow{}, nil, err
	}
	if len(headers) == 0 {
		return genreMergeHeaderRow{}, nil, gorm.ErrRecordNotFound
	}
	var sources []genreMergeSourceRecord
	if err := db.Where("merge_id = ?", id).Order("position").Find(&sources).Error; err != nil {
		return genreMergeHeaderRow{}, nil, err
	}
	return headers[0], sources, nil
}

type genreUndoRow struct {
	ID           uuid.UUID
	MergedIntoID *uuid.UUID `gorm:"column:merged_into_id"`
	WikidataQID  *string    `gorm:"column:wikidata_qid"`
}

// checkGenreMergeUndo runs the shared checks (first failure wins) and
// returns the current rows of the target and sources. With lock the genre
// rows are locked FOR UPDATE (Postgres), sorted by id.
func checkGenreMergeUndo(db *gorm.DB, h genreMergeHeaderRow, sources []genreMergeSourceRecord, lock bool) (map[uuid.UUID]genreUndoRow, error) {
	if h.UndoneAt != nil {
		return nil, &GenreMergeUndoBlockedError{Code: GenreMergeUndoAlreadyUndone}
	}
	ids := []uuid.UUID{h.TargetID}
	for _, s := range sources {
		ids = append(ids, s.SourceID)
	}
	suffix := ""
	if lock && db.Dialector.Name() == "postgres" {
		suffix = " FOR UPDATE"
	}
	var rows []genreUndoRow
	if err := db.Raw("SELECT id, merged_into_id, wikidata_qid FROM genres WHERE id IN ? ORDER BY id"+suffix, ids).Scan(&rows).Error; err != nil {
		return nil, err
	}
	genres := make(map[uuid.UUID]genreUndoRow, len(rows))
	for _, row := range rows {
		genres[row.ID] = row
	}
	target, ok := genres[h.TargetID]
	if !ok {
		return nil, &GenreMergeUndoBlockedError{Code: GenreMergeUndoTargetMissing}
	}
	if target.MergedIntoID != nil {
		return nil, &GenreMergeUndoBlockedError{Code: GenreMergeUndoTargetMerged}
	}
	for _, s := range sources {
		if _, ok := genres[s.SourceID]; !ok {
			return nil, &GenreMergeUndoBlockedError{Code: GenreMergeUndoSourceMissing}
		}
	}
	for _, s := range sources {
		if g := genres[s.SourceID]; g.MergedIntoID == nil || *g.MergedIntoID != h.TargetID {
			return nil, &GenreMergeUndoBlockedError{Code: GenreMergeUndoSourceNotAlias}
		}
	}
	for _, s := range sources {
		if s.PrevQID == nil {
			continue
		}
		var holders []uuid.UUID
		if err := db.Raw("SELECT id FROM genres WHERE wikidata_qid = ?", *s.PrevQID).Scan(&holders).Error; err != nil {
			return nil, err
		}
		for _, holder := range holders {
			if holder == s.SourceID || (s.QIDCarried && holder == h.TargetID) {
				continue
			}
			return nil, &GenreMergeUndoBlockedError{Code: GenreMergeUndoQIDConflict}
		}
	}
	return genres, nil
}

// SQL fragments shared by the preview counts and the undo. They are set
// based (no id lists in parameters), so a huge genre cannot hit parameter
// limits.
const (
	// undoMovedCandidates: moved books still linked to the target
	// (args: targetID, mergeID, sourceID).
	undoMovedCandidates = `
		FROM genre_merge_items i
		JOIN book_genres bt ON bt.book_id = i.ref_id AND bt.genre_id = ?
		WHERE i.merge_id = ? AND i.source_id = ? AND i.kind = 'book_moved'`
	// undoLinkedCandidates: already-linked books that still exist and lost
	// their source link (args: mergeID, sourceID, sourceID).
	undoLinkedCandidates = `
		FROM genre_merge_items i
		JOIN books b ON b.id = i.ref_id
		WHERE i.merge_id = ? AND i.source_id = ? AND i.kind = 'book_linked'
		AND NOT EXISTS (SELECT 1 FROM book_genres x WHERE x.book_id = i.ref_id AND x.genre_id = ?)`
)

// PreviewGenreMergeUndo implements Repository.PreviewGenreMergeUndo; it only reads.
func (r *GormRepository) PreviewGenreMergeUndo(ctx context.Context, mergeID uuid.UUID) (GenreMergeUndoPreview, error) {
	db := r.db.WithContext(ctx)
	h, sources, err := loadGenreMerge(db, mergeID, false)
	if err != nil {
		return GenreMergeUndoPreview{}, err
	}
	if _, err := checkGenreMergeUndo(db, h, sources, false); err != nil {
		return GenreMergeUndoPreview{}, err
	}
	preview := GenreMergeUndoPreview{Merge: h.entry(sources), Collections: []GenreMergeCollection{}}
	for _, s := range sources {
		var n int64
		if err := db.Raw("SELECT COUNT(*) "+undoMovedCandidates, h.TargetID, mergeID, s.SourceID).Scan(&n).Error; err != nil {
			return GenreMergeUndoPreview{}, err
		}
		preview.BooksReturned += n
		if err := db.Raw("SELECT COUNT(*) "+undoLinkedCandidates, mergeID, s.SourceID, s.SourceID).Scan(&n).Error; err != nil {
			return GenreMergeUndoPreview{}, err
		}
		preview.BooksRelinked += n
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
		FROM genre_merge_items i
		JOIN curated_collections c ON c.id = i.ref_id
		JOIN genres g ON g.id = i.source_id
		WHERE i.merge_id = ? AND i.kind = 'collection' AND c.genre_id = ?
		ORDER BY c.title, c.id
	`, mergeID, h.TargetID).Scan(&rows).Error; err != nil {
		return GenreMergeUndoPreview{}, err
	}
	for _, row := range rows {
		preview.Collections = append(preview.Collections, GenreMergeCollection{
			ID: row.ID, Title: row.Title, Slug: row.Slug, Kind: row.Kind,
			IsPublished: row.IsPublished, SourceGenreID: row.GenreID, SourceGenreName: row.GenreName,
		})
	}
	return preview, nil
}

// UndoGenreMerge implements Repository.UndoGenreMerge: the whole entry (all
// sources of a batch) is restored in one transaction, skipping whatever no
// longer matches the snapshot.
func (r *GormRepository) UndoGenreMerge(ctx context.Context, mergeID uuid.UUID, actorID uuid.UUID) (GenreMergeUndoResult, error) {
	var res GenreMergeUndoResult
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res = GenreMergeUndoResult{}
		h, sources, err := loadGenreMerge(tx, mergeID, true)
		if err != nil {
			return err
		}
		genres, err := checkGenreMergeUndo(tx, h, sources, true)
		if err != nil {
			return err
		}
		res.TargetID = h.TargetID
		for i := len(sources) - 1; i >= 0; i-- {
			if err := undoGenreMergeSource(tx, mergeID, h.TargetID, sources[i], genres, &res); err != nil {
				return err
			}
		}
		res.SourcesRestored = len(sources)
		return tx.Exec("UPDATE genre_merges SET undone_at = ?, undone_by = ? WHERE id = ?",
			time.Now().UTC(), actorID, mergeID).Error
	})
	if err != nil {
		return GenreMergeUndoResult{}, err
	}
	return res, nil
}

func undoGenreMergeSource(tx *gorm.DB, mergeID, targetID uuid.UUID, s genreMergeSourceRecord, genres map[uuid.UUID]genreUndoRow, res *GenreMergeUndoResult) error {
	sourceID := s.SourceID

	// QID: take it back from the target if it carried it, then give it to
	// the source unless the source already holds it.
	if s.PrevQID != nil {
		if s.QIDCarried && genres[targetID].WikidataQID != nil && *genres[targetID].WikidataQID == *s.PrevQID {
			if err := tx.Exec("UPDATE genres SET wikidata_qid = NULL WHERE id = ? AND wikidata_qid = ?",
				targetID, *s.PrevQID).Error; err != nil {
				return err
			}
		}
		if g := genres[sourceID]; g.WikidataQID == nil || *g.WikidataQID != *s.PrevQID {
			if err := tx.Exec("UPDATE genres SET wikidata_qid = ? WHERE id = ?", *s.PrevQID, sourceID).Error; err != nil {
				return err
			}
		}
	}

	if err := tx.Exec("UPDATE genres SET merged_into_id = NULL, status = ? WHERE id = ?", s.PrevStatus, sourceID).Error; err != nil {
		return err
	}

	// Aliases that still point at the target go back to the source.
	if err := tx.Exec(`
		UPDATE genres SET merged_into_id = ?
		WHERE merged_into_id = ?
		AND id IN (SELECT ref_id FROM genre_merge_items WHERE merge_id = ? AND source_id = ? AND kind = 'alias')
	`, sourceID, targetID, mergeID, sourceID).Error; err != nil {
		return err
	}

	// Moved books still linked to the target return to the source. The
	// target link stays when a later, not undone merge into the same target
	// recorded the book (its dedup relied on that link).
	var returned int64
	if err := tx.Raw("SELECT COUNT(*) "+undoMovedCandidates, targetID, mergeID, sourceID).Scan(&returned).Error; err != nil {
		return err
	}
	if err := tx.Exec(`
		INSERT INTO book_genres (book_id, genre_id)
		SELECT i.ref_id, ? `+undoMovedCandidates+`
		AND NOT EXISTS (SELECT 1 FROM book_genres x WHERE x.book_id = i.ref_id AND x.genre_id = ?)
	`, sourceID, targetID, mergeID, sourceID, sourceID).Error; err != nil {
		return err
	}
	if err := tx.Exec(`
		DELETE FROM book_genres
		WHERE genre_id = ?
		AND book_id IN (SELECT ref_id FROM genre_merge_items WHERE merge_id = ? AND source_id = ? AND kind = 'book_moved')
		AND book_id NOT IN (
			SELECT i2.ref_id FROM genre_merge_items i2
			JOIN genre_merges m2 ON m2.id = i2.merge_id
			WHERE m2.target_id = ? AND m2.undone_at IS NULL AND m2.id <> ?
			AND m2.created_at > (SELECT created_at FROM genre_merges WHERE id = ?)
			AND i2.kind IN ('book_moved', 'book_linked'))
	`, targetID, mergeID, sourceID, targetID, mergeID, mergeID).Error; err != nil {
		return err
	}
	res.BooksReturned += returned

	// Books that were already in the target get their source link back.
	var relinked int64
	if err := tx.Raw("SELECT COUNT(*) "+undoLinkedCandidates, mergeID, sourceID, sourceID).Scan(&relinked).Error; err != nil {
		return err
	}
	if err := tx.Exec("INSERT INTO book_genres (book_id, genre_id) SELECT i.ref_id, ? "+undoLinkedCandidates,
		sourceID, mergeID, sourceID, sourceID).Error; err != nil {
		return err
	}
	res.BooksRelinked += relinked

	// Collections still linked to the target go back to the source.
	var collections []switchedCollection
	if err := tx.Raw(`
		SELECT c.id, c.kind FROM curated_collections c
		WHERE c.genre_id = ?
		AND c.id IN (SELECT ref_id FROM genre_merge_items WHERE merge_id = ? AND source_id = ? AND kind = 'collection')
	`, targetID, mergeID, sourceID).Scan(&collections).Error; err != nil {
		return err
	}
	if len(collections) > 0 {
		if err := tx.Exec(`
			UPDATE curated_collections SET genre_id = ?, updated_at = ?
			WHERE genre_id = ?
			AND id IN (SELECT ref_id FROM genre_merge_items WHERE merge_id = ? AND source_id = ? AND kind = 'collection')
		`, sourceID, time.Now().UTC(), targetID, mergeID, sourceID).Error; err != nil {
			return err
		}
	}
	res.CollectionsSwitchedBack += len(collections)
	for _, c := range collections {
		if c.Kind == "auto" {
			res.AutoCollectionIDs = append(res.AutoCollectionIDs, c.ID)
		}
	}
	return nil
}
