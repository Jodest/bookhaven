package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/transport/http/middleware"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Genre merge journal: history, undo preview and undo.

func (f *fakeAdminService) ListGenreMerges(ctx context.Context, page, limit int) (*adminService.GenreMergePage, error) {
	f.mergeListPage, f.mergeListLimit = page, limit
	if f.mergeListResult != nil {
		return f.mergeListResult, nil
	}
	return &adminService.GenreMergePage{Page: page, Limit: limit}, nil
}

func (f *fakeAdminService) PreviewGenreMergeUndo(ctx context.Context, mergeID uuid.UUID) (*repository.GenreMergeUndoPreview, error) {
	if f.mergeUndoErr != nil {
		return nil, f.mergeUndoErr
	}
	if f.mergeUndoPreview != nil {
		return f.mergeUndoPreview, nil
	}
	return &repository.GenreMergeUndoPreview{}, nil
}

func (f *fakeAdminService) UndoGenreMerge(ctx context.Context, mergeID, actorID uuid.UUID) (*adminService.GenreMergeUndoOutcome, error) {
	f.mergeUndoActor = actorID
	if f.mergeUndoErr != nil {
		return nil, f.mergeUndoErr
	}
	if f.mergeUndoOutcome != nil {
		return f.mergeUndoOutcome, nil
	}
	return &adminService.GenreMergeUndoOutcome{Genre: &catalogDomain.Genre{ID: uuid.New(), Name: "роман", Slug: "roman", Status: "published"}}, nil
}

func newUndoContext(method, path, id string, actor *uuid.UUID) (echo.Context, func() (int, []byte)) {
	c, rec := newContext(method, path, nil)
	c.SetParamNames("id")
	c.SetParamValues(id)
	setAdminUser(c)
	if actor != nil {
		c.Set("user_id", actor.String())
	}
	return c, func() (int, []byte) { return rec.Code, rec.Body.Bytes() }
}

func TestListGenreMerges_Shape(t *testing.T) {
	h, svc := setupHandler()
	mergeID, target, source, admin := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	created := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	svc.mergeListResult = &adminService.GenreMergePage{
		Items: []repository.GenreMergeEntry{{
			ID: mergeID, CreatedAt: created,
			PerformedBy:     &repository.GenreMergeUserRef{ID: admin, Name: "Админ"},
			Target:          repository.GenreMergeGenreRef{ID: target, Name: "роман"},
			Sources:         []repository.GenreMergeGenreRef{{ID: source, Name: "романы"}},
			BooksReassigned: 4, CollectionsSwitched: 1,
		}},
		Total: 1, Page: 2, Limit: 10,
	}
	c, rec := newContext(http.MethodGet, "/api/v1/admin/genres/merges?page=2&limit=10", nil)
	setAdminUser(c)
	if err := h.ListGenreMerges(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if svc.mergeListPage != 2 || svc.mergeListLimit != 10 {
		t.Fatalf("unexpected paging: %d/%d", svc.mergeListPage, svc.mergeListLimit)
	}
	var body struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
		Page  int              `json:"page"`
		Limit int              `json:"limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Total != 1 || body.Page != 2 || body.Limit != 10 || len(body.Items) != 1 {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
	item := body.Items[0]
	if item["id"] != mergeID.String() || item["books_reassigned"] != float64(4) || item["collections_switched"] != float64(1) ||
		item["undone_at"] != nil || item["undone_by"] != nil {
		t.Fatalf("unexpected item: %v", item)
	}
	if by := item["performed_by"].(map[string]any); by["id"] != admin.String() || by["name"] != "Админ" {
		t.Fatalf("unexpected performed_by: %v", by)
	}
	if tg := item["target"].(map[string]any); tg["id"] != target.String() || tg["name"] != "роман" {
		t.Fatalf("unexpected target: %v", tg)
	}
	if srcs := item["sources"].([]any); len(srcs) != 1 || srcs[0].(map[string]any)["name"] != "романы" {
		t.Fatalf("unexpected sources: %v", item["sources"])
	}
}

func TestListGenreMerges_EmptyIsArray(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/admin/genres/merges", nil)
	setAdminUser(c)
	if err := h.ListGenreMerges(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if items, ok := body["items"].([]any); !ok || len(items) != 0 {
		t.Fatalf("items must be an empty array: %s", rec.Body.String())
	}
}

func TestPreviewGenreMergeUndo_Shape(t *testing.T) {
	h, svc := setupHandler()
	mergeID, source, colID := uuid.New(), uuid.New(), uuid.New()
	svc.mergeUndoPreview = &repository.GenreMergeUndoPreview{
		Merge:         repository.GenreMergeEntry{ID: mergeID, Target: repository.GenreMergeGenreRef{ID: uuid.New(), Name: "роман"}},
		BooksReturned: 3, BooksRelinked: 1,
		Collections: []repository.GenreMergeCollection{{ID: colID, Title: "Лучшие романы", Slug: "best", Kind: "manual", IsPublished: true, SourceGenreID: source, SourceGenreName: "романы"}},
	}
	c, body := newUndoContext(http.MethodGet, "/x", mergeID.String(), nil)
	if err := h.PreviewGenreMergeUndo(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	code, raw := body()
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", code, raw)
	}
	var got struct {
		Merge         map[string]any `json:"merge"`
		BooksReturned int64          `json:"books_returned"`
		BooksRelinked int64          `json:"books_relinked"`
		Collections   []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			SourceGenre struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"source_genre"`
		} `json:"collections"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Merge["id"] != mergeID.String() || got.BooksReturned != 3 || got.BooksRelinked != 1 || len(got.Collections) != 1 ||
		got.Collections[0].Title != "Лучшие романы" || got.Collections[0].SourceGenre.Name != "романы" || got.Collections[0].SourceGenre.ID != source.String() {
		t.Fatalf("unexpected body: %s", raw)
	}
}

type mergeUndoBody struct {
	Genre                   genreResponse `json:"genre"`
	SourcesRestored         int           `json:"sources_restored"`
	BooksReturned           int64         `json:"books_returned"`
	BooksRelinked           int64         `json:"books_relinked"`
	CollectionsSwitchedBack int           `json:"collections_switched_back"`
}

func TestUndoGenreMerge_Success(t *testing.T) {
	h, svc := setupHandler()
	mergeID, target, actor := uuid.New(), uuid.New(), uuid.New()
	svc.mergeUndoOutcome = &adminService.GenreMergeUndoOutcome{
		Genre:           &catalogDomain.Genre{ID: target, Name: "роман", Slug: "roman", Status: "published"},
		SourcesRestored: 2, BooksReturned: 4, BooksRelinked: 1, CollectionsSwitchedBack: 3,
	}
	c, body := newUndoContext(http.MethodPost, "/x", mergeID.String(), &actor)
	if err := h.UndoGenreMerge(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	code, raw := body()
	if code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", code, raw)
	}
	if svc.mergeUndoActor != actor {
		t.Fatalf("actor = %s, want %s", svc.mergeUndoActor, actor)
	}
	var got mergeUndoBody
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Genre.ID != target.String() || got.SourcesRestored != 2 || got.BooksReturned != 4 || got.BooksRelinked != 1 || got.CollectionsSwitchedBack != 3 {
		t.Fatalf("unexpected body: %s", raw)
	}
}

func TestGenreMergeUndo_Errors(t *testing.T) {
	actor := uuid.New()
	cases := []struct {
		name string
		id   string
		err  error
		want int
		code string
	}{
		{"bad id", "nope", nil, http.StatusBadRequest, ""},
		{"not found", uuid.NewString(), adminService.ErrGenreMergeNotFound, http.StatusNotFound, ""},
		{"blocked", uuid.NewString(), &adminService.GenreMergeUndoBlockedError{Code: "target_merged"}, http.StatusConflict, "target_merged"},
		{"internal", uuid.NewString(), fmt.Errorf("boom"), http.StatusInternalServerError, ""},
	}
	for _, tc := range cases {
		for name, call := range map[string]func(*Handler, echo.Context) error{
			"preview": (*Handler).PreviewGenreMergeUndo,
			"undo":    (*Handler).UndoGenreMerge,
		} {
			h, svc := setupHandler()
			svc.mergeUndoErr = tc.err
			c, body := newUndoContext(http.MethodPost, "/x", tc.id, &actor)
			if err := call(h, c); err != nil {
				t.Fatalf("%s/%s: unexpected error: %v", tc.name, name, err)
			}
			code, raw := body()
			if code != tc.want {
				t.Fatalf("%s/%s: expected %d, got %d: %s", tc.name, name, tc.want, code, raw)
			}
			if tc.code != "" {
				var got map[string]string
				if err := json.Unmarshal(raw, &got); err != nil || got["code"] != tc.code || got["error"] != "genre merge cannot be undone" {
					t.Fatalf("%s/%s: unexpected 409 body: %s", tc.name, name, raw)
				}
			}
		}
	}
}

func TestUndoGenreMerge_NoActorIs401(t *testing.T) {
	h, _ := setupHandler()
	c, body := newUndoContext(http.MethodPost, "/x", uuid.NewString(), nil)
	if err := h.UndoGenreMerge(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if code, _ := body(); code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", code)
	}
}

func TestMergeGenres_ResponseCarriesMergeID(t *testing.T) {
	h, svc := setupHandler()
	source, target, mergeID := uuid.New(), uuid.New(), uuid.New()
	svc.mergeGenresID = mergeID
	c, rec := newContext(http.MethodPost, "/x", []byte(fmt.Sprintf(`{"canonical_id":%q}`, target)))
	c.SetParamNames("id")
	c.SetParamValues(source.String())
	setAdminUser(c)
	if err := h.MergeGenres(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body mergeGenreResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.MergeID != mergeID {
		t.Fatalf("merge_id = %v (%v): %s", body.MergeID, err, rec.Body.String())
	}
}

func TestMergeGenresBatch_ResponseCarriesMergeID(t *testing.T) {
	h, svc := setupHandler()
	a, target, mergeID := uuid.New(), uuid.New(), uuid.New()
	svc.mergeBatchOutcome = &adminService.GenreBatchMergeOutcome{
		Genre:         &catalogDomain.Genre{ID: target, Name: "роман", Slug: "roman", Status: "published"},
		SourcesMerged: 1, MergeID: mergeID,
	}
	c, rec := newContext(http.MethodPost, "/x", []byte(fmt.Sprintf(`{"source_ids":[%q],"target_id":%q}`, a, target)))
	setAdminUser(c)
	if err := h.MergeGenresBatch(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body mergeGenreBatchResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.MergeID != mergeID {
		t.Fatalf("merge_id = %v (%v): %s", body.MergeID, err, rec.Body.String())
	}
}

// TestGenreMergeJournalRoutes_Auth wires the journal handlers with the
// router.go chain (adminRole + requireBearer on every route).
func TestGenreMergeJournalRoutes_Auth(t *testing.T) {
	adminRole := middleware.RequireRole(usersDomain.RoleAdmin)
	requireBearer := middleware.RequireBearerAuth()
	id := uuid.NewString()
	routes := []struct {
		name   string
		method string
		next   func(h *Handler) echo.HandlerFunc
	}{
		{"list", http.MethodGet, func(h *Handler) echo.HandlerFunc { return adminRole(requireBearer(h.ListGenreMerges)) }},
		{"undo-preview", http.MethodGet, func(h *Handler) echo.HandlerFunc { return adminRole(requireBearer(h.PreviewGenreMergeUndo)) }},
		{"undo", http.MethodPost, func(h *Handler) echo.HandlerFunc { return adminRole(requireBearer(h.UndoGenreMerge)) }},
	}
	users := []struct {
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
	for _, r := range routes {
		for _, u := range users {
			h, _ := setupHandler()
			c, rec := newContext(r.method, "/x", nil)
			c.SetParamNames("id")
			c.SetParamValues(id)
			if u.user != nil {
				c.Set("user", u.user)
				c.Set("user_id", uuid.NewString())
			}
			if u.bearer {
				c.Request().Header.Set("Authorization", "Bearer token")
			}
			if err := r.next(h)(c); err != nil {
				t.Fatalf("%s/%s: unexpected error: %v", r.name, u.name, err)
			}
			if rec.Code != u.want {
				t.Fatalf("%s/%s: expected %d, got %d", r.name, u.name, u.want, rec.Code)
			}
		}
	}
}
