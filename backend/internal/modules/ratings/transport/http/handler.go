package http

import (
	"errors"
	"net/http"

	"github.com/bookhaven/backend/internal/modules/ratings/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles ratings HTTP requests.
type Handler struct {
	service *service.Service
}

// NewHandler creates a new ratings Handler.
func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

type setRatingRequest struct {
	Value int `json:"value" validate:"required,min=1,max=5"`
}

type setRatingResponse struct {
	BookID       string  `json:"book_id"`
	Value        int     `json:"value"`
	Rating       float64 `json:"rating"`
	RatingsCount int     `json:"ratings_count"`
}

type myRatingResponse struct {
	Value int `json:"value"`
}

func getUserID(c echo.Context) (uuid.UUID, error) {
	userIDStr, ok := c.Get("user_id").(string)
	if !ok || userIDStr == "" {
		return uuid.Nil, errors.New("unauthorized")
	}
	return uuid.Parse(userIDStr)
}

func parseBookID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

// SetRating godoc
// @Summary Поставить/обновить оценку книге
// @Tags ratings
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Param body body setRatingRequest true "Оценка от 1 до 5"
// @Success 200 {object} setRatingResponse
// @Success 201 {object} setRatingResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id}/ratings [post]
func (h *Handler) SetRating(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	bookID, err := parseBookID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	var req setRatingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	result, err := h.service.SetRating(ctx, userID, bookID, req.Value)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidValue):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid rating value"})
		case errors.Is(err, service.ErrBookNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to set rating"})
		}
	}

	status := http.StatusOK
	if result.Created {
		status = http.StatusCreated
	}

	return c.JSON(status, setRatingResponse{
		BookID:       bookID.String(),
		Value:        result.Value,
		Rating:       result.Rating,
		RatingsCount: result.RatingsCount,
	})
}

// GetMyRating godoc
// @Summary Получить свою оценку книге
// @Tags ratings
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID книги"
// @Success 200 {object} myRatingResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /books/{id}/ratings/me [get]
func (h *Handler) GetMyRating(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	bookID, err := parseBookID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	value, err := h.service.GetMyRating(ctx, userID, bookID)
	if err != nil {
		if errors.Is(err, service.ErrRatingNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "rating not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get rating"})
	}

	return c.JSON(http.StatusOK, myRatingResponse{Value: value})
}
