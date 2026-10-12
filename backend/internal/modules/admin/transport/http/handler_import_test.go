package http

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/labstack/echo/v4"
)

// newMultipartImportRequest builds an echo.Context/ResponseRecorder pair for
// POST /admin/books/import: a multipart/form-data body with an uploaded
// "file" part plus optional extra form fields (e.g. "format", "status").
func newMultipartImportRequest(t *testing.T, fields map[string]string, filename, content string) (echo.Context, *httptest.ResponseRecorder) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	for k, v := range fields {
		if err := writer.WriteField(k, v); err != nil {
			t.Fatalf("write field %q: %v", k, err)
		}
	}
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write file content: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/books/import", body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestImportBooks_CSVHappyPath(t *testing.T) {
	h, svc := setupHandler()
	csvContent := "title,authors,genres\nWar and Peace,Leo Tolstoy,Novel\n"
	c, rec := newMultipartImportRequest(t, nil, "books.csv", csvContent)
	setAdminUser(c)

	if err := h.ImportBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(svc.lastImportRows) != 1 {
		t.Fatalf("expected 1 parsed row, got %d", len(svc.lastImportRows))
	}
	row := svc.lastImportRows[0]
	if row.Title != "War and Peace" || len(row.Authors) != 1 || row.Authors[0] != "Leo Tolstoy" {
		t.Fatalf("unexpected parsed row: %+v", row)
	}
}

func TestImportBooks_JSONHappyPath(t *testing.T) {
	h, svc := setupHandler()
	jsonContent := `[{"title":"Anna Karenina","authors":["Leo Tolstoy"]}]`
	c, rec := newMultipartImportRequest(t, nil, "books.json", jsonContent)
	setAdminUser(c)

	if err := h.ImportBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(svc.lastImportRows) != 1 || svc.lastImportRows[0].Title != "Anna Karenina" {
		t.Fatalf("unexpected parsed rows: %+v", svc.lastImportRows)
	}
}

func TestImportBooks_FormatFieldOverridesExtensionSniffing(t *testing.T) {
	h, svc := setupHandler()
	jsonContent := `[{"title":"Anna Karenina","authors":["Leo Tolstoy"]}]`
	// Filename has no recognizable extension; "format" field disambiguates.
	c, rec := newMultipartImportRequest(t, map[string]string{"format": "json"}, "upload.bin", jsonContent)
	setAdminUser(c)

	if err := h.ImportBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(svc.lastImportRows) != 1 {
		t.Fatalf("expected 1 parsed row, got %d", len(svc.lastImportRows))
	}
}

func TestImportBooks_MalformedJSON_BadRequest(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newMultipartImportRequest(t, nil, "books.json", "{not valid json")
	setAdminUser(c)

	if err := h.ImportBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestImportBooks_UnparseableCSVHeader_BadRequest(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newMultipartImportRequest(t, nil, "books.csv", "")
	setAdminUser(c)

	if err := h.ImportBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestImportBooks_UndeterminableFormat_BadRequest(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newMultipartImportRequest(t, nil, "upload.bin", "title,authors\nBook,Author\n")
	setAdminUser(c)

	if err := h.ImportBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestImportBooks_TooManyRows_UnprocessableEntity(t *testing.T) {
	h, _ := setupHandler()
	var sb strings.Builder
	sb.WriteString("title,authors\n")
	for i := 0; i < maxImportRows+1; i++ {
		sb.WriteString(fmt.Sprintf("Book %d,Author\n", i))
	}
	c, rec := newMultipartImportRequest(t, nil, "books.csv", sb.String())
	setAdminUser(c)

	if err := h.ImportBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", rec.Code)
	}
}

func TestImportBooks_EmptyFile_NotAnError(t *testing.T) {
	h, svc := setupHandler()
	c, rec := newMultipartImportRequest(t, nil, "books.csv", "title,authors\n")
	setAdminUser(c)

	if err := h.ImportBooks(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if len(svc.lastImportRows) != 0 {
		t.Fatalf("expected 0 rows, got %d", len(svc.lastImportRows))
	}
	body := rec.Body.String()
	for _, want := range []string{`"created_authors":[]`, `"created_genres":[]`} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected %s in the report, got %s", want, body)
		}
	}
}

func TestImportBooks_StatusFormField(t *testing.T) {
	cases := []struct {
		name   string
		fields map[string]string
		want   string
	}{
		{"absent defaults to published", nil, catalogDomain.BookStatusPublished},
		{"empty defaults to published", map[string]string{"status": "  "}, catalogDomain.BookStatusPublished},
		{"pending", map[string]string{"status": "pending"}, catalogDomain.BookStatusPending},
		{"published", map[string]string{"status": "published"}, catalogDomain.BookStatusPublished},
	}
	for _, tc := range cases {
		h, svc := setupHandler()
		c, rec := newMultipartImportRequest(t, tc.fields, "books.csv", "title,authors\nBook,Author\n")
		setAdminUser(c)

		if err := h.ImportBooks(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", tc.name, err)
		}
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: expected 200, got %d: %s", tc.name, rec.Code, rec.Body.String())
		}
		if svc.lastImportStatus != tc.want {
			t.Fatalf("%s: expected status %q, got %q", tc.name, tc.want, svc.lastImportStatus)
		}
	}
}

func TestImportBooks_InvalidStatus_BadRequest(t *testing.T) {
	for _, status := range []string{"rejected", "hidden", "true"} {
		h, svc := setupHandler()
		c, rec := newMultipartImportRequest(t, map[string]string{"status": status}, "books.csv", "title,authors\nBook,Author\n")
		setAdminUser(c)

		if err := h.ImportBooks(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", status, err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: expected 400, got %d", status, rec.Code)
		}
		if svc.lastImportRows != nil {
			t.Fatalf("%s: service must not be called", status)
		}
	}
}
