package middleware

import (
	"net/http"
	"strings"

	"github.com/bookhaven/backend/internal/modules/auth/service"
	"github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/modules/users/repository"
	"github.com/labstack/echo/v4"
)

// Auth creates an authentication middleware.
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

			c.Set("user_id", userID.String())
			c.Set("user", user)
			return next(c)
		}
	}
}

// OptionalAuth creates middleware that sets user if token is present, but doesn't require it.
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
			if err == nil {
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
