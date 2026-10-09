package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Genre moderation queue + merge (docs/features/genre-moderation-queue.md).

func (f *fakeAdminService) ListGenreModerationQueue(ctx context.Context, filter repository.GenreModerationFilter, page, limit int) ([]adminService.GenreQueueItem, int64, error) {
	f.lastGenreQueueFilter = filter
	f.lastGenreQueuePage = page
	f.lastGenreQueueLimit = limit
	if f.genreQueueErr != nil {
		return nil, 0, f.genreQueueErr
	}
	return f.genreQueueItems, int64(len(f.genreQueueItems)), nil
}

func (f *fakeAdminService) BulkSetGenreStatus(ctx context.Context, ids []uuid.UUID, status string) (adminService.GenreBulkStatusReport, error) {
	f.genreBulkCalled = true
	f.lastGenreBulkIDs = ids
	f.lastGenreBulkStatus = status
	if f.genreBulkErr != nil {
		return adminService.GenreBulkStatusReport{}, f.genreBulkErr
	}
	if f.genreBulkReport != nil {
		return *f.genreBulkReport, nil
	}
	return adminService.GenreBulkStatusReport{Processed: len(ids), Updated: len(ids), Errors: []adminService.GenreBulkStatusRowError{}}, nil
}

func (f *fakeAdminService) MergeGenres(ctx context.Context, sourceID, targetID uuid.UUID) (*catalogDomain.Genre, int64, error) {
	f.mergeGenresCalled = true
	f.lastMergeGenreSource = sourceID
	f.lastMergeGenreTarget = targetID
	if f.mergeGenresErr != nil {
		return nil, 0, f.mergeGenresErr
	}
	if f.mergeGenresResult != nil {
		return f.mergeGenresResult, f.mergeGenresBooks, nil
	}
	return &catalogDomain.Genre{ID: targetID, Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusPublished}, f.mergeGenresBooks, nil
}

func TestListGenreModerationQueue_Shape(t *testing.T) {
	h, svc := setupHandler()
	qid := "Q123"
	row := catalogDomain.Genre{ID: uuid.New(), Name: "романы", Slug: "romany", Status: "pending", WikidataQID: &qid}
	hint := catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: "published"}
	bookID := uuid.New()
	svc.genreQueueItems = []adminService.GenreQueueItem{
		{
			Genre: row, BooksTotal: 3, BooksVisible: 1,
			SampleBooks: []repository.SampleBook{{ID: bookID, Title: "Война и мир", Status: catalogDomain.BookStatusPending}},
			Similar:     []adminService.GenreHint{{Genre: hint, Match: adminService.GenreMatchSameStem}},
		},
		{Genre: catalogDomain.Genre{ID: uuid.New(), Name: "эпос", Status: "pending"}, SampleBooks: []repository.SampleBook{}, Similar: []adminService.GenreHint{}},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/genres/moderation?status=rejected&qid=without&search=ром&page=2&limit=500", nil)
	setAdminUser(c)
	if err := h.ListGenreModerationQueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	f := svc.lastGenreQueueFilter
	if f.Status != "rejected" || f.QID != "without" || f.Search != "ром" || svc.lastGenreQueuePage != 2 || svc.lastGenreQueueLimit != 50 {
		t.Fatalf("unexpected params passed: %+v page=%d limit=%d", f, svc.lastGenreQueuePage, svc.lastGenreQueueLimit)
	}

	var body listGenreQueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 2 || body.Page != 2 || body.Limit != 50 || len(body.Items) != 2 {
		t.Fatalf("unexpected envelope: %+v", body)
	}
	it := body.Items[0]
	if it.Genre.ID != row.ID.String() || it.Genre.Status != "pending" || it.Genre.WikidataQID == nil || *it.Genre.WikidataQID != qid ||
		it.BooksTotal != 3 || it.BooksVisible != 1 ||
		len(it.SampleBooks) != 1 || it.SampleBooks[0].ID != bookID.String() || it.SampleBooks[0].Status != catalogDomain.BookStatusPending ||
		len(it.Similar) != 1 || it.Similar[0].Genre.ID != hint.ID.String() || it.Similar[0].Match != "same_stem" || it.Similar[0].Genre.Status != "published" {
		t.Fatalf("unexpected item: %+v", it)
	}
	raw := rec.Body.String()
	if !strings.Contains(raw, `"similar":[]`) || !strings.Contains(raw, `"sample_books":[]`) || !strings.Contains(raw, `"created_at"`) {
		t.Fatalf("expected empty arrays and created_at: %s", raw)
	}
	if strings.Contains(raw, "merged_into") {
		t.Fatalf("merged_into_id must never be exposed: %s", raw)
	}
}

func TestListGenreModerationQueue_Errors(t *testing.T) {
	h, svc := setupHandler()
	svc.genreQueueErr = fmt.Errorf("%w: qid must be with or without", adminService.ErrInvalidModerationFilter)
	c, rec := newContext(http.MethodGet, "/api/v1/admin/genres/moderation?qid=maybe", nil)
	setAdminUser(c)
	if err := h.ListGenreModerationQueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	h, svc = setupHandler()
	svc.genreQueueErr = errors.New("boom")
	c, rec = newContext(http.MethodGet, "/api/v1/admin/genres/moderation", nil)
	setAdminUser(c)
	if err := h.ListGenreModerationQueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestBulkSetGenreStatus_Success(t *testing.T) {
	h, svc := setupHandler()
	a, b := uuid.New(), uuid.New()
	svc.genreBulkReport = &adminService.GenreBulkStatusReport{
		Processed: 2, Updated: 1,
		Errors: []adminService.GenreBulkStatusRowError{{GenreID: b.String(), Message: "genre not found"}},
	}

	body := fmt.Sprintf(`{"ids":[%q,%q],"status":"published"}`, a, b)
	c, rec := newContext(http.MethodPost, "/api/v1/admin/genres/bulk-status", []byte(body))
	setAdminUser(c)
	if err := h.BulkSetGenreStatus(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastGenreBulkStatus != "published" || len(svc.lastGenreBulkIDs) != 2 || svc.lastGenreBulkIDs[0] != a || svc.lastGenreBulkIDs[1] != b {
		t.Fatalf("unexpected args: %v %q", svc.lastGenreBulkIDs, svc.lastGenreBulkStatus)
	}
	var got map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errs, _ := got["errors"].([]interface{})
	if got["processed"] != float64(2) || got["updated"] != float64(1) || len(errs) != 1 {
		t.Fatalf("unexpected report: %v", got)
	}
	row := errs[0].(map[string]interface{})
	if row["genre_id"] != b.String() || row["message"] != "genre not found" {
		t.Fatalf("unexpected row error: %v", row)
	}
	if _, ok := row["name"]; !ok {
		t.Fatalf("expected name key in row error: %v", row)
	}
}

func TestBulkSetGenreStatus_BadRequests(t *testing.T) {
	id := uuid.New().String()
	cases := map[string]string{
		"invalid status": fmt.Sprintf(`{"ids":[%q],"status":"deleted"}`, id),
		"missing status": fmt.Sprintf(`{"ids":[%q]}`, id),
		"empty ids":      `{"ids":[],"status":"published"}`,
		"missing ids":    `{"status":"published"}`,
		"malformed id":   `{"ids":["not-a-uuid"],"status":"published"}`,
		"malformed body": `{"ids":`,
	}
	for name, body := range cases {
		h, svc := setupHandler()
		c, rec := newContext(http.MethodPost, "/api/v1/admin/genres/bulk-status", []byte(body))
		setAdminUser(c)
		if err := h.BulkSetGenreStatus(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", name, rec.Code)
		}
		if svc.genreBulkCalled {
			t.Fatalf("%s: service must not be called", name)
		}
	}
}

func TestBulkSetGenreStatus_TooLarge_BadRequest(t *testing.T) {
	h, svc := setupHandler()
	svc.genreBulkErr = adminService.ErrBulkStatusTooLarge
	c, rec := newContext(http.MethodPost, "/api/v1/admin/genres/bulk-status", []byte(fmt.Sprintf(`{"ids":[%q],"status":"rejected"}`, uuid.New())))
	setAdminUser(c)
	if err := h.BulkSetGenreStatus(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func newMergeGenreContext(sourceID, body string) (echo.Context, func() int) {
	c, rec := newContext(http.MethodPost, "/api/v1/admin/genres/"+sourceID+"/merge", []byte(body))
	c.SetParamNames("id")
	c.SetParamValues(sourceID)
	return c, func() int { return rec.Code }
}

func TestMergeGenres_Success(t *testing.T) {
	h, svc := setupHandler()
	source, target := uuid.New(), uuid.New()
	qid := "Q8261"
	svc.mergeGenresResult = &catalogDomain.Genre{ID: target, Name: "роман", Slug: "roman", Status: "published", WikidataQID: &qid}
	svc.mergeGenresBooks = 2

	c, rec := newContext(http.MethodPost, "/api/v1/admin/genres/"+source.String()+"/merge", []byte(fmt.Sprintf(`{"canonical_id":%q}`, target)))
	c.SetParamNames("id")
	c.SetParamValues(source.String())
	setAdminUser(c)
	if err := h.MergeGenres(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastMergeGenreSource != source || svc.lastMergeGenreTarget != target {
		t.Fatalf("unexpected args: %s -> %s", svc.lastMergeGenreSource, svc.lastMergeGenreTarget)
	}
	var body mergeGenreResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.BooksReassigned != 2 || body.Genre.ID != target.String() || body.Genre.WikidataQID == nil || *body.Genre.WikidataQID != qid {
		t.Fatalf("unexpected response: %+v", body)
	}
}

func TestMergeGenres_Errors(t *testing.T) {
	valid := uuid.New().String()
	cases := []struct {
		name       string
		source     string
		body       string
		serviceErr error
		want       int
		called     bool
	}{
		{"bad source id", "nope", fmt.Sprintf(`{"canonical_id":%q}`, valid), nil, http.StatusBadRequest, false},
		{"bad body", uuid.New().String(), `{"canonical_id":`, nil, http.StatusBadRequest, false},
		{"bad canonical id", uuid.New().String(), `{"canonical_id":"x"}`, nil, http.StatusBadRequest, false},
		{"self merge", valid, fmt.Sprintf(`{"canonical_id":%q}`, valid), adminService.ErrCannotMergeGenreIntoSelf, http.StatusBadRequest, true},
		{"not found or alias", uuid.New().String(), fmt.Sprintf(`{"canonical_id":%q}`, valid), adminService.ErrGenreNotFound, http.StatusNotFound, true},
		{"internal", uuid.New().String(), fmt.Sprintf(`{"canonical_id":%q}`, valid), errors.New("boom"), http.StatusInternalServerError, true},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		svc.mergeGenresErr = tc.serviceErr
		c, code := newMergeGenreContext(tc.source, tc.body)
		setAdminUser(c)
		if err := h.MergeGenres(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if code() != tc.want {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.want, code())
		}
		if svc.mergeGenresCalled != tc.called {
			t.Fatalf("%s: service called=%v, want %v", tc.name, svc.mergeGenresCalled, tc.called)
		}
	}
}

// TestUpdateGenre_AliasNotFound: the service reports an alias row as
// ErrGenreNotFound (GetGenreByID hides aliases), so PATCH gives 404.
func TestUpdateGenre_AliasNotFound(t *testing.T) {
	h, svc := setupHandler()
	svc.genreErr = adminService.ErrGenreNotFound
	id := uuid.New().String()
	c, rec := newContext(http.MethodPatch, "/api/v1/admin/genres/"+id, []byte(`{"name":"романы"}`))
	c.SetParamNames("id")
	c.SetParamValues(id)
	setAdminUser(c)
	if err := h.UpdateGenre(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// TestGenreModerationRoutes_Auth wires the handlers with the same
// middleware chain as router.go (adminRole, plus requireBearer on POSTs).
func TestGenreModerationRoutes_Auth(t *testing.T) {
	adminRole := middleware.RequireRole(usersDomain.RoleAdmin)
	requireBearer := middleware.RequireBearerAuth()
	source, target := uuid.New().String(), uuid.New().String()

	type route struct {
		name   string
		method string
		path   string
		body   []byte
		next   func(h *Handler) echo.HandlerFunc
	}
	routes := []route{
		{"queue", http.MethodGet, "/api/v1/admin/genres/moderation", nil,
			func(h *Handler) echo.HandlerFunc { return adminRole(h.ListGenreModerationQueue) }},
		{"bulk", http.MethodPost, "/api/v1/admin/genres/bulk-status", []byte(fmt.Sprintf(`{"ids":[%q],"status":"published"}`, source)),
			func(h *Handler) echo.HandlerFunc { return adminRole(requireBearer(h.BulkSetGenreStatus)) }},
		{"merge", http.MethodPost, "/api/v1/admin/genres/" + source + "/merge", []byte(fmt.Sprintf(`{"canonical_id":%q}`, target)),
			func(h *Handler) echo.HandlerFunc { return adminRole(requireBearer(h.MergeGenres)) }},
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
			h, _ := setupHandler()
			c, rec := newContext(r.method, r.path, r.body)
			if r.name == "merge" {
				c.SetParamNames("id")
				c.SetParamValues(source)
			}
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
