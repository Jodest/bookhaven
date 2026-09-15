package http

import (
	"errors"
	"net/http"
	"time"

	authDomain "github.com/bookhaven/backend/internal/modules/auth/domain"
	"github.com/bookhaven/backend/internal/modules/auth/service"
	"github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles authentication HTTP requests.
type Handler struct {
	service *service.Service
}

// NewHandler creates a new auth Handler.
func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

type registerRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required,min=2"`
}

type loginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type authResponse struct {
	User         userResponse `json:"user"`
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
}

type userResponse struct {
	ID          string     `json:"id"`
	Email       string     `json:"email"`
	Name        *string    `json:"name"`
	AvatarURL   *string    `json:"avatar_url"`
	Role        string     `json:"role"`
	ProfileType string     `json:"profile_type"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func toUserResponse(u *domain.User) userResponse {
	return userResponse{
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

// Register godoc
// @Summary Регистрация пользователя
// @Description Создаёт пользователя и сразу выполняет вход, устанавливая auth-cookies
// @Tags auth
// @Accept json
// @Produce json
// @Param body body registerRequest true "Данные регистрации"
// @Success 201 {object} authResponse
// @Failure 400 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/register [post]
func (h *Handler) Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	user, err := h.service.Register(c.Request().Context(), req.Email, req.Password, req.Name)
	if err != nil {
		if errors.Is(err, service.ErrUserExists) {
			return c.JSON(http.StatusConflict, map[string]string{"error": "user already exists"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to register"})
	}

	// Auto-login after registration
	_, tokens, err := h.service.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to create session"})
	}

	setAuthCookies(c, tokens)

	return c.JSON(http.StatusCreated, authResponse{
		User:         toUserResponse(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

// Login godoc
// @Summary Вход пользователя
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "Учётные данные"
// @Success 200 {object} authResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/login [post]
func (h *Handler) Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := c.Validate(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
	}

	user, tokens, err := h.service.Login(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to login"})
	}

	setAuthCookies(c, tokens)

	return c.JSON(http.StatusOK, authResponse{
		User:         toUserResponse(user),
		AccessToken:  tokens.AccessToken,
		RefreshToken: tokens.RefreshToken,
		ExpiresIn:    tokens.ExpiresIn,
	})
}

// Refresh godoc
// @Summary Обновить пару токенов
// @Description Принимает refresh-токен в теле запроса или в cookie refresh_token
// @Tags auth
// @Accept json
// @Produce json
// @Param body body refreshRequest false "Refresh-токен (опционально, можно передать через cookie)"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Router /auth/refresh [post]
func (h *Handler) Refresh(c echo.Context) error {
	var req refreshRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Try refresh token from cookie first
	refreshToken := req.RefreshToken
	if refreshToken == "" {
		cookie, err := c.Cookie("refresh_token")
		if err == nil && cookie != nil {
			refreshToken = cookie.Value
		}
	}

	if refreshToken == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "refresh token required"})
	}

	tokens, err := h.service.RefreshTokens(c.Request().Context(), refreshToken)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired refresh token"})
	}

	setAuthCookies(c, tokens)

	return c.JSON(http.StatusOK, tokens)
}

// Logout godoc
// @Summary Выход пользователя
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /auth/logout [post]
func (h *Handler) Logout(c echo.Context) error {
	userIDStr, ok := c.Get("user_id").(string)
	if !ok || userIDStr == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid user"})
	}

	if err := h.service.Logout(c.Request().Context(), userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to logout"})
	}

	// Clear cookies
	clearAuthCookies(c)

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// Me godoc
// @Summary Текущий пользователь
// @Tags auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} userResponse
// @Failure 401 {object} map[string]string
// @Router /auth/me [get]
func (h *Handler) Me(c echo.Context) error {
	user, ok := c.Get("user").(*domain.User)
	if !ok || user == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	return c.JSON(http.StatusOK, toUserResponse(user))
}

func setAuthCookies(c echo.Context, tokens *authDomain.TokenPair) {
	if tokens == nil {
		return
	}
	c.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    tokens.AccessToken,
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   int(tokens.ExpiresIn),
	})
	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    tokens.RefreshToken,
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
		Path:     "/api/v1/auth/refresh",
		MaxAge:   60 * 60 * 24 * 30, // 30 days -- keep in sync with has_session's MaxAge below
	})
	// has_session is a non-sensitive marker cookie: it carries no authorization
	// value and is never validated server-side. It exists purely so that
	// Edge middleware can tell "maybe still logged in, let the client resolve
	// it" apart from "definitely logged out" without being able to read the
	// HttpOnly refresh_token (whose Path is restricted to the refresh endpoint).
	// It must always be set/cleared alongside refresh_token with the exact same
	// MaxAge, so the two can never diverge in the browser.
	c.SetCookie(&http.Cookie{
		Name:     "has_session",
		Value:    "1",
		HttpOnly: true, // no legitimate reason for client JS to read/write it
		Secure:   false, // matches existing cookies; set true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
		MaxAge:   60 * 60 * 24 * 30, // 30 days -- must always match refresh_token's MaxAge above
	})
}

func clearAuthCookies(c echo.Context) {
	c.SetCookie(&http.Cookie{
		Name:     "access_token",
		Value:    "",
		HttpOnly: true,
		Path:     "/",
		MaxAge:   -1,
	})
	c.SetCookie(&http.Cookie{
		Name:     "refresh_token",
		Value:    "",
		HttpOnly: true,
		Path:     "/api/v1/auth/refresh",
		MaxAge:   -1,
	})
	c.SetCookie(&http.Cookie{
		Name:     "has_session",
		Value:    "",
		HttpOnly: true,
		Path:     "/",
		MaxAge:   -1,
	})
}


