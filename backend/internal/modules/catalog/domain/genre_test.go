package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestPublishedGenres(t *testing.T) {
	pub := uuid.New()
	got := PublishedGenres([]Genre{
		{ID: uuid.New(), Status: GenreStatusPending},
		{ID: pub, Status: GenreStatusPublished},
		{ID: uuid.New(), Status: GenreStatusRejected},
		{ID: uuid.New()},
	})
	if len(got) != 1 || got[0].ID != pub {
		t.Fatalf("expected only the published genre, got %+v", got)
	}
	if empty := PublishedGenres(nil); empty == nil || len(empty) != 0 {
		t.Fatalf("expected non-nil empty slice, got %#v", empty)
	}
}

func TestIsValidGenreStatus(t *testing.T) {
	for _, s := range []string{GenreStatusPending, GenreStatusPublished, GenreStatusRejected} {
		if !IsValidGenreStatus(s) {
			t.Fatalf("%q must be valid", s)
		}
	}
	for _, s := range []string{"", "PUBLISHED", "hidden"} {
		if IsValidGenreStatus(s) {
			t.Fatalf("%q must be invalid", s)
		}
	}
	if !(Genre{Status: GenreStatusPublished}).IsPublished() || (Genre{Status: GenreStatusPending}).IsPublished() {
		t.Fatal("IsPublished mismatch")
	}
}
