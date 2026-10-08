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

func importGenre(name, slug, status, qid string) *catalogDomain.Genre {
	g := &catalogDomain.Genre{ID: uuid.New(), Name: name, Slug: slug, Status: status}
	if qid != "" {
		q := qid
		g.WikidataQID = &q
	}
	return g
}

func TestSearchWikidataGenres_Shape(t *testing.T) {
	h, svc := setupHandler()
	target := importGenre("фэнтези", "fentezi", "pending", "")
	alias := importGenre("фентези", "fentezi-2", "rejected", "Q2")
	ru := "фэнтези"
	yes := true
	svc.genreSearchResult = &adminService.GenreSearchResult{
		ClassCheck: adminService.GenreClassCheckOK,
		Items: []adminService.GenreSearchItem{
			{QID: "Q1", Label: "фэнтези", Description: "жанр", RuLabel: &ru, LiteraryGenre: &yes},
			{QID: "Q2", Label: "фентези", RuLabel: &ru, LiteraryGenre: &yes,
				Existing: &adminService.GenreExistingMarker{Genre: target, ViaAlias: alias}},
		},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/genres/wikidata-search?q=%20%D1%84%D1%8D%D0%BD%D1%82%D0%B5%D0%B7%D0%B8%20", nil)
	setAdminUser(c)
	if err := h.SearchWikidataGenres(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastGenreSearchQ != "фэнтези" {
		t.Fatalf("query passed = %q", svc.lastGenreSearchQ)
	}

	var got struct {
		ClassCheck string                   `json:"class_check"`
		Items      []map[string]interface{} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ClassCheck != "ok" || len(got.Items) != 2 {
		t.Fatalf("unexpected body %s", rec.Body.String())
	}
	first := got.Items[0]
	if first["qid"] != "Q1" || first["ru_label"] != "фэнтези" || first["literary_genre"] != true || first["description"] != "жанр" {
		t.Fatalf("unexpected first item %v", first)
	}
	if v, ok := first["existing"]; !ok || v != nil {
		t.Fatalf("existing must be present and null, got %v", first)
	}
	existing, _ := got.Items[1]["existing"].(map[string]interface{})
	if existing["id"] != target.ID.String() || existing["status"] != "pending" || existing["slug"] != "fentezi" {
		t.Fatalf("unexpected existing %v", existing)
	}
	via, _ := existing["via_alias"].(map[string]interface{})
	if via["id"] != alias.ID.String() || via["name"] != "фентези" || via["slug"] != "fentezi-2" {
		t.Fatalf("unexpected via_alias %v", existing["via_alias"])
	}
}

func TestSearchWikidataGenres_UnavailableNulls(t *testing.T) {
	h, svc := setupHandler()
	target := importGenre("роман", "roman", "published", "Q8261")
	svc.genreSearchResult = &adminService.GenreSearchResult{
		ClassCheck: adminService.GenreClassCheckUnavailable,
		Items: []adminService.GenreSearchItem{{QID: "Q8261", Label: "роман",
			Existing: &adminService.GenreExistingMarker{Genre: target}}},
	}
	c, rec := newContext(http.MethodGet, "/api/v1/admin/genres/wikidata-search?q=x", nil)
	setAdminUser(c)
	if err := h.SearchWikidataGenres(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got map[string]interface{}
	_ = json.Unmarshal(rec.Body.Bytes(), &got)
	if got["class_check"] != "unavailable" {
		t.Fatalf("unexpected body %v", got)
	}
	item := got["items"].([]interface{})[0].(map[string]interface{})
	for _, k := range []string{"ru_label", "literary_genre"} {
		if v, ok := item[k]; !ok || v != nil {
			t.Fatalf("%s must be present and null, got %v", k, item)
		}
	}
	existing := item["existing"].(map[string]interface{})
	if v, ok := existing["via_alias"]; !ok || v != nil {
		t.Fatalf("via_alias must be present and null, got %v", existing)
	}
}

func TestSearchWikidataGenres_EmptyItemsArray(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/admin/genres/wikidata-search?q=x", nil)
	setAdminUser(c)
	if err := h.SearchWikidataGenres(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body := rec.Body.String(); body != "{\"class_check\":\"ok\",\"items\":[]}\n" {
		t.Fatalf("unexpected body %q", body)
	}
}

func TestSearchWikidataGenres_Errors(t *testing.T) {
	cases := []struct {
		q    string
		err  error
		want int
	}{
		{"", nil, http.StatusBadRequest},
		{"%20%20", nil, http.StatusBadRequest},
		{"x", bookdata.ErrInvalidQuery, http.StatusBadRequest},
		{"x", bookdata.ErrRateLimited, http.StatusBadGateway},
		{"x", fmt.Errorf("%w: status 503", bookdata.ErrSourceUnavailable), http.StatusBadGateway},
		{"x", adminService.ErrGenreImportUnavailable, http.StatusServiceUnavailable},
		{"x", errors.New("boom"), http.StatusInternalServerError},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		svc.genreSearchErr = tc.err
		c, rec := newContext(http.MethodGet, "/api/v1/admin/genres/wikidata-search?q="+tc.q, nil)
		setAdminUser(c)
		if err := h.SearchWikidataGenres(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != tc.want {
			t.Fatalf("q=%q err=%v: expected %d, got %d", tc.q, tc.err, tc.want, rec.Code)
		}
		var got map[string]string
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got["error"] == "" {
			t.Fatalf("q=%q err=%v: missing error message", tc.q, tc.err)
		}
	}
}

func TestImportWikidataGenre_Created(t *testing.T) {
	h, svc := setupHandler()
	svc.genreImportResult = &adminService.GenreImportResult{
		Outcome: adminService.GenreOutcomeCreated,
		Genre:   importGenre("фэнтези", "fentezi", "pending", "Q132311"),
	}

	c, rec := newContext(http.MethodPost, "/api/v1/admin/genres/import-wikidata", []byte(`{"qid":"Q132311"}`))
	setAdminUser(c)
	if err := h.ImportWikidataGenre(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastGenreImportQID != "Q132311" {
		t.Fatalf("qid passed = %q", svc.lastGenreImportQID)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got["outcome"] != "created" || got["qid_backfilled"] != false {
		t.Fatalf("unexpected body %v", got)
	}
	for _, k := range []string{"match", "qid_mismatch", "via_alias"} {
		if v, ok := got[k]; !ok || v != nil {
			t.Fatalf("%s must be present and null, got %v", k, got)
		}
	}
	genre, _ := got["genre"].(map[string]interface{})
	if genre["status"] != "pending" || genre["wikidata_qid"] != "Q132311" || genre["slug"] != "fentezi" {
		t.Fatalf("unexpected genre %v", genre)
	}
}

func TestImportWikidataGenre_ExistingAndRejected_OK(t *testing.T) {
	mismatch := "Q99"
	alias := importGenre("фентези", "fentezi-2", "rejected", "Q5")
	cases := []struct {
		name   string
		result adminService.GenreImportResult
	}{
		{"qid", adminService.GenreImportResult{Outcome: adminService.GenreOutcomeExisting, Match: adminService.GenreMatchQID}},
		{"slug backfilled", adminService.GenreImportResult{Outcome: adminService.GenreOutcomeExisting, Match: adminService.GenreMatchSlug, QIDBackfilled: true}},
		{"slug mismatch", adminService.GenreImportResult{Outcome: adminService.GenreOutcomeExisting, Match: adminService.GenreMatchSlug, QIDMismatch: &mismatch}},
		{"via alias", adminService.GenreImportResult{Outcome: adminService.GenreOutcomeExisting, Match: adminService.GenreMatchQID, ViaAlias: alias}},
		{"rejected", adminService.GenreImportResult{Outcome: adminService.GenreOutcomeRejected, Match: adminService.GenreMatchSlug}},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		res := tc.result
		res.Genre = importGenre("фэнтези", "fentezi", "published", "Q1")
		svc.genreImportResult = &res

		c, rec := newContext(http.MethodPost, "/api/v1/admin/genres/import-wikidata", []byte(`{"qid":"Q1"}`))
		setAdminUser(c)
		if err := h.ImportWikidataGenre(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d", tc.name, rec.Code)
		}
		var got map[string]interface{}
		_ = json.Unmarshal(rec.Body.Bytes(), &got)
		if got["outcome"] != res.Outcome || got["match"] != res.Match || got["qid_backfilled"] != res.QIDBackfilled {
			t.Fatalf("%s: unexpected body %v", tc.name, got)
		}
		if res.QIDMismatch != nil && got["qid_mismatch"] != "Q99" {
			t.Fatalf("%s: qid_mismatch = %v", tc.name, got["qid_mismatch"])
		}
		if res.ViaAlias != nil {
			via, _ := got["via_alias"].(map[string]interface{})
			if via["id"] != alias.ID.String() || via["wikidata_qid"] != "Q5" {
				t.Fatalf("%s: via_alias = %v", tc.name, got["via_alias"])
			}
		} else if v, ok := got["via_alias"]; !ok || v != nil {
			t.Fatalf("%s: via_alias must be null, got %v", tc.name, got)
		}
	}
}

func TestImportWikidataGenre_ErrorMapping(t *testing.T) {
	cases := []struct {
		err  error
		want int
		code string
	}{
		{adminService.ErrInvalidQID, http.StatusBadRequest, ""},
		{adminService.ErrWikidataEntityNotFound, http.StatusNotFound, ""},
		{adminService.ErrNoRussianLabel, http.StatusUnprocessableEntity, "no_ru_label"},
		{adminService.ErrNotALiteraryGenre, http.StatusUnprocessableEntity, "not_a_literary_genre"},
		{fmt.Errorf("create genre: %w", adminService.ErrDuplicateSlug), http.StatusConflict, ""},
		{fmt.Errorf("create genre: %w", adminService.ErrDuplicateGenreQID), http.StatusConflict, ""},
		{bookdata.ErrRateLimited, http.StatusBadGateway, ""},
		{fmt.Errorf("classify genre Q1: %w: deadline", bookdata.ErrSourceUnavailable), http.StatusBadGateway, ""},
		{adminService.ErrGenreImportUnavailable, http.StatusServiceUnavailable, ""},
		{errors.New("boom"), http.StatusInternalServerError, ""},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		svc.genreImportErr = tc.err

		c, rec := newContext(http.MethodPost, "/api/v1/admin/genres/import-wikidata", []byte(`{"qid":"Q1"}`))
		setAdminUser(c)
		if err := h.ImportWikidataGenre(c); err != nil {
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

func TestImportWikidataGenre_MalformedBody_BadRequest(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodPost, "/api/v1/admin/genres/import-wikidata", []byte(`{"qid":`))
	setAdminUser(c)
	if err := h.ImportWikidataGenre(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestGenreImportRoutes_Auth wires the handlers with the router.go
// middleware chain (adminRole, plus requireBearer on the POST).
func TestGenreImportRoutes_Auth(t *testing.T) {
	adminRole := middleware.RequireRole(usersDomain.RoleAdmin)
	requireBearer := middleware.RequireBearerAuth()

	routes := []struct {
		name   string
		method string
		path   string
		body   []byte
		next   func(h *Handler) echo.HandlerFunc
	}{
		{"search", http.MethodGet, "/api/v1/admin/genres/wikidata-search?q=x", nil,
			func(h *Handler) echo.HandlerFunc { return adminRole(h.SearchWikidataGenres) }},
		{"import", http.MethodPost, "/api/v1/admin/genres/import-wikidata", []byte(`{"qid":"Q1"}`),
			func(h *Handler) echo.HandlerFunc { return adminRole(requireBearer(h.ImportWikidataGenre)) }},
	}
	users := []struct {
		name   string
		user   *usersDomain.User
		bearer bool
		get    int
		post   int
	}{
		{"unauthenticated", nil, false, http.StatusUnauthorized, http.StatusUnauthorized},
		{"moderator", &usersDomain.User{Role: usersDomain.RoleModerator}, true, http.StatusForbidden, http.StatusForbidden},
		{"admin without bearer", &usersDomain.User{Role: usersDomain.RoleAdmin}, false, http.StatusOK, http.StatusUnauthorized},
		{"admin with bearer", &usersDomain.User{Role: usersDomain.RoleAdmin}, true, http.StatusOK, http.StatusOK},
	}
	for _, r := range routes {
		for _, u := range users {
			h, svc := setupHandler()
			svc.genreImportResult = &adminService.GenreImportResult{
				Outcome: adminService.GenreOutcomeExisting, Match: adminService.GenreMatchQID,
				Genre: importGenre("роман", "roman", "published", "Q1"),
			}
			c, rec := newContext(r.method, r.path, r.body)
			if u.user != nil {
				c.Set("user", u.user)
			}
			if u.bearer {
				c.Request().Header.Set("Authorization", "Bearer token")
			}
			if err := r.next(h)(c); err != nil {
				t.Fatalf("%s/%s: unexpected error: %v", r.name, u.name, err)
			}
			want := u.post
			if r.method == http.MethodGet {
				want = u.get
			}
			if rec.Code != want {
				t.Fatalf("%s/%s: expected %d, got %d", r.name, u.name, want, rec.Code)
			}
		}
	}
}
