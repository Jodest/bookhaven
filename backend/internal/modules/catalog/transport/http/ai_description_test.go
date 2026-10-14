package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/catalog/service"
	"github.com/google/uuid"
)

// fakeAIReader mimics the aidescriptions repository: only approved
// records are returned (a draft reads as none).
type fakeAIReader struct {
	approved map[uuid.UUID]string
	err      error
	calls    int
}

var aiGeneratedAt = time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)

func (f *fakeAIReader) ApprovedAIDescription(ctx context.Context, bookID uuid.UUID) (string, time.Time, bool, error) {
	f.calls++
	if f.err != nil {
		return "", time.Time{}, false, f.err
	}
	text, ok := f.approved[bookID]
	return text, aiGeneratedAt, ok, nil
}

func getBookBody(t *testing.T, reader AIDescriptionReader, b domain.Book) (int, map[string]json.RawMessage) {
	t.Helper()
	repo := newFakeRepository()
	repo.books = []domain.Book{b}
	h := NewHandler(service.NewService(repo), WithAIDescriptionReader(reader))
	c, rec := newContext(http.MethodGet, "/api/v1/books/"+b.Slug)
	c.SetPath("/api/v1/books/:slug")
	c.SetParamNames("slug")
	c.SetParamValues(b.Slug)
	if err := h.GetBook(c); err != nil {
		t.Fatalf("GetBook: %v", err)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v (%s)", err, rec.Body.String())
	}
	return rec.Code, body
}

func publishedBook(description *string) domain.Book {
	return domain.Book{ID: uuid.New(), Title: "Записки юного врача", Slug: "zapiski", Status: domain.BookStatusPublished, Description: description}
}

func TestGetBook_AIDescriptionFallback(t *testing.T) {
	const text = "Повесть рассказывает о молодом враче в уездной больнице."
	blank := "   "
	own := "Описание из Open Library."

	// Approved, own description blank → text.
	b := publishedBook(&blank)
	reader := &fakeAIReader{approved: map[uuid.UUID]string{b.ID: text}}
	code, body := getBookBody(t, reader, b)
	var ai struct {
		Text        string    `json:"text"`
		GeneratedAt time.Time `json:"generated_at"`
	}
	if err := json.Unmarshal(body["ai_description"], &ai); err != nil || code != 200 || ai.Text != text || !ai.GeneratedAt.Equal(aiGeneratedAt) {
		t.Fatalf("fallback: %d %s (%v)", code, body["ai_description"], err)
	}

	// Own description → null, the reader is not even consulted.
	b = publishedBook(&own)
	reader = &fakeAIReader{approved: map[uuid.UUID]string{b.ID: text}}
	if _, body := getBookBody(t, reader, b); string(body["ai_description"]) != "null" || reader.calls != 0 {
		t.Errorf("own description: ai=%s calls=%d", body["ai_description"], reader.calls)
	}

	// Only a draft (the reader returns nothing) → null.
	b = publishedBook(nil)
	if _, body := getBookBody(t, &fakeAIReader{}, b); string(body["ai_description"]) != "null" {
		t.Errorf("draft: ai=%s", body["ai_description"])
	}

	// Reader error → null, the page still answers 200.
	if code, body := getBookBody(t, &fakeAIReader{err: errors.New("db down")}, b); code != 200 || string(body["ai_description"]) != "null" {
		t.Errorf("reader error: %d ai=%s", code, body["ai_description"])
	}
}

func TestGetBook_AIDescriptionNullWithoutReader(t *testing.T) {
	h, repo := setupHandler()
	repo.books = []domain.Book{publishedBook(nil)}
	c, rec := newContext(http.MethodGet, "/api/v1/books/zapiski")
	c.SetPath("/api/v1/books/:slug")
	c.SetParamNames("slug")
	c.SetParamValues("zapiski")
	if err := h.GetBook(c); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rec.Body.String(), `"ai_description":null`) {
		t.Errorf("ai_description must always be present: %s", rec.Body.String())
	}
}
