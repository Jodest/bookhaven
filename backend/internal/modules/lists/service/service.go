package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/lists/domain"
	"github.com/bookhaven/backend/internal/modules/lists/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrListNotFound      = errors.New("list not found")
	ErrForbidden         = errors.New("forbidden")
	ErrBookNotFound      = errors.New("book not found")
	ErrBookAlreadyOnList = repository.ErrBookAlreadyOnList
	ErrBookNotOnList     = repository.ErrBookNotOnList
	ErrInvalidTitle      = errors.New("invalid list title")
	ErrInvalidDescription = errors.New("invalid list description")
)

// Service provides lists logic.
type Service struct {
	repo repository.Repository
}

// NewService creates a new lists Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// ListLists returns all lists for a user. When bookID is provided, it also
// returns the set of list IDs that already contain the book.
func (s *Service) ListLists(ctx context.Context, userID uuid.UUID, bookID *uuid.UUID) ([]domain.List, map[uuid.UUID]struct{}, error) {
	lists, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("list lists: %w", err)
	}

	var inLists map[uuid.UUID]struct{}
	if bookID != nil {
		inLists, err = s.repo.UserListsWithBook(ctx, userID, *bookID)
		if err != nil {
			return nil, nil, fmt.Errorf("get lists with book: %w", err)
		}
	}

	return lists, inLists, nil
}

// CreateList creates a new list for a user.
func (s *Service) CreateList(ctx context.Context, userID uuid.UUID, title string, description *string, isPublic bool) (*domain.List, error) {
	title = strings.TrimSpace(title)
	if len(title) < 2 || len(title) > 255 {
		return nil, ErrInvalidTitle
	}

	var desc string
	if description != nil {
		desc = strings.TrimSpace(*description)
		if len(desc) > 2000 {
			return nil, ErrInvalidDescription
		}
	}

	list := &domain.List{
		ID:          uuid.New(),
		UserID:      userID,
		Title:       title,
		Description: desc,
		IsPublic:    isPublic,
	}

	if err := s.repo.Create(ctx, list); err != nil {
		return nil, fmt.Errorf("create list: %w", err)
	}

	return list, nil
}

// GetList returns a list with paginated books (owner only).
func (s *Service) GetList(ctx context.Context, userID uuid.UUID, listID uuid.UUID, page, limit int) (*domain.List, []catalogDomain.Book, int64, error) {
	list, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, 0, ErrListNotFound
		}
		return nil, nil, 0, fmt.Errorf("get list: %w", err)
	}

	if list.UserID != userID {
		return nil, nil, 0, ErrForbidden
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	books, total, err := s.repo.ListBooks(ctx, listID, limit, offset)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("list books: %w", err)
	}

	list.BooksCount = total
	return list, books, total, nil
}

// ListPublicByUser returns all public lists owned by the user.
func (s *Service) ListPublicByUser(ctx context.Context, userID uuid.UUID) ([]domain.List, error) {
	lists, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list public lists: %w", err)
	}

	public := make([]domain.List, 0, len(lists))
	for _, l := range lists {
		if l.IsPublic {
			public = append(public, l)
		}
	}

	return public, nil
}

// GetPublicList returns a public list with paginated books.
func (s *Service) GetPublicList(ctx context.Context, listID uuid.UUID, page, limit int) (*domain.List, []catalogDomain.Book, int64, error) {
	list, err := s.repo.GetPublicByID(ctx, listID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, 0, ErrListNotFound
		}
		return nil, nil, 0, fmt.Errorf("get public list: %w", err)
	}

	if page <= 0 {
		page = 1
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := (page - 1) * limit

	books, total, err := s.repo.ListBooks(ctx, listID, limit, offset)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("list books: %w", err)
	}

	list.BooksCount = total
	return list, books, total, nil
}

// UpdateList updates a list's title, description, and visibility.
func (s *Service) UpdateList(ctx context.Context, userID uuid.UUID, listID uuid.UUID, title *string, description *string, isPublic *bool) (*domain.List, error) {
	list, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrListNotFound
		}
		return nil, fmt.Errorf("get list: %w", err)
	}

	if list.UserID != userID {
		return nil, ErrForbidden
	}

	if title != nil {
		t := strings.TrimSpace(*title)
		if len(t) < 2 || len(t) > 255 {
			return nil, ErrInvalidTitle
		}
		list.Title = t
	}

	if description != nil {
		d := strings.TrimSpace(*description)
		if len(d) > 2000 {
			return nil, ErrInvalidDescription
		}
		list.Description = d
	}

	if isPublic != nil {
		list.IsPublic = *isPublic
	}

	if err := s.repo.Update(ctx, list); err != nil {
		return nil, fmt.Errorf("update list: %w", err)
	}

	return list, nil
}

// DeleteList deletes a list.
func (s *Service) DeleteList(ctx context.Context, userID uuid.UUID, listID uuid.UUID) error {
	list, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrListNotFound
		}
		return fmt.Errorf("get list: %w", err)
	}

	if list.UserID != userID {
		return ErrForbidden
	}

	if err := s.repo.Delete(ctx, listID); err != nil {
		return fmt.Errorf("delete list: %w", err)
	}

	return nil
}

// AddBook adds a book to a list.
func (s *Service) AddBook(ctx context.Context, userID uuid.UUID, listID, bookID uuid.UUID) error {
	list, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrListNotFound
		}
		return fmt.Errorf("get list: %w", err)
	}

	if list.UserID != userID {
		return ErrForbidden
	}

	exists, err := s.repo.BookExists(ctx, bookID)
	if err != nil {
		return fmt.Errorf("check book exists: %w", err)
	}
	if !exists {
		return ErrBookNotFound
	}

	maxPos, err := s.repo.MaxPosition(ctx, listID)
	if err != nil {
		return fmt.Errorf("get max position: %w", err)
	}

	if err := s.repo.AddBook(ctx, listID, bookID, maxPos+1); err != nil {
		if errors.Is(err, repository.ErrBookAlreadyOnList) {
			return ErrBookAlreadyOnList
		}
		return fmt.Errorf("add book: %w", err)
	}
	return nil
}

// RemoveBook removes a book from a list.
func (s *Service) RemoveBook(ctx context.Context, userID uuid.UUID, listID, bookID uuid.UUID) error {
	list, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrListNotFound
		}
		return fmt.Errorf("get list: %w", err)
	}

	if list.UserID != userID {
		return ErrForbidden
	}

	if err := s.repo.RemoveBook(ctx, listID, bookID); err != nil {
		if errors.Is(err, repository.ErrBookNotOnList) {
			return ErrBookNotOnList
		}
		return fmt.Errorf("remove book: %w", err)
	}

	return nil
}

// ReorderBooks updates the order of books in a list.
func (s *Service) ReorderBooks(ctx context.Context, userID uuid.UUID, listID uuid.UUID, bookIDs []uuid.UUID) error {
	list, err := s.repo.GetByID(ctx, listID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrListNotFound
		}
		return fmt.Errorf("get list: %w", err)
	}

	if list.UserID != userID {
		return ErrForbidden
	}

	if err := s.repo.ReorderBooks(ctx, listID, bookIDs); err != nil {
		return fmt.Errorf("reorder books: %w", err)
	}

	return nil
}
