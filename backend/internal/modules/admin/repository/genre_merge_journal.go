package repository

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Genre merge journal (migration 000033). The journal is a snapshot: its
// ids are plain UUIDs without foreign keys to genres/books/collections, so
// deleting any of them never fails because of the journal.

const (
	genreMergeItemBookMoved  = "book_moved"
	genreMergeItemBookLinked = "book_linked"
	genreMergeItemAlias      = "alias"
	genreMergeItemCollection = "collection"

	// genreMergeItemBatchSize keeps the parameters of one INSERT far below
	// the SQLite/Postgres limits.
	genreMergeItemBatchSize = 500
)

type genreMergeRecord struct {
	ID                  uuid.UUID `gorm:"primaryKey"`
	TargetID            uuid.UUID
	TargetName          string
	BooksReassigned     int
	CollectionsSwitched int
	PerformedBy         *uuid.UUID
	CreatedAt           time.Time
}

func (genreMergeRecord) TableName() string { return "genre_merges" }

type genreMergeSourceRecord struct {
	MergeID    uuid.UUID `gorm:"primaryKey"`
	SourceID   uuid.UUID `gorm:"primaryKey"`
	Position   int
	SourceName string
	PrevStatus string
	PrevQID    *string `gorm:"column:prev_qid"`
	QIDCarried bool    `gorm:"column:qid_carried"`
}

func (genreMergeSourceRecord) TableName() string { return "genre_merge_sources" }

type genreMergeItemRecord struct {
	MergeID  uuid.UUID `gorm:"primaryKey"`
	SourceID uuid.UUID `gorm:"primaryKey"`
	Kind     string    `gorm:"primaryKey"`
	RefID    uuid.UUID `gorm:"primaryKey"`
}

func (genreMergeItemRecord) TableName() string { return "genre_merge_items" }

// genreMergeSnapshot is what one source looked like right before its
// mergeGenreInTx, plus the outcome of that call.
type genreMergeSnapshot struct {
	record      genreMergeSourceRecord
	moved       []uuid.UUID
	linked      []uuid.UUID
	aliases     []uuid.UUID
	collections []uuid.UUID
}

// snapshotGenreForMerge reads the source inside the merge transaction before
// it is merged. Because sources are merged one after another, a book shared
// by two sources is "moved" for the first and "linked" for the second.
func snapshotGenreForMerge(tx *gorm.DB, sourceID, targetID uuid.UUID, position int) (*genreMergeSnapshot, error) {
	snap := &genreMergeSnapshot{record: genreMergeSourceRecord{SourceID: sourceID, Position: position}}
	var row struct {
		Name        string
		Status      string
		WikidataQID *string `gorm:"column:wikidata_qid"`
	}
	if err := tx.Raw("SELECT name, status, wikidata_qid FROM genres WHERE id = ?", sourceID).Scan(&row).Error; err != nil {
		return nil, err
	}
	snap.record.SourceName = row.Name
	snap.record.PrevStatus = row.Status
	snap.record.PrevQID = row.WikidataQID

	if err := tx.Raw(`SELECT book_id FROM book_genres WHERE genre_id = ?
		AND book_id NOT IN (SELECT book_id FROM book_genres WHERE genre_id = ?)`,
		sourceID, targetID).Scan(&snap.moved).Error; err != nil {
		return nil, err
	}
	if err := tx.Raw(`SELECT book_id FROM book_genres WHERE genre_id = ?
		AND book_id IN (SELECT book_id FROM book_genres WHERE genre_id = ?)`,
		sourceID, targetID).Scan(&snap.linked).Error; err != nil {
		return nil, err
	}
	if err := tx.Raw("SELECT id FROM genres WHERE merged_into_id = ?", sourceID).Scan(&snap.aliases).Error; err != nil {
		return nil, err
	}
	return snap, nil
}

// finish records the outcome of mergeGenreInTx for the snapshot.
func (s *genreMergeSnapshot) finish(carried bool, collections []switchedCollection) {
	s.record.QIDCarried = carried
	for _, c := range collections {
		s.collections = append(s.collections, c.ID)
	}
}

// writeGenreMergeJournal stores one journal entry for the whole (batch)
// merge and returns its id. It runs in the merge transaction.
func writeGenreMergeJournal(tx *gorm.DB, targetID uuid.UUID, actorID *uuid.UUID, res GenreMergeResult, snaps []*genreMergeSnapshot) (uuid.UUID, error) {
	var targetName string
	if err := tx.Raw("SELECT name FROM genres WHERE id = ?", targetID).Scan(&targetName).Error; err != nil {
		return uuid.Nil, err
	}
	header := genreMergeRecord{
		ID:                  uuid.New(),
		TargetID:            targetID,
		TargetName:          targetName,
		BooksReassigned:     int(res.BooksReassigned),
		CollectionsSwitched: res.CollectionsSwitched,
		PerformedBy:         actorID,
		CreatedAt:           time.Now().UTC(),
	}
	if err := tx.Create(&header).Error; err != nil {
		return uuid.Nil, err
	}
	sources := make([]genreMergeSourceRecord, 0, len(snaps))
	var items []genreMergeItemRecord
	for _, s := range snaps {
		s.record.MergeID = header.ID
		sources = append(sources, s.record)
		for kind, ids := range map[string][]uuid.UUID{
			genreMergeItemBookMoved:  s.moved,
			genreMergeItemBookLinked: s.linked,
			genreMergeItemAlias:      s.aliases,
			genreMergeItemCollection: s.collections,
		} {
			for _, id := range ids {
				items = append(items, genreMergeItemRecord{MergeID: header.ID, SourceID: s.record.SourceID, Kind: kind, RefID: id})
			}
		}
	}
	if err := tx.Create(&sources).Error; err != nil {
		return uuid.Nil, err
	}
	if len(items) > 0 {
		if err := tx.CreateInBatches(&items, genreMergeItemBatchSize).Error; err != nil {
			return uuid.Nil, err
		}
	}
	return header.ID, nil
}
