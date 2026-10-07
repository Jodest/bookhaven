package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

type enrichRow struct {
	BirthDate       *time.Time
	DeathDate       *time.Time
	PhotoURL        *string
	PhotoAuthor     *string
	PhotoLicense    *string
	PhotoLicenseURL *string
	PhotoSourceURL  *string
}

func loadEnrichRow(t *testing.T, f *modFixture, id uuid.UUID) enrichRow {
	t.Helper()
	var row enrichRow
	if err := f.db.Raw("SELECT birth_date, death_date, photo_url, photo_author, photo_license, photo_license_url, photo_source_url FROM authors WHERE id = ?", id).
		Scan(&row).Error; err != nil {
		t.Fatalf("load: %v", err)
	}
	return row
}

func testPhotoFill() *PhotoFill {
	return &PhotoFill{
		URL:       "https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/X.jpg/500px-X.jpg",
		Author:    strPtr("Sergei Prokudin-Gorskii"),
		License:   "Public domain",
		SourceURL: "https://commons.wikimedia.org/wiki/File:X.jpg",
	}
}

func TestFillAuthorEnrichment_FillsEmptyColumns(t *testing.T) {
	f := newModerationRepo(t)
	ctx := context.Background()
	id := f.author("Лев Толстой", "pending", strPtr("Q7243"))
	birth := time.Date(1828, 9, 9, 0, 0, 0, 0, time.UTC)
	death := time.Date(1910, 11, 20, 0, 0, 0, 0, time.UTC)

	if err := f.repo.FillAuthorEnrichment(ctx, id, AuthorEnrichmentFill{BirthDate: &birth, DeathDate: &death, Photo: testPhotoFill()}); err != nil {
		t.Fatalf("fill: %v", err)
	}
	row := loadEnrichRow(t, f, id)
	if row.BirthDate == nil || !row.BirthDate.Equal(birth) || row.DeathDate == nil || !row.DeathDate.Equal(death) {
		t.Fatalf("dates not filled: %+v", row)
	}
	if row.PhotoURL == nil || *row.PhotoURL != testPhotoFill().URL || row.PhotoAuthor == nil || *row.PhotoAuthor != "Sergei Prokudin-Gorskii" ||
		row.PhotoLicense == nil || *row.PhotoLicense != "Public domain" || row.PhotoLicenseURL != nil ||
		row.PhotoSourceURL == nil || *row.PhotoSourceURL != "https://commons.wikimedia.org/wiki/File:X.jpg" {
		t.Fatalf("photo not filled: %+v", row)
	}
}

func TestFillAuthorEnrichment_NeverOverwrites(t *testing.T) {
	f := newModerationRepo(t)
	ctx := context.Background()
	id := f.author("Автор", "published", strPtr("Q1"))
	manualBirth := time.Date(1800, 1, 2, 0, 0, 0, 0, time.UTC)
	// A manual photo (and date) set between the service's read and the
	// fill update must win.
	if err := f.db.Exec("UPDATE authors SET birth_date = ?, photo_url = ? WHERE id = ?", manualBirth, "https://example.com/manual.jpg", id).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	birth := time.Date(1828, 9, 9, 0, 0, 0, 0, time.UTC)
	death := time.Date(1910, 11, 20, 0, 0, 0, 0, time.UTC)

	if err := f.repo.FillAuthorEnrichment(ctx, id, AuthorEnrichmentFill{BirthDate: &birth, DeathDate: &death, Photo: testPhotoFill()}); err != nil {
		t.Fatalf("fill: %v", err)
	}
	row := loadEnrichRow(t, f, id)
	if row.BirthDate == nil || !row.BirthDate.Equal(manualBirth) {
		t.Fatalf("birth date overwritten: %v", row.BirthDate)
	}
	if row.DeathDate == nil || !row.DeathDate.Equal(death) {
		t.Fatalf("empty death date not filled: %v", row.DeathDate)
	}
	if row.PhotoURL == nil || *row.PhotoURL != "https://example.com/manual.jpg" {
		t.Fatalf("manual photo overwritten: %v", row.PhotoURL)
	}
	if row.PhotoAuthor != nil || row.PhotoLicense != nil || row.PhotoSourceURL != nil {
		t.Fatalf("attribution must not be written without the photo: %+v", row)
	}
}

func TestFillAuthorEnrichment_EmptyStringPhotoIsFilled(t *testing.T) {
	f := newModerationRepo(t)
	id := f.author("Автор", "pending", strPtr("Q1"))
	if err := f.db.Exec("UPDATE authors SET photo_url = '' WHERE id = ?", id).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	if err := f.repo.FillAuthorEnrichment(context.Background(), id, AuthorEnrichmentFill{Photo: testPhotoFill()}); err != nil {
		t.Fatalf("fill: %v", err)
	}
	row := loadEnrichRow(t, f, id)
	if row.PhotoURL == nil || *row.PhotoURL != testPhotoFill().URL || row.BirthDate != nil {
		t.Fatalf("unexpected row: %+v", row)
	}
}

func TestGetAuthorsByIDs(t *testing.T) {
	f := newModerationRepo(t)
	a := f.author("А", "pending", nil)
	b := f.author("Б", "rejected", strPtr("Q2"))
	f.author("В", "published", nil)

	got, err := f.repo.GetAuthorsByIDs(context.Background(), []uuid.UUID{a, b, uuid.New()})
	if err != nil {
		t.Fatalf("GetAuthorsByIDs: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d authors, want 2", len(got))
	}
	empty, err := f.repo.GetAuthorsByIDs(context.Background(), nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("empty: %v %v", empty, err)
	}
}
