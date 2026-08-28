package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

// BookHandler handles book-related HTTP requests.
type BookHandler struct {
	db interface{}
}

// NewBookHandler creates a new BookHandler.
func NewBookHandler() *BookHandler {
	return &BookHandler{}
}

// List returns a list of books.
func (h *BookHandler) List(c echo.Context) error {
	books := []map[string]interface{}{
		{
			"slug":  "war-and-peace",
			"title": "Война и мир",
		},
		{
			"slug":  "crime-and-punishment",
			"title": "Преступление и наказание",
		},
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"items": books,
		"total": len(books),
	})
}

// Get returns a single book by slug.
func (h *BookHandler) Get(c echo.Context) error {
	slug := c.Param("slug")
	return c.JSON(http.StatusOK, map[string]interface{}{
		"slug":  slug,
		"title": "Заглушка для книги: " + slug,
	})
}
