package http

import (
	"encoding/json"
	"strings"
	"testing"

	domain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Only published genres appear in public book payloads
// (docs/features/genre-status.md rule 7).
func mixedStatusBook() domain.Book {
	return domain.Book{
		ID:    uuid.New(),
		Title: "Книга",
		Genres: []domain.Genre{
			{ID: uuid.New(), Name: "pending-genre", Slug: "pending-genre", Status: domain.GenreStatusPending},
			{ID: uuid.New(), Name: "published-genre", Slug: "published-genre", Status: domain.GenreStatusPublished},
			{ID: uuid.New(), Name: "rejected-genre", Slug: "rejected-genre", Status: domain.GenreStatusRejected},
		},
	}
}

func assertOnlyPublishedGenre(t *testing.T, name string, v interface{}) {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("%s: marshal: %v", name, err)
	}
	got := string(raw)
	if !strings.Contains(got, "published-genre") || strings.Contains(got, "pending-genre") || strings.Contains(got, "rejected-genre") {
		t.Fatalf("%s: expected only the published genre, got %s", name, got)
	}
	if strings.Contains(got, "\"status\"") {
		t.Fatalf("%s: public genre payload must not expose status, got %s", name, got)
	}
}

func TestBookMappers_OnlyPublishedGenres(t *testing.T) {
	b := mixedStatusBook()
	assertOnlyPublishedGenre(t, "list item", ToBookListItemResponse(b))
	assertOnlyPublishedGenre(t, "detail", toBookResponse(b))
	assertOnlyPublishedGenre(t, "author page books", toAuthorDetailResponse(domain.Author{
		ID: uuid.New(), Name: "Автор", Slug: "avtor", Status: domain.AuthorStatusPublished, Books: []domain.Book{b},
	}))
}
