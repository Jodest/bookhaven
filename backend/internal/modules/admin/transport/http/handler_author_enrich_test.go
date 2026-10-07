package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func TestEnrichAuthorsFromWikidata_OK(t *testing.T) {
	h, svc := setupHandler()
	id := uuid.New()
	svc.enrichReport = &adminService.AuthorEnrichmentReport{
		Processed: 1, Enriched: 1,
		Items: []adminService.AuthorEnrichmentItem{{
			AuthorID: id.String(), Name: "Лев Толстой", Outcome: "enriched",
			Filled: []string{"birth_date", "photo"}, Notes: []string{"death_date_unusable"},
		}},
	}

	body := []byte(fmt.Sprintf(`{"ids":[%q]}`, id))
	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/enrich-wikidata", body)
	setAdminUser(c)
	if err := h.EnrichAuthorsFromWikidata(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(svc.lastEnrichIDs) != 1 || svc.lastEnrichIDs[0] != id {
		t.Fatalf("ids passed = %v", svc.lastEnrichIDs)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	items, _ := got["items"].([]interface{})
	if got["processed"] != float64(1) || got["enriched"] != float64(1) || len(items) != 1 {
		t.Fatalf("unexpected body: %v", got)
	}
	item := items[0].(map[string]interface{})
	for _, key := range []string{"author_id", "name", "outcome", "filled", "notes", "message"} {
		if _, ok := item[key]; !ok {
			t.Fatalf("item missing %q: %v", key, item)
		}
	}
}

func TestEnrichAuthorsFromWikidata_ErrorMapping(t *testing.T) {
	valid := fmt.Sprintf(`{"ids":[%q]}`, uuid.New())
	cases := []struct {
		name string
		body string
		err  error
		want int
	}{
		{"empty ids", `{"ids":[]}`, nil, http.StatusBadRequest},
		{"invalid uuid", `{"ids":["nope"]}`, nil, http.StatusBadRequest},
		{"malformed", `{`, nil, http.StatusBadRequest},
		{"too many", valid, adminService.ErrAuthorEnrichmentTooLarge, http.StatusBadRequest},
		{"not configured", valid, adminService.ErrAuthorEnrichmentUnavailable, http.StatusServiceUnavailable},
		{"rate limited", valid, bookdata.ErrRateLimited, http.StatusBadGateway},
		{"unavailable", valid, fmt.Errorf("wrapped: %w", bookdata.ErrSourceUnavailable), http.StatusBadGateway},
		{"other", valid, errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		svc.enrichErr = tc.err
		c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/enrich-wikidata", []byte(tc.body))
		setAdminUser(c)
		if err := h.EnrichAuthorsFromWikidata(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if rec.Code != tc.want {
			t.Fatalf("%s: expected %d, got %d: %s", tc.name, tc.want, rec.Code, rec.Body.String())
		}
	}
}

func TestEnrichAuthorsFromWikidataRoute_Auth(t *testing.T) {
	adminRole := middleware.RequireRole(usersDomain.RoleAdmin)
	requireBearer := middleware.RequireBearerAuth()

	cases := []struct {
		name   string
		user   *usersDomain.User
		bearer bool
		want   int
	}{
		{"unauthenticated", nil, false, http.StatusUnauthorized},
		{"moderator", &usersDomain.User{Role: usersDomain.RoleModerator}, true, http.StatusForbidden},
		{"admin without bearer", &usersDomain.User{Role: usersDomain.RoleAdmin}, false, http.StatusUnauthorized},
		{"admin with bearer", &usersDomain.User{Role: usersDomain.RoleAdmin}, true, http.StatusOK},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		svc.enrichReport = &adminService.AuthorEnrichmentReport{Items: []adminService.AuthorEnrichmentItem{}}
		var next echo.HandlerFunc = adminRole(requireBearer(h.EnrichAuthorsFromWikidata))

		c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/enrich-wikidata", []byte(fmt.Sprintf(`{"ids":[%q]}`, uuid.New())))
		if tc.user != nil {
			c.Set("user", tc.user)
		}
		if tc.bearer {
			c.Request().Header.Set("Authorization", "Bearer token")
		}
		if err := next(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if rec.Code != tc.want {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.want, rec.Code)
		}
	}
}

func TestImportWikidataAuthor_EnrichmentInResponse(t *testing.T) {
	h, svc := setupHandler()
	author := importedAuthor("pending", "Q7243")
	photo, license, source := "https://upload.wikimedia.org/wikipedia/commons/thumb/a/ab/T.jpg/500px-T.jpg", "Public domain", "https://commons.wikimedia.org/wiki/File:T.jpg"
	author.PhotoURL, author.PhotoLicense, author.PhotoSourceURL = &photo, &license, &source
	svc.importResult = &adminService.AuthorImportResult{
		Outcome:    adminService.AuthorOutcomeCreated,
		Author:     author,
		Enrichment: &adminService.AuthorEnrichmentSummary{Filled: []string{"photo"}, Notes: []string{}},
	}

	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/import-wikidata", []byte(`{"qid":"Q7243"}`))
	setAdminUser(c)
	if err := h.ImportWikidataAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	enrichment, _ := got["enrichment"].(map[string]interface{})
	if filled, _ := enrichment["filled"].([]interface{}); len(filled) != 1 || filled[0] != "photo" || enrichment["error"] != "" {
		t.Fatalf("unexpected enrichment: %v", got["enrichment"])
	}
	attribution, _ := got["author"].(map[string]interface{})["photo_attribution"].(map[string]interface{})
	if attribution["license"] != "Public domain" || attribution["source_url"] != source || attribution["author"] != nil || attribution["license_url"] != nil {
		t.Fatalf("unexpected photo_attribution: %v", attribution)
	}

	// existing → enrichment null, manual photo → photo_attribution null.
	h, svc = setupHandler()
	manual := importedAuthor("published", "Q7243")
	manualPhoto := "https://example.com/p.jpg"
	manual.PhotoURL = &manualPhoto
	svc.importResult = &adminService.AuthorImportResult{Outcome: adminService.AuthorOutcomeExisting, Match: adminService.AuthorMatchQID, Author: manual}
	c, rec = newContext(http.MethodPost, "/api/v1/admin/authors/import-wikidata", []byte(`{"qid":"Q7243"}`))
	setAdminUser(c)
	if err := h.ImportWikidataAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got = map[string]interface{}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if v, ok := got["enrichment"]; !ok || v != nil {
		t.Fatalf("expected enrichment:null, got %v", got)
	}
	if v, ok := got["author"].(map[string]interface{})["photo_attribution"]; !ok || v != nil {
		t.Fatalf("expected photo_attribution:null, got %v", got["author"])
	}
}
