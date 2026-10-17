package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

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
	ErrInvalidDescription    = errors.New("invalid shelf description")
	ErrNothingToUpdate       = errors.New("nothing to update")
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
// Description is optional; isPublic defaults to true when nil.
func (s *Service) CreateShelf(ctx context.Context, userID uuid.UUID, name string, description *string, isPublic *bool) (*domain.Shelf, error) {
	name = strings.TrimSpace(name)
	if !validName(name) {
		return nil, ErrInvalidName
	}

	desc := ""
	if description != nil {
		desc = strings.TrimSpace(*description)
		if !validDescription(desc) {
			return nil, ErrInvalidDescription
		}
	}

	public := true
	if isPublic != nil {
		public = *isPublic
	}

	shelf := &domain.Shelf{
		ID:          uuid.New(),
		UserID:      userID,
		Name:        name,
		Type:        domain.ShelfTypeCustom,
		Description: desc,
		IsPublic:    public,
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

// ListPublicByUser returns the user's public shelves that have at least
// one visible book, with BooksCount computed over visible books only.
// Private shelves are skipped.
func (s *Service) ListPublicByUser(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error) {
	shelves, err := s.repo.ListByUserVisible(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list public shelves: %w", err)
	}

	public := make([]domain.Shelf, 0, len(shelves))
	for _, sh := range shelves {
		if sh.IsPublic && sh.BooksCount > 0 {
			public = append(public, sh)
		}
	}

	return public, nil
}

// GetPublicShelf returns a shelf with paginated, visible-only books. Unlike
// GetShelf, it performs no ownership check. A private shelf is reported as not
// found for everyone, including its owner (who uses GetShelf instead).
func (s *Service) GetPublicShelf(ctx context.Context, shelfID uuid.UUID, page, limit int) (*domain.Shelf, []catalogDomain.Book, int64, error) {
	shelf, err := s.repo.GetByID(ctx, shelfID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, 0, ErrShelfNotFound
		}
		return nil, nil, 0, fmt.Errorf("get shelf: %w", err)
	}

	if !shelf.IsPublic {
		return nil, nil, 0, ErrShelfNotFound
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

// UpdateShelfInput holds the optional fields of a partial shelf update.
type UpdateShelfInput struct {
	Name        *string
	Description *string
	IsPublic    *bool
}

func validName(name string) bool {
	n := utf8.RuneCountInString(name)
	return n >= 2 && n <= 100
}

func validDescription(desc string) bool {
	return utf8.RuneCountInString(desc) <= 2000
}

// UpdateShelf partially updates a custom shelf (name, description, privacy).
func (s *Service) UpdateShelf(ctx context.Context, userID uuid.UUID, shelfID uuid.UUID, in UpdateShelfInput) (*domain.Shelf, error) {
	if in.Name == nil && in.Description == nil && in.IsPublic == nil {
		return nil, ErrNothingToUpdate
	}

	var name, desc string
	if in.Name != nil {
		name = strings.TrimSpace(*in.Name)
		if !validName(name) {
			return nil, ErrInvalidName
		}
	}
	if in.Description != nil {
		desc = strings.TrimSpace(*in.Description)
		if !validDescription(desc) {
			return nil, ErrInvalidDescription
		}
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

	if in.Name != nil {
		shelf.Name = name
	}
	if in.Description != nil {
		shelf.Description = desc
	}
	if in.IsPublic != nil {
		shelf.IsPublic = *in.IsPublic
	}
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
