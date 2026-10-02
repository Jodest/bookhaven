package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
)

func TestBulkPublishBooks_Success(t *testing.T) {
	h, svc := setupHandler()
	svc.bulkPublishReport = &adminService.BulkPublishReport{
		Processed: 2,
		Published: 1,
		Errors: []adminService.BulkPublishRowError{
			{BookID: "book-2", Title: "Плохая книга", Message: "boom"},
		},
	}

	c, rec := newContext(http.MethodPost, "/api/v1/admin/books/bulk-publish", []byte(`{"search":"Толстой"}`))
	setAdminUser(c)

	if err := h.BulkPublishBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastBulkPublishSearch != "Толстой" {
		t.Fatalf("expected search to be passed through, got %q", svc.lastBulkPublishSearch)
	}

	var body adminService.BulkPublishReport
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body.Processed != 2 || body.Published != 1 || len(body.Errors) != 1 {
		t.Fatalf("unexpected report: %+v", body)
	}
}

func TestBulkPublishBooks_EmptyBody_AllPendingReview(t *testing.T) {
	h, svc := setupHandler()

	c, rec := newContext(http.MethodPost, "/api/v1/admin/books/bulk-publish", nil)
	setAdminUser(c)

	if err := h.BulkPublishBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.lastBulkPublishSearch != "" {
		t.Fatalf("expected empty search, got %q", svc.lastBulkPublishSearch)
	}
}

func TestBulkPublishBooks_TooLarge_UnprocessableEntity(t *testing.T) {
	h, svc := setupHandler()
	svc.bulkPublishErr = adminService.ErrBulkPublishTooLarge

	c, rec := newContext(http.MethodPost, "/api/v1/admin/books/bulk-publish", nil)
	setAdminUser(c)

	if err := h.BulkPublishBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if body["error"] != adminService.ErrBulkPublishTooLarge.Error() {
		t.Fatalf("unexpected error message: %q", body["error"])
	}
}

func TestBulkPublishBooks_ServiceError_InternalServerError(t *testing.T) {
	h, svc := setupHandler()
	svc.bulkPublishErr = errors.New("boom")

	c, rec := newContext(http.MethodPost, "/api/v1/admin/books/bulk-publish", nil)
	setAdminUser(c)

	if err := h.BulkPublishBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
