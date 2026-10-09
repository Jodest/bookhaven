package http

import (
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

func TestListAuthorModerationQueue_Shape(t *testing.T) {
	h, svc := setupHandler()
	qid := "Q7243"
	src := "wikidata"
	row := catalogDomain.Author{ID: uuid.New(), Name: "Л. Н. Толстой", Slug: "l-n-tolstoy", Status: "pending", ExternalSource: &src, ExternalID: &qid}
	hint := catalogDomain.Author{ID: uuid.New(), Name: "Лев Толстой", Slug: "lev-tolstoy", Status: "published"}
	bookID := uuid.New()
	svc.queueItems = []adminService.AuthorQueueItem{
		{
			Author: row, BooksTotal: 3, BooksVisible: 1,
			SampleBooks: []repository.SampleBook{{ID: bookID, Title: "Война и мир", Status: catalogDomain.BookStatusPending}},
			Similar:     []adminService.AuthorHint{{Author: hint, Match: adminService.MatchSurnameInitial}},
		},
		{Author: catalogDomain.Author{ID: uuid.New(), Name: "Гомер", Status: "pending"}, SampleBooks: []repository.SampleBook{}, Similar: []adminService.AuthorHint{}},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/authors/moderation?status=rejected&qid=with&books=none&search=x&page=2&limit=500", nil)
	setAdminUser(c)
	if err := h.ListAuthorModerationQueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	f := svc.lastQueueFilter
	if f.Status != "rejected" || f.QID != "with" || f.Books != "none" || f.Search != "x" || svc.lastQueuePage != 2 || svc.lastQueueLimit != 50 {
		t.Fatalf("unexpected params passed: %+v page=%d limit=%d", f, svc.lastQueuePage, svc.lastQueueLimit)
	}

	var body listAuthorQueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 2 || body.Page != 2 || body.Limit != 50 || len(body.Items) != 2 {
		t.Fatalf("unexpected envelope: %+v", body)
	}
	it := body.Items[0]
	if it.Author.ID != row.ID.String() || it.Author.Status != "pending" || it.Author.ExternalID == nil || *it.Author.ExternalID != qid ||
		it.BooksTotal != 3 || it.BooksVisible != 1 ||
		len(it.SampleBooks) != 1 || it.SampleBooks[0].ID != bookID.String() || it.SampleBooks[0].Status != catalogDomain.BookStatusPending ||
		len(it.Similar) != 1 || it.Similar[0].Author.ID != hint.ID.String() || it.Similar[0].Match != "surname_initial" || it.Similar[0].Author.Status != "published" {
		t.Fatalf("unexpected item: %+v", it)
	}
	if !strings.Contains(rec.Body.String(), `"similar":[]`) || !strings.Contains(rec.Body.String(), `"sample_books":[]`) {
		t.Fatalf("expected empty arrays, not null: %s", rec.Body.String())
	}
}

func TestListAuthorModerationQueue_InvalidFilter_BadRequest(t *testing.T) {
	h, svc := setupHandler()
	svc.queueErr = fmt.Errorf("%w: qid must be with or without", adminService.ErrInvalidModerationFilter)

	c, rec := newContext(http.MethodGet, "/api/v1/admin/authors/moderation?qid=maybe", nil)
	setAdminUser(c)
	if err := h.ListAuthorModerationQueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestListAuthorModerationQueue_ServiceError(t *testing.T) {
	h, svc := setupHandler()
	svc.queueErr = errors.New("boom")

	c, rec := newContext(http.MethodGet, "/api/v1/admin/authors/moderation", nil)
	setAdminUser(c)
	if err := h.ListAuthorModerationQueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestBulkSetAuthorStatus_Success(t *testing.T) {
	h, svc := setupHandler()
	a, b := uuid.New(), uuid.New()
	svc.bulkStatusReport = &adminService.BulkStatusReport{
		Processed: 2, Updated: 1,
		Errors: []adminService.BulkStatusRowError{{AuthorID: b.String(), Message: "author not found"}},
	}

	body := fmt.Sprintf(`{"ids":[%q,%q],"status":"published"}`, a, b)
	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/bulk-status", []byte(body))
	setAdminUser(c)
	if err := h.BulkSetAuthorStatus(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastBulkStatus != "published" || len(svc.lastBulkIDs) != 2 || svc.lastBulkIDs[0] != a || svc.lastBulkIDs[1] != b {
		t.Fatalf("unexpected args: %v %q", svc.lastBulkIDs, svc.lastBulkStatus)
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
	if row["author_id"] != b.String() || row["message"] != "author not found" {
		t.Fatalf("unexpected row error: %v", row)
	}
	if _, ok := row["name"]; !ok {
		t.Fatalf("expected name key in row error: %v", row)
	}
}

func TestBulkSetAuthorStatus_BadRequests(t *testing.T) {
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
		c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/bulk-status", []byte(body))
		setAdminUser(c)
		if err := h.BulkSetAuthorStatus(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", name, rec.Code)
		}
		if svc.bulkStatusCalled {
			t.Fatalf("%s: service must not be called", name)
		}
	}
}

func TestBulkSetAuthorStatus_TooLarge_BadRequest(t *testing.T) {
	h, svc := setupHandler()
	svc.bulkStatusErr = adminService.ErrBulkStatusTooLarge

	c, rec := newContext(http.MethodPost, "/api/v1/admin/authors/bulk-status", []byte(fmt.Sprintf(`{"ids":[%q],"status":"rejected"}`, uuid.New())))
	setAdminUser(c)
	if err := h.BulkSetAuthorStatus(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestAuthorModerationRoutes_Auth wires the handlers with the same
// middleware chain as router.go (adminRole, plus requireBearer on the POST).
func TestAuthorModerationRoutes_Auth(t *testing.T) {
	adminRole := middleware.RequireRole(usersDomain.RoleAdmin)
	requireBearer := middleware.RequireBearerAuth()
	h, _ := setupHandler()

	get := adminRole(h.ListAuthorModerationQueue)
	post := adminRole(requireBearer(h.BulkSetAuthorStatus))
	body := []byte(fmt.Sprintf(`{"ids":[%q],"status":"published"}`, uuid.New()))

	cases := []struct {
		name   string
		method string
		next   echo.HandlerFunc
		user   *usersDomain.User
		bearer bool
		want   int
	}{
		{"get unauthenticated", http.MethodGet, get, nil, false, http.StatusUnauthorized},
		{"get moderator", http.MethodGet, get, &usersDomain.User{Role: usersDomain.RoleModerator}, true, http.StatusForbidden},
		{"get admin", http.MethodGet, get, &usersDomain.User{Role: usersDomain.RoleAdmin}, false, http.StatusOK},
		{"post unauthenticated", http.MethodPost, post, nil, false, http.StatusUnauthorized},
		{"post moderator", http.MethodPost, post, &usersDomain.User{Role: usersDomain.RoleModerator}, true, http.StatusForbidden},
		{"post admin without bearer", http.MethodPost, post, &usersDomain.User{Role: usersDomain.RoleAdmin}, false, http.StatusUnauthorized},
		{"post admin with bearer", http.MethodPost, post, &usersDomain.User{Role: usersDomain.RoleAdmin}, true, http.StatusOK},
	}
	for _, tc := range cases {
		var c echo.Context
		var code func() int
		if tc.method == http.MethodGet {
			ctx, rec := newContext(tc.method, "/api/v1/admin/authors/moderation", nil)
			c, code = ctx, func() int { return rec.Code }
		} else {
			ctx, rec := newContext(tc.method, "/api/v1/admin/authors/bulk-status", body)
			c, code = ctx, func() int { return rec.Code }
		}
		if tc.user != nil {
			c.Set("user", tc.user)
		}
		if tc.bearer {
			c.Request().Header.Set("Authorization", "Bearer token")
		}
		if err := tc.next(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if code() != tc.want {
			t.Fatalf("%s: expected %d, got %d", tc.name, tc.want, code())
		}
	}
}
