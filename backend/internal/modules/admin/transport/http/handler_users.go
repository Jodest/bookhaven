package http

import (
	"errors"
	"net/http"
	"time"

	adminService "github.com/bookhaven/backend/internal/modules/admin/service"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/labstack/echo/v4"
)

// adminUserResponse is a user as seen by the admin users list. It never
// carries password_hash.
type adminUserResponse struct {
	ID              string     `json:"id"`
	Email           string     `json:"email"`
	Name            *string    `json:"name"`
	AvatarURL       *string    `json:"avatar_url"`
	Role            string     `json:"role"`
	ProfileType     string     `json:"profile_type"`
	EmailVerifiedAt *time.Time `json:"email_verified_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type listUsersResponse struct {
	Items []adminUserResponse `json:"items"`
	Total int64               `json:"total"`
	Page  int                 `json:"page"`
	Limit int                 `json:"limit"`
}

// changeUserRoleRequest is the PATCH /admin/users/:id/role body.
type changeUserRoleRequest struct {
	// Role is one of user | contributor | moderator | admin.
	Role string `json:"role"`
}

func toAdminUserResponse(u usersDomain.User) adminUserResponse {
	return adminUserResponse{
		ID:              u.ID.String(),
		Email:           u.Email,
		Name:            u.Name,
		AvatarURL:       u.AvatarURL,
		Role:            string(u.Role),
		ProfileType:     string(u.ProfileType),
		EmailVerifiedAt: u.EmailVerifiedAt,
		CreatedAt:       u.CreatedAt,
	}
}

func invalidRoleResponse(c echo.Context) error {
	return c.JSON(http.StatusBadRequest, map[string]string{"error": "role is invalid", "code": "invalid_role"})
}

// ListUsers godoc
// @Summary Список пользователей (админ)
// @Description Требуется роль admin. Поиск по email или имени (подстрока без учёта регистра), фильтр по роли. Сортировка: сначала новые. Хеш пароля никогда не возвращается. 400 с code: invalid_role — недопустимое значение фильтра role.
// @Tags admin
// @Produce json
// @Security BearerAuth
// @Param search query string false "Поиск по email или имени"
// @Param role query string false "Роль: user | contributor | moderator | admin"
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы, максимум 100"
// @Success 200 {object} listUsersResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/users [get]
func (h *Handler) ListUsers(c echo.Context) error {
	ctx := c.Request().Context()
	page, limit := parsePagination(c)

	users, total, err := h.service.ListUsers(ctx, c.QueryParam("search"), c.QueryParam("role"), page, limit)
	if err != nil {
		if errors.Is(err, adminService.ErrInvalidRole) {
			return invalidRoleResponse(c)
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list users"})
	}

	items := make([]adminUserResponse, 0, len(users))
	for _, u := range users {
		items = append(items, toAdminUserResponse(u))
	}
	return c.JSON(http.StatusOK, listUsersResponse{Items: items, Total: total, Page: page, Limit: limit})
}

// ChangeUserRole godoc
// @Summary Изменить роль пользователя (админ)
// @Description Требуется роль admin и Bearer-токен. Роль: user | contributor | moderator | admin. Изменение вступает в силу со следующего запроса пользователя. Установка текущей роли ничего не меняет (200). 400 с code: invalid_role; 404 с code: user_not_found; 409 с code: cannot_change_own_role (нельзя менять собственную роль) | last_admin (нельзя снять роль с последнего администратора). 403 — у выполняющего запрос больше нет роли admin.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID пользователя"
// @Param body body changeUserRoleRequest true "Новая роль"
// @Success 200 {object} adminUserResponse
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 409 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /admin/users/{id}/role [patch]
func (h *Handler) ChangeUserRole(c echo.Context) error {
	ctx := c.Request().Context()

	actor, ok := c.Get("user").(*usersDomain.User)
	if !ok || actor == nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	targetID, err := parseID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}

	var req changeUserRoleRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	user, err := h.service.ChangeUserRole(ctx, actor.ID, targetID, req.Role)
	if err != nil {
		switch {
		case errors.Is(err, adminService.ErrInvalidRole):
			return invalidRoleResponse(c)
		case errors.Is(err, adminService.ErrUserNotFound):
			return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found", "code": "user_not_found"})
		case errors.Is(err, adminService.ErrCannotChangeOwnRole):
			return c.JSON(http.StatusConflict, map[string]string{"error": "cannot change own role", "code": "cannot_change_own_role"})
		case errors.Is(err, adminService.ErrLastAdmin):
			return c.JSON(http.StatusConflict, map[string]string{"error": "cannot remove the last admin", "code": "last_admin"})
		case errors.Is(err, adminService.ErrActorNotAdmin):
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to change user role"})
		}
	}
	return c.JSON(http.StatusOK, toAdminUserResponse(*user))
}
