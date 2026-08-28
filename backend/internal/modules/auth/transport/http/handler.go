package http

import (
	"errors"
	"net/http"

	authDomain "github.com/bookhaven/backend/internal/modules/auth/domain"
	"github.com/bookhaven/backend/internal/modules/auth/service"
	"github.com/bookhaven/backend/internal/modules/users/domain"
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
	ID          string `json:"id"`
	Email       string `json:"email"`
	Name        string `json:"name"`
	Role        string `json:"role"`
	ProfileType string `json:"profile_type"`
}

func toUserResponse(u *domain.User) userResponse {
	name := ""
	if u.Name != nil {
		name = *u.Name
	}
	return userResponse{
		ID:          u.ID.String(),
		Email:       u.Email,
		Name:        name,
		Role:        string(u.Role),
		ProfileType: string(u.ProfileType),
	}
}

// Register handles user registration.
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

// Login handles user login.
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

// Refresh handles token refresh.
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

// Logout handles user logout.
func (h *Handler) Logout(c echo.Context) error {
	userID, ok := c.Get("user_id").(string)
	if !ok || userID == "" {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// Clear cookies
	clearAuthCookies(c)

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// Me returns current user info.
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
		MaxAge:   60 * 60 * 24 * 30, // 30 days
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
}


