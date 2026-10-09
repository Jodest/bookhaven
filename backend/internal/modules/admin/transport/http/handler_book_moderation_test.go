package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/google/uuid"
)

// Book moderation queue (docs/features/book-moderation-queue.md).

func (f *fakeAdminService) ListBookModerationQueue(ctx context.Context, filter repository.BookModerationFilter, page, limit int) ([]adminService.BookQueueItem, int64, error) {
	f.lastBookQueueFilter = filter
	f.lastBookQueuePage = page
	f.lastBookQueueLimit = limit
	if f.bookQueueErr != nil {
		return nil, 0, f.bookQueueErr
	}
	return f.bookQueueItems, int64(len(f.bookQueueItems)), nil
}

func TestListBookModerationQueue_Shape(t *testing.T) {
	h, svc := setupHandler()
	src, extID, isbn := "openlibrary", "OL123W", "9780306406157"
	year := 1999
	created := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	book := catalogDomain.Book{
		ID: uuid.New(), Title: "Война и мир", Slug: "voyna-i-mir", Status: catalogDomain.BookStatusPending,
		ISBN: &isbn, PublishedYear: &year, ExternalSource: &src, ExternalID: &extID, CreatedAt: created,
		Authors: []catalogDomain.Author{{ID: uuid.New(), Name: "Лев Толстой", Slug: "tolstoy", Status: catalogDomain.AuthorStatusPending}},
		Genres:  []catalogDomain.Genre{{ID: uuid.New(), Name: "роман", Slug: "roman", Status: catalogDomain.GenreStatusRejected}},
	}
	hint := repository.BookHintCandidate{ID: uuid.New(), Title: "Война и мир", Slug: "voyna-i-mir-2", Status: catalogDomain.BookStatusPublished, ISBN: &isbn}
	svc.bookQueueItems = []adminService.BookQueueItem{
		{Book: book, Similar: []adminService.BookHint{{Book: hint, Match: adminService.BookMatchSameISBN}}},
		{Book: catalogDomain.Book{ID: uuid.New(), Title: "Стихи", Status: catalogDomain.BookStatusPending}, Similar: []adminService.BookHint{}},
	}

	c, rec := newContext(http.MethodGet, "/api/v1/admin/books/moderation?status=rejected&source=harvested&search=мир&page=2&limit=500", nil)
	setAdminUser(c)
	if err := h.ListBookModerationQueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	f := svc.lastBookQueueFilter
	if f.Status != "rejected" || f.Source != "harvested" || f.Search != "мир" || svc.lastBookQueuePage != 2 || svc.lastBookQueueLimit != 50 {
		t.Fatalf("unexpected params passed: %+v page=%d limit=%d", f, svc.lastBookQueuePage, svc.lastBookQueueLimit)
	}

	var body listBookQueueResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 2 || body.Page != 2 || body.Limit != 50 || len(body.Items) != 2 {
		t.Fatalf("unexpected envelope: %+v", body)
	}
	it := body.Items[0]
	if it.Book.ID != book.ID.String() || it.Book.Status != "pending" || it.Book.ISBN == nil || *it.Book.ISBN != isbn ||
		it.Book.PublishedYear == nil || *it.Book.PublishedYear != year || it.Book.ExternalSource == nil || *it.Book.ExternalSource != src ||
		len(it.Book.Authors) != 1 || it.Book.Authors[0].Status != "pending" ||
		len(it.Book.Genres) != 1 || it.Book.Genres[0].Status != "rejected" ||
		it.ExternalID == nil || *it.ExternalID != extID || !it.CreatedAt.Equal(created) {
		t.Fatalf("unexpected item: %+v", it)
	}
	if len(it.Similar) != 1 || it.Similar[0].Match != "same_isbn" || it.Similar[0].Book.ID != hint.ID.String() ||
		it.Similar[0].Book.Status != "published" || it.Similar[0].Book.ISBN == nil || *it.Similar[0].Book.ISBN != isbn {
		t.Fatalf("unexpected hints: %+v", it.Similar)
	}
	raw := rec.Body.String()
	for _, want := range []string{`"similar":[]`, `"external_id":null`, `"published_year":null`, `"external_source":null`} {
		if !strings.Contains(raw, want) {
			t.Fatalf("expected %s in %s", want, raw)
		}
	}
}

func TestListBookModerationQueue_Errors(t *testing.T) {
	h, svc := setupHandler()
	svc.bookQueueErr = fmt.Errorf("%w: source must be manual or harvested", adminService.ErrInvalidModerationFilter)
	c, rec := newContext(http.MethodGet, "/api/v1/admin/books/moderation?source=bogus", nil)
	setAdminUser(c)
	if err := h.ListBookModerationQueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "invalid moderation filter: source must be manual or harvested") {
		t.Fatalf("expected 400 with message, got %d: %s", rec.Code, rec.Body.String())
	}

	svc.bookQueueErr = errors.New("db down")
	c, rec = newContext(http.MethodGet, "/api/v1/admin/books/moderation", nil)
	setAdminUser(c)
	if err := h.ListBookModerationQueue(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "failed to list book moderation queue") {
		t.Fatalf("expected 500, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestBookModerationQueueRoute_Auth(t *testing.T) {
	adminRole := middleware.RequireRole(usersDomain.RoleAdmin)
	users := []struct {
		name string
		user *usersDomain.User
		want int
	}{
		{"unauthenticated", nil, http.StatusUnauthorized},
		{"moderator", &usersDomain.User{Role: usersDomain.RoleModerator}, http.StatusForbidden},
		{"user", &usersDomain.User{Role: usersDomain.RoleUser}, http.StatusForbidden},
		{"admin", &usersDomain.User{Role: usersDomain.RoleAdmin}, http.StatusOK},
	}
	for _, u := range users {
		h, _ := setupHandler()
		c, rec := newContext(http.MethodGet, "/api/v1/admin/books/moderation", nil)
		if u.user != nil {
			c.Set("user", u.user)
		}
		if err := adminRole(h.ListBookModerationQueue)(c); err != nil {
			t.Fatalf("%s: unexpected error: %v", u.name, err)
		}
		if rec.Code != u.want {
			t.Fatalf("%s: expected %d, got %d", u.name, u.want, rec.Code)
		}
	}
}
