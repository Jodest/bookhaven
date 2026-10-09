package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func newGetByIDContext(path, id string) (echo.Context, *httptest.ResponseRecorder) {
	c, rec := newContext(http.MethodGet, path+id, nil)
	c.SetPath(path + ":id")
	c.SetParamNames("id")
	c.SetParamValues(id)
	setAdminUser(c)
	return c, rec
}

func strPtr(s string) *string { return &s }

func TestGetAuthor(t *testing.T) {
	const path = "/api/v1/admin/authors/"

	for _, status := range []string{
		catalogDomain.AuthorStatusPublished,
		catalogDomain.AuthorStatusPending,
		catalogDomain.AuthorStatusRejected,
	} {
		t.Run("200 "+status, func(t *testing.T) {
			h, svc := setupHandler()
			id := uuid.New()
			svc.authors = []catalogDomain.Author{{
				ID:             id,
				Name:           "Лев Толстой",
				Slug:           "lev-tolstoy",
				Status:         status,
				ExternalSource: strPtr(catalogDomain.AuthorExternalSourceWikidata),
				ExternalID:     strPtr("Q7243"),
			}}
			c, rec := newGetByIDContext(path, id.String())

			if err := h.GetAuthor(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			var body authorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}
			if body.ID != id.String() || body.Status != status {
				t.Fatalf("unexpected body: %+v", body)
			}
			if body.ExternalSource == nil || *body.ExternalSource != "wikidata" || body.ExternalID == nil || *body.ExternalID != "Q7243" {
				t.Fatalf("expected external identity, got %+v", body)
			}
		})
	}

	t.Run("400 invalid id, service not called", func(t *testing.T) {
		h, svc := setupHandler()
		c, rec := newGetByIDContext(path, "not-a-uuid")

		if err := h.GetAuthor(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		assertErrorBody(t, rec, "invalid author id")
		if svc.getAuthorCalled {
			t.Fatal("service must not be called for an invalid id")
		}
	})

	t.Run("404 not found", func(t *testing.T) {
		h, _ := setupHandler()
		c, rec := newGetByIDContext(path, uuid.New().String())

		if err := h.GetAuthor(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		assertErrorBody(t, rec, "author not found")
	})

	t.Run("500 other error", func(t *testing.T) {
		h, svc := setupHandler()
		svc.getAuthorErr = errors.New("db down")
		c, rec := newGetByIDContext(path, uuid.New().String())

		if err := h.GetAuthor(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		assertErrorBody(t, rec, "failed to get author")
	})
}

func TestGetGenre(t *testing.T) {
	const path = "/api/v1/admin/genres/"

	for _, status := range []string{
		catalogDomain.GenreStatusPublished,
		catalogDomain.GenreStatusPending,
		catalogDomain.GenreStatusRejected,
	} {
		t.Run("200 "+status, func(t *testing.T) {
			h, svc := setupHandler()
			id := uuid.New()
			svc.genres = []catalogDomain.Genre{{
				ID:          id,
				Name:        "роман",
				Slug:        "roman",
				Status:      status,
				WikidataQID: strPtr("Q8261"),
			}}
			c, rec := newGetByIDContext(path, id.String())

			if err := h.GetGenre(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
			}
			var body genreResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("failed to decode: %v", err)
			}
			if body.ID != id.String() || body.Status != status {
				t.Fatalf("unexpected body: %+v", body)
			}
			if body.WikidataQID == nil || *body.WikidataQID != "Q8261" {
				t.Fatalf("expected wikidata_qid Q8261, got %+v", body.WikidataQID)
			}
		})
	}

	t.Run("400 invalid id, service not called", func(t *testing.T) {
		h, svc := setupHandler()
		c, rec := newGetByIDContext(path, "not-a-uuid")

		if err := h.GetGenre(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
		assertErrorBody(t, rec, "invalid genre id")
		if svc.getGenreCalled {
			t.Fatal("service must not be called for an invalid id")
		}
	})

	t.Run("404 not found", func(t *testing.T) {
		h, svc := setupHandler()
		svc.getGenreErr = adminService.ErrGenreNotFound
		c, rec := newGetByIDContext(path, uuid.New().String())

		if err := h.GetGenre(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", rec.Code)
		}
		assertErrorBody(t, rec, "genre not found")
	})

	t.Run("500 other error", func(t *testing.T) {
		h, svc := setupHandler()
		svc.getGenreErr = errors.New("db down")
		c, rec := newGetByIDContext(path, uuid.New().String())

		if err := h.GetGenre(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", rec.Code)
		}
		assertErrorBody(t, rec, "failed to get genre")
	})
}

func assertErrorBody(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode error body: %v", err)
	}
	if body["error"] != want {
		t.Fatalf("expected error %q, got %q", want, body["error"])
	}
}
