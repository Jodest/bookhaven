package middleware

import (
	"net/http"
	"strings"

	"github.com/bookhaven/backend/internal/modules/auth/service"
	"github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/modules/users/repository"
	"github.com/labstack/echo/v4"
)

// codeEmailNotVerified mirrors the auth transport's code (not imported to
// keep middleware independent of the auth handler package).
const codeEmailNotVerified = "email_not_verified"

// Auth creates an authentication middleware. Users whose email is not
// verified are not authenticated (401 email_not_verified).
func Auth(authService *service.Service, usersRepo repository.Repository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := extractToken(c)
			if token == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}

			userID, err := authService.ValidateAccessToken(token)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid or expired token"})
			}

			user, err := usersRepo.GetByID(c.Request().Context(), userID)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "user not found"})
			}
			if user.EmailVerifiedAt == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "email not verified", "code": codeEmailNotVerified})
			}

			c.Set("user_id", userID.String())
			c.Set("user", user)
			return next(c)
		}
	}
}

// OptionalAuth creates middleware that sets user if token is present, but doesn't require it.
// Users whose email is not verified are treated as anonymous.
func OptionalAuth(authService *service.Service, usersRepo repository.Repository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			token := extractToken(c)
			if token == "" {
				return next(c)
			}

			userID, err := authService.ValidateAccessToken(token)
			if err != nil {
				return next(c)
			}

			user, err := usersRepo.GetByID(c.Request().Context(), userID)
			if err == nil && user.EmailVerifiedAt != nil {
				c.Set("user_id", userID.String())
				c.Set("user", user)
			}

			return next(c)
		}
	}
}

// RequireRole creates middleware that requires a specific role.
func RequireRole(roles ...domain.Role) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get("user").(*domain.User)
			if !ok || user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}

			for _, role := range roles {
				if user.Role == role {
					return next(c)
				}
			}

			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
	}
}

// RequireBearerAuth ensures the request was authenticated with an
// Authorization: Bearer header. It must be used after the Auth middleware so
// the user is already validated; this middleware only verifies the token was
// presented as a bearer header to protect cookie-authenticated requests from
// CSRF on state-changing routes.
func RequireBearerAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			user, ok := c.Get("user").(*domain.User)
			if !ok || user == nil {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			}

			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "bearer token required"})
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") || parts[1] == "" {
				return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid authorization header"})
			}

			return next(c)
		}
	}
}

func extractToken(c echo.Context) string {
	// Try Authorization header
	authHeader := c.Request().Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
			return parts[1]
		}
	}

	// Try cookie
	cookie, err := c.Cookie("access_token")
	if err == nil && cookie != nil {
		return cookie.Value
	}

	return ""
}
