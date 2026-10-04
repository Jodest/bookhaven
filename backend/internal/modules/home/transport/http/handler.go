// Package http exposes the home page endpoints (see
// docs/features/home-page-content.md).
package http

import (
	"context"
	"net/http"
	"time"

	catalogdomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	cataloghttp "github.com/bookhaven/backend/internal/modules/catalog/transport/http"
	"github.com/bookhaven/backend/internal/modules/home/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// HomeService is the subset of *service.Service used by the handler.
type HomeService interface {
	GetHome(ctx context.Context) (*service.Home, error)
	RandomBook(ctx context.Context, genreID, excludeID *uuid.UUID) (*catalogdomain.Book, error)
}

// Handler handles home HTTP requests.
type Handler struct {
	service HomeService
}

// NewHandler creates a new home Handler.
func NewHandler(svc HomeService) *Handler {
	return &Handler{service: svc}
}

type bookOfTheDayResponse struct {
	Book               cataloghttp.BookListItemResponse `json:"book"`
	DescriptionExcerpt *string                          `json:"description_excerpt"`
}

type calendarAuthorResponse struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Slug      string  `json:"slug"`
	PhotoURL  *string `json:"photo_url"`
	BirthDate *string `json:"birth_date"`
	DeathDate *string `json:"death_date"`
}

type calendarEventResponse struct {
	Type   string                             `json:"type" enums:"birth,death"`
	Years  int                                `json:"years"`
	Author calendarAuthorResponse             `json:"author"`
	Books  []cataloghttp.BookListItemResponse `json:"books"`
}

type anniversaryResponse struct {
	Years int                              `json:"years"`
	Book  cataloghttp.BookListItemResponse `json:"book"`
}

type calendarResponse struct {
	Date          string                  `json:"date"`
	IsToday       bool                    `json:"is_today"`
	Events        []calendarEventResponse `json:"events"`
	Anniversaries []anniversaryResponse   `json:"anniversaries"`
}

type reviewUserResponse struct {
	ID        string  `json:"id"`
	Name      *string `json:"name"`
	AvatarURL *string `json:"avatar_url"`
}

type reviewBookAuthorResponse struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type reviewBookResponse struct {
	ID       string                     `json:"id"`
	Slug     string                     `json:"slug"`
	Title    string                     `json:"title"`
	CoverURL *string                    `json:"cover_url"`
	Authors  []reviewBookAuthorResponse `json:"authors"`
}

type recentReviewResponse struct {
	ID         string             `json:"id"`
	Excerpt    string             `json:"excerpt"`
	Rating     int                `json:"rating"`
	LikesCount int                `json:"likes_count"`
	CreatedAt  time.Time          `json:"created_at"`
	User       reviewUserResponse `json:"user"`
	Book       reviewBookResponse `json:"book"`
}

type homeResponse struct {
	Date          string                             `json:"date"`
	BookOfTheDay  *bookOfTheDayResponse              `json:"book_of_the_day"`
	Calendar      *calendarResponse                  `json:"calendar"`
	NewBooks      []cataloghttp.BookListItemResponse `json:"new_books"`
	TopRated      []cataloghttp.BookListItemResponse `json:"top_rated"`
	RecentReviews []recentReviewResponse             `json:"recent_reviews"`
}

type randomBookResponse struct {
	Book *cataloghttp.BookListItemResponse `json:"book"`
}

func formatDate(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format("2006-01-02")
	return &s
}

func toBookList(books []catalogdomain.Book) []cataloghttp.BookListItemResponse {
	items := make([]cataloghttp.BookListItemResponse, 0, len(books))
	for _, b := range books {
		items = append(items, cataloghttp.ToBookListItemResponse(b))
	}
	return items
}

func toHomeResponse(h *service.Home) homeResponse {
	resp := homeResponse{
		Date:          h.Date,
		NewBooks:      toBookList(h.NewBooks),
		TopRated:      toBookList(h.TopRated),
		RecentReviews: make([]recentReviewResponse, 0, len(h.RecentReviews)),
	}

	if h.BookOfTheDay != nil {
		resp.BookOfTheDay = &bookOfTheDayResponse{
			Book:               cataloghttp.ToBookListItemResponse(h.BookOfTheDay.Book),
			DescriptionExcerpt: h.BookOfTheDay.DescriptionExcerpt,
		}
	}

	if h.Calendar != nil {
		cal := &calendarResponse{
			Date:          h.Calendar.Date,
			IsToday:       h.Calendar.IsToday,
			Events:        make([]calendarEventResponse, 0, len(h.Calendar.Events)),
			Anniversaries: make([]anniversaryResponse, 0, len(h.Calendar.Anniversaries)),
		}
		for _, e := range h.Calendar.Events {
			cal.Events = append(cal.Events, calendarEventResponse{
				Type:  e.Type,
				Years: e.Years,
				Author: calendarAuthorResponse{
					ID:        e.Author.AuthorID.String(),
					Name:      e.Author.Name,
					Slug:      e.Author.Slug,
					PhotoURL:  e.Author.PhotoURL,
					BirthDate: formatDate(e.Author.BirthDate),
					DeathDate: formatDate(e.Author.DeathDate),
				},
				Books: toBookList(e.Books),
			})
		}
		for _, a := range h.Calendar.Anniversaries {
			cal.Anniversaries = append(cal.Anniversaries, anniversaryResponse{
				Years: a.Years,
				Book:  cataloghttp.ToBookListItemResponse(a.Book),
			})
		}
		resp.Calendar = cal
	}

	for _, r := range h.RecentReviews {
		authors := make([]reviewBookAuthorResponse, 0, len(r.Book.Authors))
		for _, a := range r.Book.Authors {
			authors = append(authors, reviewBookAuthorResponse{Name: a.Name, Slug: a.Slug})
		}
		resp.RecentReviews = append(resp.RecentReviews, recentReviewResponse{
			ID:         r.ID.String(),
			Excerpt:    r.Excerpt,
			Rating:     r.Rating,
			LikesCount: r.LikesCount,
			CreatedAt:  r.CreatedAt,
			User: reviewUserResponse{
				ID:        r.User.ID.String(),
				Name:      r.User.Name,
				AvatarURL: r.User.AvatarURL,
			},
			Book: reviewBookResponse{
				ID:       r.Book.ID.String(),
				Slug:     r.Book.Slug,
				Title:    r.Book.Title,
				CoverURL: r.Book.CoverURL,
				Authors:  authors,
			},
		})
	}
	return resp
}

// GetHome godoc
// @Summary Главная страница (публичные блоки)
// @Description Книга дня, литературный календарь (события авторов на сегодня или ближайший день в пределах 7 дней, юбилеи книг года), новое в каталоге, высоко оценённые книги и свежие рецензии. День — дата по Москве (UTC+3). Блок, который не удалось собрать, возвращается пустым (null или []).
// @Tags home
// @Produce json
// @Success 200 {object} homeResponse
// @Failure 500 {object} map[string]string
// @Router /home [get]
func (h *Handler) GetHome(c echo.Context) error {
	home, err := h.service.GetHome(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to get home"})
	}
	return c.JSON(http.StatusOK, toHomeResponse(home))
}

func parseOptionalUUID(raw string) (*uuid.UUID, error) {
	if raw == "" {
		return nil, nil
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return nil, err
	}
	return &id, nil
}

// RandomBook godoc
// @Summary Случайная книга («Что почитать?»)
// @Description Возвращает одну случайную видимую книгу: сначала из хорошо оценённых, затем из книг с оценками, затем любую. Не кэшируется. Если ничего не найдено, book = null.
// @Tags home
// @Produce json
// @Param genre_id query string false "ID жанра (UUID)"
// @Param exclude_id query string false "ID книги, которую исключить (UUID)"
// @Success 200 {object} randomBookResponse
// @Failure 400 {object} map[string]string
// @Failure 500 {object} map[string]string
// @Router /home/random [get]
func (h *Handler) RandomBook(c echo.Context) error {
	genreID, err := parseOptionalUUID(c.QueryParam("genre_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid genre_id"})
	}
	excludeID, err := parseOptionalUUID(c.QueryParam("exclude_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid exclude_id"})
	}

	book, err := h.service.RandomBook(c.Request().Context(), genreID, excludeID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to pick a book"})
	}
	resp := randomBookResponse{}
	if book != nil {
		item := cataloghttp.ToBookListItemResponse(*book)
		resp.Book = &item
	}
	return c.JSON(http.StatusOK, resp)
}
