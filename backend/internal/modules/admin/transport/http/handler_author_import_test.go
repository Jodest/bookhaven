package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func importedAuthor(status, qid string) *catalogDomain.Author {
	src := catalogDomain.AuthorExternalSourceWikidata
	return &catalogDomain.Author{ID: uuid.New(), Name: "Лев Толстой", Slug: "lev-tolstoy", Status: status, ExternalSource: &src, ExternalID: &qid}
}

func TestImportWikidataAuthor_Created(t *testing.T) {
	h, svc := setupHandler()
	namesake := importedAuthor("published", "Q100")
	svc.importResult = &adminService.AuthorImportResult{
		Outcome:    adminService.AuthorOutcomeCreated,
		Author:     importedAuthor("pending", "Q7200"),
		NamesakeOf: namesake,
	}

	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/import-wikidata", []byte(`{"qid":"Q7200"}`))
	setAdminUser(c)
	if err := h.ImportWikidataAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastImportQID != "Q7200" {
		t.Fatalf("qid passed = %q", svc.lastImportQID)
	}

	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["outcome"] != "created" || got["qid_backfilled"] != false {
		t.Fatalf("unexpected body: %v", got)
	}
	if v, ok := got["match"]; !ok || v != nil {
		t.Fatalf("expected match:null, got %v", got)
	}
	author, _ := got["author"].(map[string]interface{})
	if author["status"] != "pending" || author["external_id"] != "Q7200" || author["external_source"] != "wikidata" {
		t.Fatalf("unexpected author: %v", author)
	}
	ns, _ := got["namesake_of"].(map[string]interface{})
	if ns["id"] != namesake.ID.String() {
		t.Fatalf("unexpected namesake_of: %v", got["namesake_of"])
	}
}

func TestImportWikidataAuthor_ExistingAndRejected_OK(t *testing.T) {
	cases := []struct {
		outcome, match string
		backfilled     bool
	}{
		{adminService.AuthorOutcomeExisting, adminService.AuthorMatchQID, false},
		{adminService.AuthorOutcomeExisting, adminService.AuthorMatchSlug, true},
		{adminService.AuthorOutcomeRejected, adminService.AuthorMatchQID, false},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		svc.importResult = &adminService.AuthorImportResult{Outcome: tc.outcome, Match: tc.match, QIDBackfilled: tc.backfilled, Author: importedAuthor("published", "Q7200")}

		c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/import-wikidata", []byte(`{"qid":"Q7200"}`))
		setAdminUser(c)
		if err := h.ImportWikidataAuthor(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("%s/%s: expected 200, got %d", tc.outcome, tc.match, rec.Code)
		}
		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got["outcome"] != tc.outcome || got["match"] != tc.match || got["qid_backfilled"] != tc.backfilled {
			t.Fatalf("unexpected body: %v", got)
		}
		if v, ok := got["namesake_of"]; !ok || v != nil {
			t.Fatalf("expected namesake_of:null, got %v", got)
		}
	}
}

func TestImportWikidataAuthor_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
		code string
	}{
		{adminService.ErrInvalidQID, http.StatusBadRequest, ""},
		{adminService.ErrWikidataEntityNotFound, http.StatusNotFound, ""},
		{adminService.ErrNoRussianLabel, http.StatusUnprocessableEntity, "no_ru_label"},
		{adminService.ErrNotAPerson, http.StatusUnprocessableEntity, "not_a_person"},
		{fmt.Errorf("create author: %w", adminService.ErrDuplicateSlug), http.StatusConflict, ""},
		{bookdata.ErrRateLimited, http.StatusBadGateway, ""},
		{fmt.Errorf("%w: status 503", bookdata.ErrSourceUnavailable), http.StatusBadGateway, ""},
		{adminService.ErrAuthorImportUnavailable, http.StatusServiceUnavailable, ""},
		{errors.New("boom"), http.StatusInternalServerError, ""},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		svc.importErr = tc.err

		c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/import-wikidata", []byte(`{"qid":"Q1"}`))
		setAdminUser(c)
		if err := h.ImportWikidataAuthor(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != tc.want {
			t.Fatalf("%v: expected %d, got %d", tc.err, tc.want, rec.Code)
		}
		var got map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got["error"] == "" || got["code"] != tc.code {
			t.Fatalf("%v: unexpected body %v", tc.err, got)
		}
	}
}

func TestImportWikidataAuthor_MalformedBody_BadRequest(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/import-wikidata", []byte(`{"qid":`))
	setAdminUser(c)
	if err := h.ImportWikidataAuthor(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestImportWikidataAuthorRoute_Auth wires the handler with the router.go
// middleware chain (adminRole + requireBearer).
func TestImportWikidataAuthorRoute_Auth(t *testing.T) {
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
		svc.importResult = &adminService.AuthorImportResult{Outcome: adminService.AuthorOutcomeExisting, Match: adminService.AuthorMatchQID, Author: importedAuthor("published", "Q1")}
		var next echo.HandlerFunc = adminRole(requireBearer(h.ImportWikidataAuthor))

		c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/import-wikidata", []byte(`{"qid":"Q1"}`))
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
