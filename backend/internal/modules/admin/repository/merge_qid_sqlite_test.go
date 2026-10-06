package repository

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// mergeQIDDDL recreates the authors/book_authors shape (incl. the
// (external_source, external_id) partial unique index from migration
// 000015) with SQLite-native types so GormRepository.MergeAuthors' real SQL
// can be exercised. The production schema is Postgres-only.
const mergeQIDDDL = `
CREATE TABLE authors (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	status TEXT NOT NULL DEFAULT 'published',
	external_source TEXT,
	external_id TEXT
);
CREATE UNIQUE INDEX idx_authors_external_source_id ON authors(external_source, external_id) WHERE external_source IS NOT NULL;
CREATE TABLE book_authors (book_id TEXT NOT NULL, author_id TEXT NOT NULL, PRIMARY KEY (book_id, author_id));
`

type authorIdentityRow struct {
	Status         string
	ExternalSource *string
	ExternalID     *string
}

func newMergeQIDRepo(t *testing.T) (*GormRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(mergeQIDDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &GormRepository{db: db}, db
}

func insertAuthor(t *testing.T, db *gorm.DB, id uuid.UUID, slug, status string, qid *string) {
	t.Helper()
	var source *string
	if qid != nil {
		s := "wikidata"
		source = &s
	}
	if err := db.Exec("INSERT INTO authors (id, name, slug, status, external_source, external_id) VALUES (?, ?, ?, ?, ?, ?)",
		id, slug, slug, status, source, qid).Error; err != nil {
		t.Fatalf("insert author: %v", err)
	}
}

func loadIdentity(t *testing.T, db *gorm.DB, id uuid.UUID) authorIdentityRow {
	t.Helper()
	var row authorIdentityRow
	if err := db.Raw("SELECT status, external_source, external_id FROM authors WHERE id = ?", id).Scan(&row).Error; err != nil {
		t.Fatalf("load author: %v", err)
	}
	return row
}

func TestMergeAuthors_CarriesQIDToCanonicalWithoutOne(t *testing.T) {
	repo, db := newMergeQIDRepo(t)
	dup, canon := uuid.New(), uuid.New()
	qid := "Q7243"
	insertAuthor(t, db, dup, "dup", "pending", &qid)
	insertAuthor(t, db, canon, "canon", "published", nil)

	if _, err := repo.MergeAuthors(context.Background(), dup, canon); err != nil {
		t.Fatalf("MergeAuthors: %v", err)
	}
	got := loadIdentity(t, db, canon)
	if got.ExternalSource == nil || *got.ExternalSource != "wikidata" || got.ExternalID == nil || *got.ExternalID != "Q7243" {
		t.Fatalf("expected canonical to receive wikidata/Q7243, got %+v", got)
	}
	if got.Status != "published" {
		t.Fatalf("canonical status must be unchanged, got %q", got.Status)
	}
}

func TestMergeAuthors_CanonicalKeepsOwnQID(t *testing.T) {
	repo, db := newMergeQIDRepo(t)
	dup, canon := uuid.New(), uuid.New()
	dupQID, canonQID := "Q1", "Q2"
	insertAuthor(t, db, dup, "dup", "published", &dupQID)
	insertAuthor(t, db, canon, "canon", "published", &canonQID)

	if _, err := repo.MergeAuthors(context.Background(), dup, canon); err != nil {
		t.Fatalf("MergeAuthors: %v", err)
	}
	got := loadIdentity(t, db, canon)
	if got.ExternalID == nil || *got.ExternalID != "Q2" {
		t.Fatalf("expected canonical to keep Q2, got %+v", got)
	}
}
