package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrShelfNotFound         = errors.New("shelf not found")
	ErrForbidden             = errors.New("forbidden")
	ErrBookNotFound          = errors.New("book not found")
	ErrBookAlreadyOnShelf    = repository.ErrBookAlreadyOnShelf
	ErrBookNotOnShelf        = repository.ErrBookNotOnShelf
	ErrDefaultShelfImmutable = errors.New("default shelf cannot be modified")
	ErrInvalidName           = errors.New("invalid shelf name")
)

// Service provides shelves logic.
type Service struct {
	repo repository.Repository
}

// NewService creates a new shelves Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// ListShelves returns all shelves for a user, optionally marking whether they contain a book.
func (s *Service) ListShelves(ctx context.Context, userID uuid.UUID, bookID *uuid.UUID) ([]domain.Shelf, error) {
	if err := s.repo.EnsureDefaultShelves(ctx, userID); err != nil {
		return nil, fmt.Errorf("ensure default shelves: %w", err)
	}

	shelves, err := s.repo.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list shelves: %w", err)
	}

	if bookID != nil {
		shelfIDs, err := s.repo.UserShelvesWithBook(ctx, userID, *bookID)
		if err != nil {
			return nil, fmt.Errorf("get shelves with book: %w", err)
		}
		for i := range shelves {
			if _, ok := shelfIDs[shelves[i].ID]; ok {
				shelves[i].HasBook = true
			}
		}
	}

	return shelves, nil
}

// CreateShelf creates a custom shelf for a user.
func (s *Service) CreateShelf(ctx context.Context, userID uuid.UUID, name string) (*domain.Shelf, error) {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 100 {
		return nil, ErrInvalidName
	}

	shelf := &domain.Shelf{
		ID:     uuid.New(),
		UserID: userID,
		Name:   name,
		Type:   domain.ShelfTypeCustom,
	}

	if err := s.repo.Create(ctx, shelf); err != nil {
		return nil, fmt.Errorf("create shelf: %w", err)
	}

	return shelf, nil
}

// GetShelf returns a shelf with paginated books.
func (s *Service) GetShelf(ctx context.Context, userID uuid.UUID, shelfID uuid.UUID, page, limit int) (*domain.Shelf, []catalogDomain.Book, int64, error) {
	shelf, err := s.repo.GetByID(ctx, shelfID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, 0, ErrShelfNotFound
		}
		return nil, nil, 0, fmt.Errorf("get shelf: %w", err)
	}

	if shelf.UserID != userID {
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

	books, total, err := s.repo.ListBooks(ctx, shelfID, limit, offset)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("list books: %w", err)
	}

	shelf.BooksCount = total
	return shelf, books, total, nil
}

// RenameShelf renames a custom shelf.
func (s *Service) RenameShelf(ctx context.Context, userID uuid.UUID, shelfID uuid.UUID, name string) (*domain.Shelf, error) {
	name = strings.TrimSpace(name)
	if len(name) < 2 || len(name) > 100 {
		return nil, ErrInvalidName
	}

	shelf, err := s.repo.GetByID(ctx, shelfID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrShelfNotFound
		}
		return nil, fmt.Errorf("get shelf: %w", err)
	}

	if shelf.UserID != userID {
		return nil, ErrForbidden
	}

	if !isCustomShelf(shelf.Type) {
		return nil, ErrDefaultShelfImmutable
	}

	shelf.Name = name
	if err := s.repo.Update(ctx, shelf); err != nil {
		return nil, fmt.Errorf("update shelf: %w", err)
	}

	return shelf, nil
}

// DeleteShelf deletes a custom shelf.
func (s *Service) DeleteShelf(ctx context.Context, userID uuid.UUID, shelfID uuid.UUID) error {
	shelf, err := s.repo.GetByID(ctx, shelfID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrShelfNotFound
		}
		return fmt.Errorf("get shelf: %w", err)
	}

	if shelf.UserID != userID {
		return ErrForbidden
	}

	if !isCustomShelf(shelf.Type) {
		return ErrDefaultShelfImmutable
	}

	if err := s.repo.Delete(ctx, shelfID); err != nil {
		return fmt.Errorf("delete shelf: %w", err)
	}

	return nil
}

// AddBook adds a book to a shelf, maintaining exclusivity of the default trio.
func (s *Service) AddBook(ctx context.Context, userID uuid.UUID, shelfID, bookID uuid.UUID) error {
	shelf, err := s.repo.GetByID(ctx, shelfID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrShelfNotFound
		}
		return fmt.Errorf("get shelf: %w", err)
	}

	if shelf.UserID != userID {
		return ErrForbidden
	}

	exists, err := s.repo.BookExists(ctx, bookID)
	if err != nil {
		return fmt.Errorf("check book exists: %w", err)
	}
	if !exists {
		return ErrBookNotFound
	}

	if isDefaultShelf(shelf.Type) {
		otherTypes := otherDefaultTypes(shelf.Type)
		return s.repo.Transaction(ctx, func(repo repository.Repository) error {
			for _, t := range otherTypes {
				otherShelf, err := repo.GetByUserAndType(ctx, userID, t)
				if err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					return err
				}
				if err := repo.RemoveBook(ctx, otherShelf.ID, bookID); err != nil && !errors.Is(err, ErrBookNotOnShelf) {
					return err
				}
			}
			return repo.AddBook(ctx, shelfID, bookID)
		})
	}

	if err := s.repo.AddBook(ctx, shelfID, bookID); err != nil {
		return fmt.Errorf("add book: %w", err)
	}
	return nil
}

// RemoveBook removes a book from a shelf.
func (s *Service) RemoveBook(ctx context.Context, userID uuid.UUID, shelfID, bookID uuid.UUID) error {
	shelf, err := s.repo.GetByID(ctx, shelfID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrShelfNotFound
		}
		return fmt.Errorf("get shelf: %w", err)
	}

	if shelf.UserID != userID {
		return ErrForbidden
	}

	if err := s.repo.RemoveBook(ctx, shelfID, bookID); err != nil {
		if errors.Is(err, repository.ErrBookNotOnShelf) {
			return ErrBookNotOnShelf
		}
		return fmt.Errorf("remove book: %w", err)
	}

	return nil
}

func isDefaultShelf(t domain.ShelfType) bool {
	return t == domain.ShelfTypeWantToRead || t == domain.ShelfTypeReading || t == domain.ShelfTypeRead
}

func isCustomShelf(t domain.ShelfType) bool {
	return t == domain.ShelfTypeCustom
}

func otherDefaultTypes(t domain.ShelfType) [2]domain.ShelfType {
	switch t {
	case domain.ShelfTypeWantToRead:
		return [2]domain.ShelfType{domain.ShelfTypeReading, domain.ShelfTypeRead}
	case domain.ShelfTypeReading:
		return [2]domain.ShelfType{domain.ShelfTypeWantToRead, domain.ShelfTypeRead}
	case domain.ShelfTypeRead:
		return [2]domain.ShelfType{domain.ShelfTypeWantToRead, domain.ShelfTypeReading}
	}
	return [2]domain.ShelfType{}
}
