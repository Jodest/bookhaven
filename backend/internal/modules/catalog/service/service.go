package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/catalog/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var ErrBookNotFound = errors.New("book not found")
var ErrAuthorNotFound = errors.New("author not found")

// Service provides catalog logic.
type Service struct {
	repo repository.Repository
}

// NewService creates a new catalog Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// ListBooks returns a paginated list of books.
func (s *Service) ListBooks(ctx context.Context, filter repository.BookFilter) ([]domain.Book, int64, error) {
	books, total, err := s.repo.ListBooks(ctx, filter)
	if err != nil {
		return nil, 0, fmt.Errorf("list books: %w", err)
	}
	return books, total, nil
}

// GetBookBySlug returns a book by slug.
func (s *Service) GetBookBySlug(ctx context.Context, slug string) (*domain.Book, error) {
	book, err := s.repo.GetBookBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBookNotFound
		}
		return nil, fmt.Errorf("get book: %w", err)
	}
	return book, nil
}

// GetAuthorBySlug returns an author by slug.
func (s *Service) GetAuthorBySlug(ctx context.Context, slug string) (*domain.Author, error) {
	author, err := s.repo.GetAuthorBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAuthorNotFound
		}
		return nil, fmt.Errorf("get author: %w", err)
	}
	return author, nil
}

// ListGenres returns all genres.
func (s *Service) ListGenres(ctx context.Context) ([]domain.Genre, error) {
	genres, err := s.repo.ListGenres(ctx)
	if err != nil {
		return nil, fmt.Errorf("list genres: %w", err)
	}
	return genres, nil
}

// ListAuthors returns authors matching a search query.
func (s *Service) ListAuthors(ctx context.Context, search string, limit int) ([]domain.Author, int64, error) {
	authors, total, err := s.repo.ListAuthors(ctx, search, limit)
	if err != nil {
		return nil, 0, fmt.Errorf("list authors: %w", err)
	}
	return authors, total, nil
}

// ListLanguages returns distinct language values from books.
func (s *Service) ListLanguages(ctx context.Context) ([]string, error) {
	languages, err := s.repo.ListLanguages(ctx)
	if err != nil {
		return nil, fmt.Errorf("list languages: %w", err)
	}
	return languages, nil
}

// ParseUUID parses a UUID string.
func ParseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}
