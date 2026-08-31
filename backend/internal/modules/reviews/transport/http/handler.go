package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	reviewsDomain "github.com/bookhaven/backend/internal/modules/reviews/domain"
	"github.com/bookhaven/backend/internal/modules/reviews/service"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles reviews HTTP requests.
type Handler struct {
	service *service.Service
}

// NewHandler creates a new reviews Handler.
func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

type createReviewRequest struct {
	Rating  int    `json:"rating" validate:"required,min=1,max=5"`
	Content string `json:"content" validate:"required,min=1,max=10000"`
}

type updateReviewRequest struct {
	Rating  *int    `json:"rating" validate:"omitempty,min=1,max=5"`
	Content *string `json:"content" validate:"omitempty,min=1,max=10000"`
}

type reviewAuthorResponse struct {
	ID        string  `json:"id"`
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

type reviewResponse struct {
	ID         string               `json:"id"`
	BookID     string               `json:"book_id"`
	UserID     string               `json:"user_id"`
	User       reviewAuthorResponse `json:"user"`
	Rating     int                  `json:"rating"`
	Content    string               `json:"content"`
	LikesCount int                  `json:"likes_count"`
	LikedByMe  bool                 `json:"liked_by_me"`
	CreatedAt  time.Time            `json:"created_at"`
	UpdatedAt  time.Time            `json:"updated_at"`
}

type listReviewsResponse struct {
	Items    []reviewResponse `json:"items"`
	Total    int64            `json:"total"`
	Page     int              `json:"page"`
	Limit    int              `json:"limit"`
	MyReview *reviewResponse  `json:"my_review,omitempty"`
}

type toggleLikeResponse struct {
	LikesCount int  `json:"likes_count"`
	Liked      bool `json:"liked"`
}

func getUserID(c echo.Context) (uuid.UUID, error) {
	userIDStr, ok := c.Get("user_id").(string)
	if !ok || userIDStr == "" {
		return uuid.Nil, errors.New("unauthorized")
	}
	return uuid.Parse(userIDStr)
}

func getOptionalUserID(c echo.Context) *uuid.UUID {
	userIDStr, ok := c.Get("user_id").(string)
	if !ok || userIDStr == "" {
		return nil
	}
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil
	}
	return &userID
}

func toReviewResponse(r reviewsDomain.Review) reviewResponse {
	author := reviewAuthorResponse{ID: r.UserID.String()}
	if r.Author != nil {
		author.Name = r.Author.Name
		author.AvatarURL = r.Author.AvatarURL
	}
	return reviewResponse{
		ID:         r.ID.String(),
		BookID:     r.BookID.String(),
		UserID:     r.UserID.String(),
		User:       author,
		Rating:     r.Rating,
		Content:    r.Content,
		LikesCount: r.LikesCount,
		LikedByMe:  r.LikedByMe,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
}

func parseBookID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

func parseReviewID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

// ListReviews handles GET /api/v1/books/:id/reviews.
func (h *Handler) ListReviews(c echo.Context) error {
	ctx := c.Request().Context()

	bookID, err := parseBookID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	userID := getOptionalUserID(c)

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	result, err := h.service.ListReviews(ctx, bookID, userID, page, limit)
	if err != nil {
		if errors.Is(err, service.ErrBookNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list reviews"})
	}

	items := make([]reviewResponse, 0, len(result.Items))
	for _, r := range result.Items {
		items = append(items, toReviewResponse(r))
	}

	resp := listReviewsResponse{
		Items: items,
		Total: result.Total,
		Page:  result.Page,
		Limit: result.Limit,
	}
	if result.MyReview != nil {
		review := toReviewResponse(*result.MyReview)
		if currentUser, ok := c.Get("user").(*usersDomain.User); ok && currentUser != nil {
			review.User.Name = currentUser.Name
			review.User.AvatarURL = currentUser.AvatarURL
		}
		resp.MyReview = &review
	}

	return c.JSON(http.StatusOK, resp)
}

// CreateReview handles POST /api/v1/books/:id/reviews.
func (h *Handler) CreateReview(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	bookID, err := parseBookID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid book id"})
	}

	var req createReviewRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	review, err := h.service.CreateReview(ctx, userID, bookID, req.Rating, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRating), errors.Is(err, service.ErrInvalidContent):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, service.ErrBookNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "book not found"})
		case errors.Is(err, service.ErrAlreadyReviewed):
			return c.JSON(http.StatusConflict, map[string]string{"error": "already reviewed"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create review"})
		}
	}

	return c.JSON(http.StatusCreated, toReviewResponse(*review))
}

// UpdateReview handles PATCH /api/v1/reviews/:id.
func (h *Handler) UpdateReview(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	reviewID, err := parseReviewID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid review id"})
	}

	var req updateReviewRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	review, err := h.service.UpdateReview(ctx, userID, reviewID, req.Rating, req.Content)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidRating), errors.Is(err, service.ErrInvalidContent), errors.Is(err, service.ErrNothingToUpdate):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		case errors.Is(err, service.ErrReviewNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "review not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update review"})
		}
	}

	return c.JSON(http.StatusOK, toReviewResponse(*review))
}

// DeleteReview handles DELETE /api/v1/reviews/:id.
func (h *Handler) DeleteReview(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	reviewID, err := parseReviewID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid review id"})
	}

	if err := h.service.DeleteReview(ctx, userID, reviewID); err != nil {
		switch {
		case errors.Is(err, service.ErrReviewNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "review not found"})
		case errors.Is(err, service.ErrForbidden):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to delete review"})
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// ToggleLike handles POST /api/v1/reviews/:id/likes.
func (h *Handler) ToggleLike(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	reviewID, err := parseReviewID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid review id"})
	}

	likesCount, liked, err := h.service.ToggleLike(ctx, userID, reviewID)
	if err != nil {
		if errors.Is(err, service.ErrReviewNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "review not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to toggle like"})
	}

	return c.JSON(http.StatusOK, toggleLikeResponse{
		LikesCount: likesCount,
		Liked:      liked,
	})
}
