package http

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	notificationsDomain "github.com/bookhaven/backend/internal/modules/notifications/domain"
	"github.com/bookhaven/backend/internal/modules/notifications/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// Handler handles notifications HTTP requests.
type Handler struct {
	service *service.Service
}

// NewHandler creates a new notifications Handler.
func NewHandler(service *service.Service) *Handler {
	return &Handler{service: service}
}

type notificationActorResponse struct {
	ID        string  `json:"id"`
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

type notificationReviewResponse struct {
	ID        string `json:"id"`
	BookID    string `json:"book_id"`
	BookSlug  string `json:"book_slug"`
	BookTitle string `json:"book_title"`
}

type notificationResponse struct {
	ID        string                      `json:"id"`
	Type      string                      `json:"type"`
	Actor     notificationActorResponse   `json:"actor"`
	Review    *notificationReviewResponse `json:"review,omitempty"`
	IsRead    bool                        `json:"is_read"`
	CreatedAt time.Time                   `json:"created_at"`
}

type listNotificationsResponse struct {
	Items []notificationResponse `json:"items"`
	Total int64                  `json:"total"`
	Page  int                    `json:"page"`
	Limit int                    `json:"limit"`
}

type unreadCountResponse struct {
	Count int64 `json:"count"`
}

func toNotificationResponse(n notificationsDomain.Notification) notificationResponse {
	resp := notificationResponse{
		ID:   n.ID.String(),
		Type: n.Type,
		Actor: notificationActorResponse{
			ID:        n.ActorID.String(),
			Name:      n.ActorName,
			AvatarURL: n.ActorAvatarURL,
		},
		IsRead:    n.IsRead,
		CreatedAt: n.CreatedAt,
	}
	if n.ReviewID != nil && n.BookID != nil {
		var bookSlug, bookTitle string
		if n.BookSlug != nil {
			bookSlug = *n.BookSlug
		}
		if n.BookTitle != nil {
			bookTitle = *n.BookTitle
		}
		resp.Review = &notificationReviewResponse{
			ID:        n.ReviewID.String(),
			BookID:    n.BookID.String(),
			BookSlug:  bookSlug,
			BookTitle: bookTitle,
		}
	}
	return resp
}

func getUserID(c echo.Context) (uuid.UUID, error) {
	userIDStr, ok := c.Get("user_id").(string)
	if !ok || userIDStr == "" {
		return uuid.Nil, errors.New("unauthorized")
	}
	return uuid.Parse(userIDStr)
}

func parseNotificationID(c echo.Context) (uuid.UUID, error) {
	return uuid.Parse(c.Param("id"))
}

// ListNotifications godoc
// @Summary Список уведомлений текущего пользователя
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param page query int false "Номер страницы"
// @Param limit query int false "Размер страницы"
// @Success 200 {object} listNotificationsResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /notifications [get]
func (h *Handler) ListNotifications(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	result, err := h.service.List(ctx, userID, page, limit)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to list notifications"})
	}

	items := make([]notificationResponse, 0, len(result.Items))
	for _, n := range result.Items {
		items = append(items, toNotificationResponse(n))
	}

	return c.JSON(http.StatusOK, listNotificationsResponse{
		Items: items,
		Total: result.Total,
		Page:  result.Page,
		Limit: result.Limit,
	})
}

// UnreadCount godoc
// @Summary Количество непрочитанных уведомлений
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} unreadCountResponse
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /notifications/unread-count [get]
func (h *Handler) UnreadCount(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	count, err := h.service.UnreadCount(ctx, userID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to count notifications"})
	}

	return c.JSON(http.StatusOK, unreadCountResponse{Count: count})
}

// MarkRead godoc
// @Summary Отметить уведомление прочитанным
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Param id path string true "ID уведомления"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /notifications/{id}/read [patch]
func (h *Handler) MarkRead(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	notificationID, err := parseNotificationID(c)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid notification id"})
	}

	if err := h.service.MarkRead(ctx, userID, notificationID); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "notification not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to mark notification read"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

// MarkAllRead godoc
// @Summary Отметить все уведомления прочитанными
// @Tags notifications
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string
// @Failure 401 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /notifications/read-all [post]
func (h *Handler) MarkAllRead(c echo.Context) error {
	ctx := c.Request().Context()

	userID, err := getUserID(c)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	if err := h.service.MarkAllRead(ctx, userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to mark notifications read"})
	}

	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
