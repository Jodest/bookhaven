package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Book status model (docs/features/book-status-model.md).

func TestCreateBook_Status(t *testing.T) {
	t.Run("omitted is passed as nil and responds published", func(t *testing.T) {
		h, svc := setupHandler()
		c, rec := newContext(http.MethodPost, "/api/v1/admin/books", []byte(`{"title":"Книга","language":"ru"}`))
		setAdminUser(c)
		if err := h.CreateBook(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
		}
		if svc.lastBookInput == nil || svc.lastBookInput.Status != nil {
			t.Fatalf("expected nil status in the input, got %+v", svc.lastBookInput)
		}
		var body bookResponse
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Status != catalogDomain.BookStatusPublished {
			t.Fatalf("expected status published, got %+v err=%v", body, err)
		}
	})

	t.Run("explicit status is passed through", func(t *testing.T) {
		h, svc := setupHandler()
		c, rec := newContext(http.MethodPost, "/api/v1/admin/books", []byte(`{"title":"Книга","language":"ru","status":"pending"}`))
		setAdminUser(c)
		if err := h.CreateBook(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusCreated || svc.lastBookInput.Status == nil || *svc.lastBookInput.Status != "pending" {
			t.Fatalf("expected pending passed through, got %d %+v", rec.Code, svc.lastBookInput)
		}
	})

	t.Run("invalid status is 400 and the service is not called", func(t *testing.T) {
		h, svc := setupHandler()
		c, rec := newContext(http.MethodPost, "/api/v1/admin/books", []byte(`{"title":"Книга","language":"ru","status":"hidden"}`))
		setAdminUser(c)
		if err := h.CreateBook(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusBadRequest || svc.lastBookInput != nil {
			t.Fatalf("expected 400 without a service call, got %d %+v", rec.Code, svc.lastBookInput)
		}
	})

	t.Run("service ErrInvalidBookStatus maps to 400", func(t *testing.T) {
		h, svc := setupHandler()
		svc.createBookErr = adminService.ErrInvalidBookStatus
		c, rec := newContext(http.MethodPost, "/api/v1/admin/books", []byte(`{"title":"Книга","language":"ru"}`))
		setAdminUser(c)
		if err := h.CreateBook(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", rec.Code)
		}
	})
}

func TestUpdateBook_Status(t *testing.T) {
	newUpdate := func(id uuid.UUID, body string) (echo.Context, func() int, func() []byte) {
		c, rec := newContext(http.MethodPatch, "/api/v1/admin/books/"+id.String(), []byte(body))
		c.SetParamNames("id")
		c.SetParamValues(id.String())
		setAdminUser(c)
		return c, func() int { return rec.Code }, func() []byte { return rec.Body.Bytes() }
	}

	t.Run("omitted keeps the status", func(t *testing.T) {
		h, svc := setupHandler()
		id := uuid.New()
		svc.books = []catalogDomain.Book{{ID: id, Title: "Книга", Language: "ru", Status: catalogDomain.BookStatusRejected}}
		c, code, body := newUpdate(id, `{"title":"Книга","language":"ru"}`)
		if err := h.UpdateBook(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var resp bookResponse
		if code() != http.StatusOK || json.Unmarshal(body(), &resp) != nil || resp.Status != catalogDomain.BookStatusRejected {
			t.Fatalf("expected 200 with rejected unchanged, got %d %s", code(), body())
		}
		if svc.lastBookInput.Status != nil {
			t.Fatalf("expected nil status in the input, got %q", *svc.lastBookInput.Status)
		}
	})

	t.Run("changed", func(t *testing.T) {
		h, svc := setupHandler()
		id := uuid.New()
		svc.books = []catalogDomain.Book{{ID: id, Title: "Книга", Language: "ru", Status: catalogDomain.BookStatusPending}}
		c, code, body := newUpdate(id, `{"title":"Книга","language":"ru","status":"published"}`)
		if err := h.UpdateBook(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		var resp bookResponse
		if code() != http.StatusOK || json.Unmarshal(body(), &resp) != nil || resp.Status != catalogDomain.BookStatusPublished {
			t.Fatalf("expected 200 with published, got %d %s", code(), body())
		}
	})

	t.Run("invalid is 400", func(t *testing.T) {
		h, svc := setupHandler()
		id := uuid.New()
		svc.books = []catalogDomain.Book{{ID: id, Title: "Книга", Language: "ru", Status: catalogDomain.BookStatusPending}}
		c, code, _ := newUpdate(id, `{"title":"Книга","language":"ru","status":"visible"}`)
		if err := h.UpdateBook(c); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if code() != http.StatusBadRequest || svc.lastBookInput != nil {
			t.Fatalf("expected 400 without a service call, got %d", code())
		}
	})
}

func TestBulkSetBookStatus_Success(t *testing.T) {
	h, svc := setupHandler()
	a, b := uuid.New(), uuid.New()
	svc.bookBulkReport = &adminService.BookBulkStatusReport{
		Processed: 2, Updated: 1,
		Errors: []adminService.BookBulkStatusRowError{{BookID: b.String(), Message: "book not found"}},
	}

	body := fmt.Sprintf(`{"ids":[%q,%q],"status":"published"}`, a, b)
	c, rec := newContext(http.MethodPost, "/api/v1/admin/books/bulk-status", []byte(body))
	setAdminUser(c)
	if err := h.BulkSetBookStatus(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastBookBulkStatus != "published" || len(svc.lastBookBulkIDs) != 2 || svc.lastBookBulkIDs[0] != a || svc.lastBookBulkIDs[1] != b {
		t.Fatalf("unexpected args: %v %q", svc.lastBookBulkIDs, svc.lastBookBulkStatus)
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
	if row["book_id"] != b.String() || row["message"] != "book not found" {
		t.Fatalf("unexpected row error: %v", row)
	}
	if _, ok := row["title"]; !ok {
		t.Fatalf("expected title key in row error: %v", row)
	}
}

func TestBulkSetBookStatus_BadRequests(t *testing.T) {
	id := uuid.New().String()
	cases := map[string]string{
		"invalid status": fmt.Sprintf(`{"ids":[%q],"status":"hidden"}`, id),
		"missing status": fmt.Sprintf(`{"ids":[%q]}`, id),
		"empty ids":      `{"ids":[],"status":"published"}`,
		"missing ids":    `{"status":"published"}`,
		"malformed id":   `{"ids":["not-a-uuid"],"status":"published"}`,
		"malformed body": `{"ids":`,
	}
	for name, body := range cases {
		h, svc := setupHandler()
		c, rec := newContext(http.MethodPost, "/api/v1/admin/books/bulk-status", []byte(body))
		setAdminUser(c)
		if err := h.BulkSetBookStatus(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", name, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", name, rec.Code)
		}
		if svc.bookBulkCalled {
			t.Fatalf("%s: service must not be called", name)
		}
	}
}

func TestBulkSetBookStatus_TooLarge_BadRequest(t *testing.T) {
	h, svc := setupHandler()
	svc.bookBulkErr = adminService.ErrBulkStatusTooLarge
	c, rec := newContext(http.MethodPost, "/api/v1/admin/books/bulk-status", []byte(fmt.Sprintf(`{"ids":[%q],"status":"rejected"}`, uuid.New())))
	setAdminUser(c)
	if err := h.BulkSetBookStatus(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

// TestBulkSetBookStatus_Auth wires the handler with the same middleware
// chain as router.go (adminRole group + requireBearer).
func TestBulkSetBookStatus_Auth(t *testing.T) {
	adminRole := middleware.RequireRole(usersDomain.RoleAdmin)
	requireBearer := middleware.RequireBearerAuth()
	body := []byte(fmt.Sprintf(`{"ids":[%q],"status":"published"}`, uuid.New()))

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
		h, _ := setupHandler()
		next := adminRole(requireBearer(h.BulkSetBookStatus))
		c, rec := newContext(http.MethodPost, "/api/v1/admin/books/bulk-status", body)
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
