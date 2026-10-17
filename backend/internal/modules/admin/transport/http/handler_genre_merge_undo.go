package http

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/bookhaven/backend/internal/modules/admin/repository"
	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// genreMergeUserRef is the admin who merged or undid a merge.
type genreMergeUserRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// genreMergeGenreRef is a genre as recorded at merge time.
type genreMergeGenreRef struct {
	ID   uuid.UUID `json:"id"`
	Name string    `json:"name"`
}

// genreMergeEntryResponse is one merge journal entry.
type genreMergeEntryResponse struct {
	ID                  uuid.UUID            `json:"id"`
	CreatedAt           time.Time            `json:"created_at"`
	PerformedBy         *genreMergeUserRef   `json:"performed_by"`
	Target              genreMergeGenreRef   `json:"target"`
	Sources             []genreMergeGenreRef `json:"sources"`
	BooksReassigned     int                  `json:"books_reassigned"`
	CollectionsSwitched int                  `json:"collections_switched"`
	UndoneAt            *time.Time           `json:"undone_at"`
	UndoneBy            *genreMergeUserRef   `json:"undone_by"`
}

// genreMergeListResponse is one page of the merge history.
type genreMergeListResponse struct {
	Items []genreMergeEntryResponse `json:"items"`
	Total int64                     `json:"total"`
	Page  int                       `json:"page"`
	Limit int                       `json:"limit"`
}

// genreMergeUndoPreviewResponse is the read-only summary of an undo.
type genreMergeUndoPreviewResponse struct {
	Merge         genreMergeEntryResponse `json:"merge"`
	BooksReturned int64                   `json:"books_returned"`
	BooksRelinked int64                   `json:"books_relinked"`
	Collections   []genreMergeCollection  `json:"collections"`
}

// genreMergeUndoResponse is the result of an undo.
type genreMergeUndoResponse struct {
	Genre                   genreResponse `json:"genre"`
	SourcesRestored         int           `json:"sources_restored"`
	BooksReturned           int64         `json:"books_returned"`
	BooksRelinked           int64         `json:"books_relinked"`
	CollectionsSwitchedBack int           `json:"collections_switched_back"`
}

// genreMergeUndoBlockedResponse is the 409 body of a blocked undo.
type genreMergeUndoBlockedResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// requestActorID returns the authenticated user's id set by the auth
// middleware.
func requestActorID(c echo.Context) (uuid.UUID, bool) {
	raw, ok := c.Get("user_id").(string)
	if !ok {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

// withRequestActor stores the authenticated user in ctx for the merge
// journal; without an authenticated user the entry has no performer.
func withRequestActor(c echo.Context, ctx context.Context) context.Context {
	if id, ok := requestActorID(c); ok {
		return adminService.WithActor(ctx, id)
	}
	return ctx
}

func toGenreMergeUserRef(u *repository.GenreMergeUserRef) *genreMergeUserRef {
	if u == nil {
		return nil
	}
	return &genreMergeUserRef{ID: u.ID, Name: u.Name}
}

func toGenreMergeEntryResponse(e repository.GenreMergeEntry) genreMergeEntryResponse {
	sources := make([]genreMergeGenreRef, 0, len(e.Sources))
	for _, s := range e.Sources {
		sources = append(sources, genreMergeGenreRef{ID: s.ID, Name: s.Name})
	}
	return genreMergeEntryResponse{
		ID:                  e.ID,
		CreatedAt:           e.CreatedAt,
		PerformedBy:         toGenreMergeUserRef(e.PerformedBy),
		Target:              genreMergeGenreRef{ID: e.Target.ID, Name: e.Target.Name},
		Sources:             sources,
		BooksReassigned:     e.BooksReassigned,
		CollectionsSwitched: e.CollectionsSwitched,
		UndoneAt:            e.UndoneAt,
		UndoneBy:            toGenreMergeUserRef(e.UndoneBy),
	}
}

// writeGenreMergeUndoError maps the journal service errors to 404/409/500.
func writeGenreMergeUndoError(c echo.Context, err error, failure string) error {
	var blocked *adminService.GenreMergeUndoBlockedError
	switch {
	case errors.Is(err, adminService.ErrGenreMergeNotFound):
		return c.JSON(http.StatusNotFound, map[string]string{"error": "genre merge not found"})
	case errors.As(err, &blocked):
		return c.JSON(http.StatusConflict, genreMergeUndoBlockedResponse{Error: "genre merge cannot be undone", Code: blocked.Code})
	default:
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": failure})
	}
}

// ListGenreMerges godoc
// @Summary История слияний жанров (админ)
// @Description Требуется роль admin и Bearer-токен. Журнал слияний (одиночных и пакетных) от новых к старым; пакетное слияние — одна запись. Слияния, выполненные до появления журнала, в списке отсутствуют и не отменяются. limit по умолчанию 20, максимум 100.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param page query int false "Номер страницы" default(1)
// @Param limit query int false "Размер страницы" default(20)
// @Success 200 {object} genreMergeListResponse
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/genres/merges [get]
func (h *Handler) ListGenreMerges(c echo.Context) error {
	page, limit := parsePagination(c)
	result, err := h.service.ListGenreMerges(c.Request().Context(), page, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list genre merges"})
	}
	items := make([]genreMergeEntryResponse, 0, len(result.Items))
	for _, e := range result.Items {
		items = append(items, toGenreMergeEntryResponse(e))
	}
	return c.JSON(http.StatusOK, genreMergeListResponse{Items: items, Total: result.Total, Page: result.Page, Limit: result.Limit})
}

// PreviewGenreMergeUndo godoc
// @Summary Предпросмотр отмены слияния жанров (админ)
// @Description Требуется роль admin и Bearer-токен. Только чтение, ничего не меняет. Возвращает запись журнала, число книг, которые вернутся в исходные жанры (books_returned) или снова получат исходный жанр, оставаясь в целевом (books_relinked), и подборки, которые вернутся к исходным жанрам. Если отмена невозможна — 409 с кодом (already_undone, target_missing, target_merged, source_missing, source_not_alias, qid_conflict).
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID записи журнала слияний"
// @Success 200 {object} genreMergeUndoPreviewResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} genreMergeUndoBlockedResponse
// @Failure 500 {object} map[string]string
// @Router /admin/genres/merges/{id}/undo-preview [get]
func (h *Handler) PreviewGenreMergeUndo(c echo.Context) error {
	mergeID, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid merge id"})
	}
	preview, err := h.service.PreviewGenreMergeUndo(c.Request().Context(), mergeID)
	if err != nil {
		return writeGenreMergeUndoError(c, err, "failed to preview genre merge undo")
	}
	collections := make([]genreMergeCollection, 0, len(preview.Collections))
	for _, col := range preview.Collections {
		collections = append(collections, genreMergeCollection{
			ID:          col.ID,
			Title:       col.Title,
			Slug:        col.Slug,
			Kind:        col.Kind,
			IsPublished: col.IsPublished,
			SourceGenre: genreMergeCollectionSource{ID: col.SourceGenreID, Name: col.SourceGenreName},
		})
	}
	return c.JSON(http.StatusOK, genreMergeUndoPreviewResponse{
		Merge:         toGenreMergeEntryResponse(preview.Merge),
		BooksReturned: preview.BooksReturned,
		BooksRelinked: preview.BooksRelinked,
		Collections:   collections,
	})
}

// UndoGenreMerge godoc
// @Summary Отменить слияние жанров (админ)
// @Description Требуется роль admin и Bearer-токен. Без тела. Целиком (все жанры пакетного слияния) в одной транзакции возвращает записанное состояние: исходные жанры снова обычные, с прежним статусом и QID, их книги, алиасы и подборки возвращаются; всё, что изменилось после слияния (удалённые книги и подборки, снятые связи, переназначенные подборки), пропускается. Автоматические подборки пересчитываются после отмены. Если отмена небезопасна — 409 с кодом (already_undone, target_missing, target_merged, source_missing, source_not_alias, qid_conflict) и ничего не меняется. Слияния без записи в журнале (до появления журнала) отменить нельзя.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID записи журнала слияний"
// @Success 200 {object} genreMergeUndoResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} genreMergeUndoBlockedResponse
// @Failure 500 {object} map[string]string
// @Router /admin/genres/merges/{id}/undo [post]
func (h *Handler) UndoGenreMerge(c echo.Context) error {
	actorID, ok := requestActorID(c)
	if !ok {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	mergeID, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid merge id"})
	}
	outcome, err := h.service.UndoGenreMerge(c.Request().Context(), mergeID, actorID)
	if err != nil {
		return writeGenreMergeUndoError(c, err, "failed to undo genre merge")
	}
	return c.JSON(http.StatusOK, genreMergeUndoResponse{
		Genre:                   toGenreResponse(*outcome.Genre),
		SourcesRestored:         outcome.SourcesRestored,
		BooksReturned:           outcome.BooksReturned,
		BooksRelinked:           outcome.BooksRelinked,
		CollectionsSwitchedBack: outcome.CollectionsSwitchedBack,
	})
}
