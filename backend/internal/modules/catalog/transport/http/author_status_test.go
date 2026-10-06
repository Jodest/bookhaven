package http

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	domain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Non-published authors are embedded by name only: slug null, no photo or
// birth date (docs/features/author-status.md rule 7).
func TestToAuthorResponse_NonPublishedIsNameOnly(t *testing.T) {
	photo := "https://example.com/p.jpg"
	birth := time.Date(1828, 9, 9, 0, 0, 0, 0, time.UTC)
	for _, status := range []string{domain.AuthorStatusPending, domain.AuthorStatusRejected} {
		a := domain.Author{ID: uuid.New(), Name: "Лев Толстой", Slug: "lev-tolstoy", Status: status, PhotoURL: &photo, BirthDate: &birth}
		raw, err := json.Marshal(toAuthorResponse(a))
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		got := string(raw)
		if !strings.Contains(got, `"slug":null`) || strings.Contains(got, "photo_url") || strings.Contains(got, "birth_date") {
			t.Fatalf("%s author: expected name-only payload, got %s", status, got)
		}
		if !strings.Contains(got, `"name":"Лев Толстой"`) || !strings.Contains(got, a.ID.String()) {
			t.Fatalf("%s author: expected id and name kept, got %s", status, got)
		}
	}
}

func TestToAuthorResponse_PublishedKeepsSlug(t *testing.T) {
	photo := "https://example.com/p.jpg"
	a := domain.Author{ID: uuid.New(), Name: "Лев Толстой", Slug: "lev-tolstoy", Status: domain.AuthorStatusPublished, PhotoURL: &photo}
	raw, err := json.Marshal(toAuthorResponse(a))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"slug":"lev-tolstoy"`) || !strings.Contains(string(raw), "photo_url") {
		t.Fatalf("expected full published payload, got %s", raw)
	}
}
