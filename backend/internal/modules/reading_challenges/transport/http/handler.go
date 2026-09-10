package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/bookhaven/backend/internal/modules/reading_challenges/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles reading-challenges HTTP requests.
type Handler struct {
	service *service.Service
}

// NewHandler creates a new reading-challenges Handler.
func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

type upsertGoalRequest struct {
	Goal int `json:"goal" validate:"min=1,max=10000"`
}

type progressResponse struct {
	Year            int   `json:"year"`
	Goal            *int  `json:"goal"`
	BooksRead       int64 `json:"books_read"`
	ProgressPercent *int  `json:"progress_percent"`
	Completed       bool  `json:"completed"`
}

type listProgressResponse struct {
	Items []progressResponse `json:"items"`
	Total int                `json:"total"`
}

func toProgressResponse(p *service.Progress) progressResponse {
	return progressResponse{
		Year:            p.Year,
		Goal:            p.Goal,
		BooksRead:       p.BooksRead,
		ProgressPercent: p.ProgressPercent,
		Completed:       p.Completed,
	}
}

func getUserID(c echo.Context) (uuid.UUID, error) {
	userIDStr, ok := c.Get("user_id").(string)
	if !ok || userIDStr == "" {
		return uuid.Nil, errors.New("unauthorized")
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return uuid.Nil, errors.New("invalid user")
	}
	return userID, nil
}

func parseYear(c echo.Context) (int, error) {
	return strconv.Atoi(c.Param("year"))
}

// UpsertGoal godoc
// @Summary Установить/обновить цель читательского вызова на год
// @Tags reading-challenges
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param year path int true "Год"
// @Param body body upsertGoalRequest true "Цель по количеству книг"
// @Success 200 {object} progressResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reading-challenges/{year} [put]
func (h *Handler) UpsertGoal(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	year, err := parseYear(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid year"})
	}

	var req upsertGoalRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid goal"})
	}

	progress, err := h.service.UpsertGoal(ctx, userID, year, req.Goal)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidYear):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid year"})
		case errors.Is(err, service.ErrInvalidGoal):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid goal"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to save reading challenge"})
		}
	}

	return c.JSON(http.StatusOK, toProgressResponse(progress))
}

// GetChallenge godoc
// @Summary Прогресс читательского вызова за год
// @Tags reading-challenges
// @Produce json
// @Security BearerAuth
// @Param year path int true "Год"
// @Success 200 {object} progressResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reading-challenges/{year} [get]
func (h *Handler) GetChallenge(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	year, err := parseYear(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid year"})
	}

	progress, err := h.service.GetProgress(ctx, userID, year)
	if err != nil {
		if errors.Is(err, service.ErrInvalidYear) {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid year"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get reading challenge"})
	}

	return c.JSON(http.StatusOK, toProgressResponse(progress))
}

// ListChallenges godoc
// @Summary История читательского вызова по годам
// @Tags reading-challenges
// @Produce json
// @Security BearerAuth
// @Success 200 {object} listProgressResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /reading-challenges [get]
func (h *Handler) ListChallenges(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	history, err := h.service.ListHistory(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list reading challenges"})
	}

	items := make([]progressResponse, 0, len(history))
	for i := range history {
		items = append(items, toProgressResponse(&history[i]))
	}

	return c.JSON(http.StatusOK, listProgressResponse{Items: items, Total: len(items)})
}
