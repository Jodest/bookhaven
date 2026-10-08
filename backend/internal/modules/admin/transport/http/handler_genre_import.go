package http

import (
	"errors"
	"net/http"
	"strings"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/labstack/echo/v4"
)

// Genre import from Wikidata (docs/features/genre-wikidata-import.md).

// genreAliasRefResponse is the merged alias a search marker was found
// through.
type genreAliasRefResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Slug string `json:"slug"`
}

// genreExistingResponse is the "already in BookHaven" marker of a genre
// search candidate (QID match only): the canonical genre plus the alias
// carrying the QID, if any.
type genreExistingResponse struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Slug     string                 `json:"slug"`
	Status   string                 `json:"status"`
	ViaAlias *genreAliasRefResponse `json:"via_alias"`
}

// genreWikidataCandidateResponse is one candidate of
// GET /admin/genres/wikidata-search.
type genreWikidataCandidateResponse struct {
	QID         string `json:"qid"`
	Label       string `json:"label"`
	Description string `json:"description"`
	// RuLabel is the Russian label; null when absent or when class_check
	// is unavailable.
	RuLabel *string `json:"ru_label"`
	// LiteraryGenre is true for checked candidates, null when class_check
	// is unavailable.
	LiteraryGenre *bool                  `json:"literary_genre"`
	Existing      *genreExistingResponse `json:"existing"`
}

// genreWikidataSearchResponse is the 200 response of
// GET /admin/genres/wikidata-search.
type genreWikidataSearchResponse struct {
	// ClassCheck is ok | unavailable.
	ClassCheck string                           `json:"class_check"`
	Items      []genreWikidataCandidateResponse `json:"items"`
}

func toGenreWikidataSearchResponse(r adminService.GenreSearchResult) genreWikidataSearchResponse {
	items := make([]genreWikidataCandidateResponse, 0, len(r.Items))
	for _, it := range r.Items {
		item := genreWikidataCandidateResponse{
			QID:           it.QID,
			Label:         it.Label,
			Description:   it.Description,
			RuLabel:       it.RuLabel,
			LiteraryGenre: it.LiteraryGenre,
		}
		if it.Existing != nil && it.Existing.Genre != nil {
			g := it.Existing.Genre
			item.Existing = &genreExistingResponse{ID: g.ID.String(), Name: g.Name, Slug: g.Slug, Status: g.Status}
			if a := it.Existing.ViaAlias; a != nil {
				item.Existing.ViaAlias = &genreAliasRefResponse{ID: a.ID.String(), Name: a.Name, Slug: a.Slug}
			}
		}
		items = append(items, item)
	}
	return genreWikidataSearchResponse{ClassCheck: r.ClassCheck, Items: items}
}

// SearchWikidataGenres godoc
// @Summary Поиск литературных жанров в Wikidata (админ)
// @Description Требуется роль admin. Только чтение: ищет элементы Wikidata по названию (wbsearchentities, 20 кандидатов), одним SPARQL-запросом оставляет только литературные жанры и получает русское название, возвращает первые 10. Если проверку выполнить не удалось, class_check = unavailable, кандидаты не отфильтрованы, ru_label и literary_genre = null (импорт проверит жанр сам). Поле existing — жанр BookHaven с тем же QID (каноничный жанр; via_alias — слитый жанр, которому принадлежит QID) или null. Отсутствие совпадений — не ошибка (200, items: []).
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param q query string true "Название жанра"
// @Success 200 {object} genreWikidataSearchResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/genres/wikidata-search [get]
func (h *Handler) SearchWikidataGenres(c echo.Context) error {
	q := strings.TrimSpace(c.QueryParam("q"))
	if q == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "q is required"})
	}

	result, err := h.service.SearchWikidataGenres(c.Request().Context(), q)
	if err != nil {
		switch {
		case errors.Is(err, bookdata.ErrInvalidQuery):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid query"})
		case errors.Is(err, bookdata.ErrRateLimited), errors.Is(err, bookdata.ErrSourceUnavailable):
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "wikidata is unavailable; retry later"})
		case errors.Is(err, adminService.ErrGenreImportUnavailable):
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "genre import is not configured"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to search genres"})
		}
	}
	return c.JSON(http.StatusOK, toGenreWikidataSearchResponse(*result))
}

// importWikidataGenreRequest is the POST /admin/genres/import-wikidata body.
type importWikidataGenreRequest struct {
	QID string `json:"qid" example:"Q132311"`
}

// genreImportResponse is the POST /admin/genres/import-wikidata result.
type genreImportResponse struct {
	// Outcome is created | existing | rejected.
	Outcome string `json:"outcome"`
	// Match is qid | slug for existing/rejected, null for created.
	Match         *string `json:"match"`
	QIDBackfilled bool    `json:"qid_backfilled"`
	// QIDMismatch is the QID of a same-named genre that carries a
	// different QID (left untouched), null otherwise.
	QIDMismatch *string `json:"qid_mismatch"`
	// ViaAlias is the merged genre the match was followed through, null
	// for a direct match.
	ViaAlias *genreResponse `json:"via_alias"`
	// Genre is the canonical genre.
	Genre genreResponse `json:"genre"`
}

func toGenreImportResponse(r adminService.GenreImportResult) genreImportResponse {
	resp := genreImportResponse{
		Outcome:       r.Outcome,
		QIDBackfilled: r.QIDBackfilled,
		QIDMismatch:   r.QIDMismatch,
		Genre:         toGenreResponse(*r.Genre),
	}
	if r.Match != "" {
		match := r.Match
		resp.Match = &match
	}
	if r.ViaAlias != nil {
		alias := toGenreResponse(*r.ViaAlias)
		resp.ViaAlias = &alias
	}
	return resp
}

// ImportWikidataGenre godoc
// @Summary Импортировать литературный жанр из Wikidata по QID (админ)
// @Description Требуется роль admin и Bearer-токен. Сначала ищет жанр в БД по QID с учётом слитых жанров (без запроса к Wikidata), затем загружает элемент Wikidata (нужно русское название), проверяет, что это литературный жанр, и сопоставляет по правилам харвеста: совпадение по slug без QID — QID привязывается; с другим QID — существующий жанр не меняется (qid_mismatch); слитый жанр — возвращается каноничный (via_alias); иначе создаётся жанр в статусе pending с QID. Отклонённые жанры повторно не создаются (outcome=rejected). Название, slug и статус существующего жанра не меняются. 201 — создан, 200 — existing / rejected. 422 содержит code: no_ru_label | not_a_literary_genre.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body importWikidataGenreRequest true "QID элемента Wikidata"
// @Success 201 {object} genreImportResponse
// @Success 200 {object} genreImportResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 422 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Failure 502 {object} map[string]string
// @Failure 503 {object} map[string]string
// @Router /admin/genres/import-wikidata [post]
func (h *Handler) ImportWikidataGenre(c echo.Context) error {
	var req importWikidataGenreRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	result, err := h.service.ImportWikidataGenre(c.Request().Context(), req.QID)
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrInvalidQID):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid qid"})
		case errors.Is(err, adminService.ErrWikidataEntityNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "wikidata entity not found"})
		case errors.Is(err, adminService.ErrNoRussianLabel):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "wikidata entity has no russian label", "code": "no_ru_label"})
		case errors.Is(err, adminService.ErrNotALiteraryGenre):
			return c.JSON(http.StatusUnprocessableEntity, map[string]string{"error": "wikidata entity is not a literary genre", "code": "not_a_literary_genre"})
		case errors.Is(err, adminService.ErrDuplicateSlug), errors.Is(err, adminService.ErrDuplicateGenreQID):
			return c.JSON(http.StatusConflict, map[string]string{"error": "genre was created concurrently; retry"})
		case errors.Is(err, bookdata.ErrRateLimited), errors.Is(err, bookdata.ErrSourceUnavailable):
			return c.JSON(http.StatusBadGateway, map[string]string{"error": "wikidata is unavailable; retry later"})
		case errors.Is(err, adminService.ErrGenreImportUnavailable):
			return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "genre import is not configured"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to import genre"})
		}
	}

	status := http.StatusOK
	if result.Outcome == adminService.GenreOutcomeCreated {
		status = http.StatusCreated
	}
	return c.JSON(status, toGenreImportResponse(*result))
}
