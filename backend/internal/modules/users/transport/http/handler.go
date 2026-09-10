package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	listsDomain "github.com/bookhaven/backend/internal/modules/lists/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	usersService "github.com/bookhaven/backend/internal/modules/users/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// presentString distinguishes a JSON field that was omitted from one explicitly
// set to null or a string value.
type presentString struct {
	present bool
	value   *string
}

func (p *presentString) UnmarshalJSON(data []byte) error {
	p.present = true
	if string(data) == "null" {
		p.value = nil
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	p.value = &s
	return nil
}

func (p presentString) Present() bool  { return p.present }
func (p presentString) Value() *string { return p.value }

// Handler handles user profile HTTP requests.
type Handler struct {
	userService  UserService
	listsService ListService
}

// UserService provides user profile operations.
type UserService interface {
	GetByID(ctx context.Context, id uuid.UUID) (*usersDomain.User, error)
	UpdateMe(ctx context.Context, userID uuid.UUID, name, avatarURL, profileType *string) (*usersDomain.User, error)
}

// ListService provides public list operations for profile activity.
type ListService interface {
	ListPublicByUser(ctx context.Context, userID uuid.UUID) ([]listsDomain.List, error)
}

// NewHandler creates a new users Handler.
func NewHandler(userService UserService, listsService ListService) *Handler {
	return &Handler{
		userService:  userService,
		listsService: listsService,
	}
}

type updateProfileRequest struct {
	Name        *string       `json:"name"`
	AvatarURL   presentString `json:"avatar_url"`
	ProfileType *string       `json:"profile_type"`
}

type ownerUserResponse struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Name        *string    `json:"name"`
	AvatarURL   *string    `json:"avatar_url"`
	Role        string     `json:"role"`
	ProfileType string     `json:"profile_type"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type publicUserResponse struct {
	ID          string     `json:"id"`
	Name        *string    `json:"name"`
	AvatarURL   *string    `json:"avatar_url"`
	ProfileType string     `json:"profile_type"`
	JoinedAt    time.Time  `json:"joined_at"`
}

type publicListActivityResponse struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	IsPublic    bool      `json:"is_public"`
	BooksCount  int64     `json:"books_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type publicProfileResponse struct {
	User  publicUserResponse           `json:"user"`
	Lists []publicListActivityResponse `json:"lists"`
}

func toOwnerUserResponse(u *usersDomain.User) ownerUserResponse {
	return ownerUserResponse{
		ID:          u.ID.String(),
		Email:       u.Email,
		Name:        u.Name,
		AvatarURL:   u.AvatarURL,
		Role:        string(u.Role),
		ProfileType: string(u.ProfileType),
		CreatedAt:   u.CreatedAt,
		UpdatedAt:   u.UpdatedAt,
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

func parseUserID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

// GetUser godoc
// @Summary Публичный профиль пользователя
// @Tags users
// @Produce json
// @Param id path string true "ID пользователя"
// @Success 200 {object} publicProfileResponse
// @Failure 400 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/{id} [get]
func (h *Handler) GetUser(c echo.Context) error {
	ctx := c.Request().Context()

	id, err := parseUserID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}

	user, err := h.userService.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, usersService.ErrUserNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get user"})
	}

	lists, err := h.listsService.ListPublicByUser(ctx, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get public activity"})
	}

	activity := make([]publicListActivityResponse, 0, len(lists))
	for _, l := range lists {
		activity = append(activity, publicListActivityResponse{
			ID:          l.ID.String(),
			Title:       l.Title,
			Description: l.Description,
			IsPublic:    l.IsPublic,
			BooksCount:  l.BooksCount,
			CreatedAt:   l.CreatedAt,
			UpdatedAt:   l.UpdatedAt,
		})
	}

	return c.JSON(http.StatusOK, publicProfileResponse{
		User: publicUserResponse{
			ID:          user.ID.String(),
			Name:        user.Name,
			AvatarURL:   user.AvatarURL,
			ProfileType: string(user.ProfileType),
			JoinedAt:    user.CreatedAt,
		},
		Lists: activity,
	})
}

// UpdateMe godoc
// @Summary Обновить свой профиль
// @Tags users
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body updateProfileRequest true "Поля для обновления"
// @Success 200 {object} ownerUserResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /users/me [patch]
func (h *Handler) UpdateMe(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	var req updateProfileRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	var avatarURL *string
	if req.AvatarURL.Present() {
		if v := req.AvatarURL.Value(); v != nil {
			avatarURL = v
		} else {
			empty := ""
			avatarURL = &empty
		}
	}

	user, err := h.userService.UpdateMe(ctx, userID, req.Name, avatarURL, req.ProfileType)
	if err != nil {
		switch {
		case errors.Is(err, usersService.ErrUserNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
		case errors.Is(err, usersService.ErrInvalidName),
			errors.Is(err, usersService.ErrInvalidAvatarURL),
			errors.Is(err, usersService.ErrInvalidProfileType):
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to update profile"})
		}
	}

	return c.JSON(http.StatusOK, toOwnerUserResponse(user))
}
