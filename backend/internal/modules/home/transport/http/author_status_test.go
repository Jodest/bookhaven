package http

import (
	"encoding/json"
	"testing"

	catalogdomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

func TestToReviewBookAuthorResponse_NonPublishedHasNullSlug(t *testing.T) {
	pending := catalogdomain.Author{ID: uuid.New(), Name: "Аноним", Slug: "anonim", Status: catalogdomain.AuthorStatusPending}
	published := catalogdomain.Author{ID: uuid.New(), Name: "Лев Толстой", Slug: "lev-tolstoy", Status: catalogdomain.AuthorStatusPublished}

	raw, err := json.Marshal([]reviewBookAuthorResponse{toReviewBookAuthorResponse(pending), toReviewBookAuthorResponse(published)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `[{"name":"Аноним","slug":null},{"name":"Лев Толстой","slug":"lev-tolstoy"}]`
	if string(raw) != want {
		t.Fatalf("got %s, want %s", raw, want)
	}
}
