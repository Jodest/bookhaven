package repository

import (
	"context"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// genreStatusDDL recreates the tables the admin genre and dashboard queries
// touch, with SQLite-native types (docs/features/genre-status.md).
const genreStatusDDL = `
CREATE TABLE genres (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	slug TEXT NOT NULL UNIQUE,
	status TEXT NOT NULL DEFAULT 'pending',
	wikidata_qid TEXT,
	created_at DATETIME
);
CREATE UNIQUE INDEX idx_genres_wikidata_qid ON genres(wikidata_qid) WHERE wikidata_qid IS NOT NULL;
CREATE TABLE books (id TEXT PRIMARY KEY, is_hidden BOOLEAN NOT NULL DEFAULT 0, external_source TEXT, deleted_at DATETIME);
CREATE TABLE authors (id TEXT PRIMARY KEY, status TEXT NOT NULL DEFAULT 'published');
CREATE TABLE reviews (id TEXT PRIMARY KEY, is_hidden BOOLEAN NOT NULL DEFAULT 0, deleted_at DATETIME);
`

func newGenreStatusRepo(t *testing.T) (*GormRepository, *gorm.DB) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.Exec(genreStatusDDL).Error; err != nil {
		t.Fatalf("create schema: %v", err)
	}
	return &GormRepository{db: db}, db
}

func TestGenreRepo_CreatePersistsStatusAndQID(t *testing.T) {
	repo, db := newGenreStatusRepo(t)
	ctx := context.Background()
	qid := "Q8261"
	g := &catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished, WikidataQID: &qid}
	if err := repo.CreateGenre(ctx, g); err != nil {
		t.Fatalf("create: %v", err)
	}

	// The explicit column tag must map to wikidata_qid (not wikidata_q_id).
	var raw struct {
		Status      string
		WikidataQID *string `gorm:"column:wikidata_qid"`
	}
	if err := db.Raw("SELECT status, wikidata_qid FROM genres WHERE id = ?", g.ID).Scan(&raw).Error; err != nil {
		t.Fatalf("raw select: %v", err)
	}
	if raw.Status != catalogDomain.GenreStatusPublished || raw.WikidataQID == nil || *raw.WikidataQID != qid {
		t.Fatalf("unexpected persisted row: %+v", raw)
	}

	got, err := repo.GetGenreByQID(ctx, qid)
	if err != nil || got.ID != g.ID {
		t.Fatalf("GetGenreByQID: %+v err=%v", got, err)
	}
	if _, err := repo.GetGenreByQID(ctx, "Q1"); err != gorm.ErrRecordNotFound {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}

func TestGenreRepo_SetGenreQIDKeepsStatus(t *testing.T) {
	repo, _ := newGenreStatusRepo(t)
	ctx := context.Background()
	g := &catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusRejected}
	if err := repo.CreateGenre(ctx, g); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := repo.SetGenreQID(ctx, g.ID, "Q8261"); err != nil {
		t.Fatalf("set qid: %v", err)
	}
	got, err := repo.GetGenreByID(ctx, g.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != catalogDomain.GenreStatusRejected || got.WikidataQID == nil || *got.WikidataQID != "Q8261" {
		t.Fatalf("unexpected genre after backfill: %+v", got)
	}

	other := &catalogDomain.Genre{ID: uuid.New(), Name: "повесть", Slug: "povest", Status: catalogDomain.GenreStatusPending}
	if err := repo.CreateGenre(ctx, other); err != nil {
		t.Fatalf("create other: %v", err)
	}
	if err := repo.SetGenreQID(ctx, other.ID, "Q8261"); err == nil {
		t.Fatal("expected unique violation on a taken QID")
	}
}

func TestGenreRepo_ListGenresStatusFilter(t *testing.T) {
	repo, _ := newGenreStatusRepo(t)
	ctx := context.Background()
	for _, g := range []catalogDomain.Genre{
		{ID: uuid.New(), Name: "а", Slug: "a", Status: catalogDomain.GenreStatusPending},
		{ID: uuid.New(), Name: "б", Slug: "b", Status: catalogDomain.GenreStatusPublished},
		{ID: uuid.New(), Name: "в", Slug: "v", Status: catalogDomain.GenreStatusRejected},
	} {
		g := g
		if err := repo.CreateGenre(ctx, &g); err != nil {
			t.Fatalf("create: %v", err)
		}
	}

	all, total, err := repo.ListGenres(ctx, "", "", 20, 0)
	if err != nil || total != 3 || len(all) != 3 {
		t.Fatalf("expected all 3, got %d/%d err=%v", len(all), total, err)
	}
	pending, total, err := repo.ListGenres(ctx, "", catalogDomain.GenreStatusPending, 20, 0)
	if err != nil || total != 1 || len(pending) != 1 || pending[0].Slug != "a" {
		t.Fatalf("expected only pending, got %+v total=%d err=%v", pending, total, err)
	}
}

func TestGenreRepo_DashboardCountsPendingGenres(t *testing.T) {
	repo, db := newGenreStatusRepo(t)
	ctx := context.Background()
	for _, st := range []string{"pending", "pending", "published", "rejected"} {
		if err := db.Exec("INSERT INTO genres (id, name, slug, status) VALUES (?, ?, ?, ?)", uuid.New(), "g", uuid.NewString(), st).Error; err != nil {
			t.Fatalf("insert: %v", err)
		}
	}
	stats, err := repo.DashboardCounts(ctx)
	if err != nil {
		t.Fatalf("dashboard: %v", err)
	}
	if stats.PendingGenres != 2 || stats.Genres != 4 {
		t.Fatalf("expected pending_genres=2 genres=4, got %+v", stats)
	}
}
