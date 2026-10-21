package http

import (
	"errors"
	"net/http"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

type verdictResponse struct {
	ScreenWorkID     string  `json:"screen_work_id"`
	VotesCount       int     `json:"votes_count"`
	BookBetter       *int    `json:"book_better"`
	AdaptationBetter *int    `json:"adaptation_better"`
	Same             *int    `json:"same"`
	MyVote           *string `json:"my_vote"`
}

type verdictsResponse struct {
	CanVote  bool              `json:"can_vote"`
	MinVotes int               `json:"min_votes"`
	Items    []verdictResponse `json:"items"`
}

type voteRequest struct {
	Value string `json:"value"`
}

func toVerdictResponse(v domain.Verdict) verdictResponse {
	return verdictResponse{
		ScreenWorkID: v.ScreenWorkID.String(), VotesCount: v.Votes,
		BookBetter: v.BookBetter, AdaptationBetter: v.AdaptationBetter, Same: v.Same, MyVote: v.MyVote,
	}
}

// currentUserID reads the user set by the auth middleware; false for a guest.
func currentUserID(c echo.Context) (uuid.UUID, bool) {
	raw, ok := c.Get("user_id").(string)
	if !ok || raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	return id, err == nil
}

// ListBookVerdicts godoc
// @Summary Вердикт читателей «книга или экранизация»
// @Description Доступно без авторизации (токен необязателен). Для каждой видимой экранизации опубликованной книги - число голосов; счётчики по значениям (book_better, adaptation_better, same) null, пока голосов меньше min_votes (5). my_vote - голос текущего пользователя, can_vote - true, если книга у него на полке «Прочитано». Порядок как у списка экранизаций. Неизвестная или неопубликованная книга - 404
// @Tags screen-works
// @Produce json
// @Param id path string true "ID книги"
// @Success 200 {object} verdictsResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id}/adaptations/verdicts [get]
func (h *Handler) ListBookVerdicts(c echo.Context) error {
	bookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	var userID *uuid.UUID
	if id, ok := currentUserID(c); ok {
		userID = &id
	}
	res, err := h.service.ListVerdicts(c.Request().Context(), bookID, userID)
	if err != nil {
		if errors.Is(err, domain.ErrBookNotFound) {
			return errJSON(c, http.StatusNotFound, "book not found")
		}
		return errJSON(c, http.StatusInternalServerError, "failed to get adaptation verdicts")
	}
	items := make([]verdictResponse, 0, len(res.Items))
	for _, v := range res.Items {
		items = append(items, toVerdictResponse(v))
	}
	return c.JSON(http.StatusOK, verdictsResponse{CanVote: res.CanVote, MinVotes: domain.MinVerdictVotes, Items: items})
}

// VoteAdaptation godoc
// @Summary Проголосовать «книга или экранизация»
// @Description Требуется авторизация. Создаёт или меняет голос текущего пользователя (value: book | adaptation | same). Голосовать могут только читатели, у которых книга на полке «Прочитано» (иначе 403). Возвращает вердикт этой экранизации. Скрытая или не связанная с книгой экранизация - 404
// @Tags screen-works
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Param screen_work_id path string true "ID экранизации"
// @Param body body voteRequest true "Голос"
// @Success 200 {object} verdictResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id}/adaptations/{screen_work_id}/vote [put]
func (h *Handler) VoteAdaptation(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return errJSON(c, http.StatusUnauthorized, "unauthorized")
	}
	bookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	workID, err := uuid.Parse(c.Param("screen_work_id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	var req voteRequest
	if err := c.Bind(&req); err != nil {
		return errJSON(c, http.StatusBadRequest, domain.ErrVoteValue.Error())
	}
	v, err := h.service.Vote(c.Request().Context(), userID, bookID, workID, req.Value)
	if err != nil {
		switch {
		case errors.Is(err, domain.ErrVoteValue):
			return errJSON(c, http.StatusBadRequest, domain.ErrVoteValue.Error())
		case errors.Is(err, domain.ErrBookNotFound):
			return errJSON(c, http.StatusNotFound, "book not found")
		case errors.Is(err, domain.ErrAdaptationNotFound):
			return errJSON(c, http.StatusNotFound, domain.ErrAdaptationNotFound.Error())
		case errors.Is(err, domain.ErrNotReader):
			return errJSON(c, http.StatusForbidden, domain.ErrNotReader.Error())
		default:
			return errJSON(c, http.StatusInternalServerError, "failed to save vote")
		}
	}
	return c.JSON(http.StatusOK, toVerdictResponse(*v))
}

// UnvoteAdaptation godoc
// @Summary Убрать свой голос «книга или экранизация»
// @Description Требуется авторизация. Удаляет голос текущего пользователя; без голоса тоже 204. Полку, связь и статус книги не проверяет
// @Tags screen-works
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Param screen_work_id path string true "ID экранизации"
// @Success 204
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id}/adaptations/{screen_work_id}/vote [delete]
func (h *Handler) UnvoteAdaptation(c echo.Context) error {
	userID, ok := currentUserID(c)
	if !ok {
		return errJSON(c, http.StatusUnauthorized, "unauthorized")
	}
	bookID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid book id")
	}
	workID, err := uuid.Parse(c.Param("screen_work_id"))
	if err != nil {
		return errJSON(c, http.StatusBadRequest, "invalid screen work id")
	}
	if err := h.service.Unvote(c.Request().Context(), userID, bookID, workID); err != nil {
		return errJSON(c, http.StatusInternalServerError, "failed to remove vote")
	}
	return c.NoContent(http.StatusNoContent)
}
