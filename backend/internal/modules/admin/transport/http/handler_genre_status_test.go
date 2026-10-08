package http

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
)

// Admin genre status + Wikidata QID (docs/features/genre-status.md).

func TestListGenres_IncludesStatusAndQID(t *testing.T) {
	h, svc := setupHandler()
	qid := "Q8261"
	svc.genres = []catalogDomain.Genre{
		{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPending, WikidataQID: &qid},
		{ID: uuid.New(), Name: "повесть", Slug: "povest", Status: catalogDomain.GenreStatusPublished},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/genres?status=pending", nil)
	setAdminUser(c)
	if err := h.ListGenres(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if svc.lastGenreStatusFilter != catalogDomain.GenreStatusPending {
		t.Fatalf("expected status filter forwarded, got %q", svc.lastGenreStatusFilter)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"status":"pending"`) || !strings.Contains(body, `"wikidata_qid":"Q8261"`) {
		t.Fatalf("expected status and qid in payload, got %s", body)
	}
	// wikidata_qid is not omitempty: a genre without one serializes null.
	if !strings.Contains(body, `"wikidata_qid":null`) {
		t.Fatalf("expected wikidata_qid null for a genre without QID, got %s", body)
	}
}

func TestListGenres_InvalidStatus_BadRequest(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/admin/genres?status=bogus", nil)
	setAdminUser(c)
	if err := h.ListGenres(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestCreateGenre_PublishedWithQID(t *testing.T) {
	h, svc := setupHandler()
	c, rec := newContext(http.MethodPost, "/api/v1/admin/genres", []byte(`{"name":"роман","status":"rejected","wikidata_qid":"Q8261"}`))
	setAdminUser(c)
	if err := h.CreateGenre(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", rec.Code)
	}
	if svc.lastGenreInput == nil || svc.lastGenreInput.Status != nil {
		t.Fatalf("create must not forward status, got %+v", svc.lastGenreInput)
	}
	var body genreResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != catalogDomain.GenreStatusPublished || body.WikidataQID == nil || *body.WikidataQID != "Q8261" {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestGenreMutations_ErrorMapping(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"invalid status", adminService.ErrInvalidGenreStatus, http.StatusBadRequest},
		{"invalid qid", adminService.ErrInvalidQID, http.StatusBadRequest},
		{"duplicate qid", adminService.ErrDuplicateGenreQID, http.StatusConflict},
		{"duplicate slug", adminService.ErrDuplicateSlug, http.StatusConflict},
	}
	for _, tc := range cases {
		t.Run("patch "+tc.name, func(t *testing.T) {
			h, svc := setupHandler()
			id := uuid.New()
			svc.genres = []catalogDomain.Genre{{ID: id, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPending}}
			svc.genreErr = tc.err

			c, rec := newContext(http.MethodPatch, "/api/v1/admin/genres/"+id.String(), []byte(`{"name":"роман","status":"published","wikidata_qid":"Q1"}`))
			c.SetParamNames("id")
			c.SetParamValues(id.String())
			setAdminUser(c)
			if err := h.UpdateGenre(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, rec.Code)
			}
		})
	}
	for _, tc := range cases[1:] {
		t.Run("post "+tc.name, func(t *testing.T) {
			h, svc := setupHandler()
			svc.genreErr = tc.err
			c, rec := newContext(http.MethodPost, "/api/v1/admin/genres", []byte(`{"name":"роман","wikidata_qid":"Q1"}`))
			setAdminUser(c)
			if err := h.CreateGenre(c); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if rec.Code != tc.want {
				t.Fatalf("expected %d, got %d", tc.want, rec.Code)
			}
		})
	}
}

func TestUpdateGenre_ForwardsStatusAndQID(t *testing.T) {
	h, svc := setupHandler()
	id := uuid.New()
	svc.genres = []catalogDomain.Genre{{ID: id, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPending}}

	c, rec := newContext(http.MethodPatch, "/api/v1/admin/genres/"+id.String(), []byte(`{"name":"роман","status":"published","wikidata_qid":""}`))
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	setAdminUser(c)
	if err := h.UpdateGenre(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	in := svc.lastGenreInput
	if in == nil || in.Status == nil || *in.Status != "published" || in.WikidataQID == nil || *in.WikidataQID != "" {
		t.Fatalf("expected status and empty qid (clear) forwarded, got %+v", in)
	}

	// Omitted fields stay nil (unchanged).
	c, _ = newContext(http.MethodPatch, "/api/v1/admin/genres/"+id.String(), []byte(`{"name":"роман","wikidata_qid":null}`))
	c.SetParamNames("id")
	c.SetParamValues(id.String())
	setAdminUser(c)
	if err := h.UpdateGenre(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if svc.lastGenreInput.Status != nil || svc.lastGenreInput.WikidataQID != nil {
		t.Fatalf("expected omitted/null fields to stay nil, got %+v", svc.lastGenreInput)
	}
}

func TestAdminBookResponse_GenresIncludeStatus(t *testing.T) {
	qid := "Q8261"
	b := catalogDomain.Book{ID: uuid.New(), Title: "Книга", Genres: []catalogDomain.Genre{
		{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusRejected, WikidataQID: &qid},
	}}
	raw, err := json.Marshal(toBookResponse(b))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(raw), `"status":"rejected"`) || !strings.Contains(string(raw), `"wikidata_qid":"Q8261"`) {
		t.Fatalf("expected admin book genres with status and qid, got %s", raw)
	}
}
