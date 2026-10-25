package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
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
	now  func() time.Time
}

// NewService creates a new shelves Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo, now: func() time.Time { return time.Now().UTC() }}
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

// ReadSummary returns the caller's reading summary (finished reads count,
// last finish date, re-reading flag) of one book.
func (s *Service) ReadSummary(ctx context.Context, userID, bookID uuid.UUID) (domain.ReadSummary, error) {
	sum, err := s.repo.ReadSummary(ctx, userID, bookID)
	if err != nil {
		return domain.ReadSummary{}, fmt.Errorf("read summary: %w", err)
	}
	return sum, nil
}

// ReadCounts returns the finished-reads counts of the given books for ownerID
// (the shelf owner, not the caller), keyed by book id.
func (s *Service) ReadCounts(ctx context.Context, ownerID uuid.UUID, books []catalogDomain.Book) (map[uuid.UUID]domain.ReadCount, error) {
	ids := make([]uuid.UUID, 0, len(books))
	for _, b := range books {
		ids = append(ids, b.ID)
	}
	counts, err := s.repo.ReadCounts(ctx, ownerID, ids)
	if err != nil {
		return nil, fmt.Errorf("read counts: %w", err)
	}
	return counts, nil
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
		now := s.now()
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
			// The shelf mutation goes first; the journal step runs only when it
			// actually changed a row (a duplicate add returns an error and rolls back).
			if err := repo.AddBook(ctx, shelfID, bookID); err != nil {
				return err
			}
			return journalOnAdd(ctx, repo, shelf.Type, userID, bookID, now)
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

	remove := func(repo repository.Repository) error {
		if err := repo.RemoveBook(ctx, shelfID, bookID); err != nil {
			if errors.Is(err, repository.ErrBookNotOnShelf) {
				return ErrBookNotOnShelf
			}
			return fmt.Errorf("remove book: %w", err)
		}
		return nil
	}

	if !isDefaultShelf(shelf.Type) {
		return remove(s.repo)
	}

	return s.repo.Transaction(ctx, func(repo repository.Repository) error {
		// The «Прочитано» undo needs the moment the book was put on the shelf,
		// so it is read before the shelf item is deleted.
		var addedAt time.Time
		if shelf.Type == domain.ShelfTypeRead {
			at, ok, err := repo.ShelfItemAddedAt(ctx, shelfID, bookID)
			if err != nil {
				return fmt.Errorf("shelf item added at: %w", err)
			}
			if ok {
				addedAt = at
			}
		}
		if err := remove(repo); err != nil {
			return err
		}
		return journalOnRemove(ctx, repo, shelf.Type, userID, bookID, addedAt)
	})
}

// journalOnAdd applies the reading-journal transition for a move to the
// default shelf of the given type. The effect depends only on the target.
func journalOnAdd(ctx context.Context, repo repository.Repository, target domain.ShelfType, userID, bookID uuid.UUID, now time.Time) error {
	switch target {
	case domain.ShelfTypeReading:
		return repo.OpenRead(ctx, userID, bookID, now)
	case domain.ShelfTypeRead:
		return repo.FinishRead(ctx, userID, bookID, now)
	case domain.ShelfTypeAbandoned:
		return repo.AbandonOpenRead(ctx, userID, bookID, now)
	case domain.ShelfTypeWantToRead:
		return repo.DeleteOpenRead(ctx, userID, bookID)
	}
	return nil
}

// journalOnRemove applies the reading-journal transition for removing the book
// from the default shelf of the given type. Removing from «Прочитано» is an
// undo of the reading that the status recorded: the latest finished attempt
// that was not dated by hand and finished when the book was put on the shelf
// (addedAt, minus a slack for the Go/DB clock difference) or later.
func journalOnRemove(ctx context.Context, repo repository.Repository, from domain.ShelfType, userID, bookID uuid.UUID, addedAt time.Time) error {
	switch from {
	case domain.ShelfTypeReading:
		return repo.DeleteOpenRead(ctx, userID, bookID)
	case domain.ShelfTypeRead:
		return repo.DeleteLatestFinishedRead(ctx, userID, bookID, addedAt.Add(-time.Minute))
	}
	return nil
}

func isDefaultShelf(t domain.ShelfType) bool {
	return t == domain.ShelfTypeWantToRead || t == domain.ShelfTypeReading ||
		t == domain.ShelfTypeRead || t == domain.ShelfTypeAbandoned
}

func isCustomShelf(t domain.ShelfType) bool {
	return t == domain.ShelfTypeCustom
}

var defaultShelfTypes = []domain.ShelfType{
	domain.ShelfTypeWantToRead, domain.ShelfTypeReading, domain.ShelfTypeRead, domain.ShelfTypeAbandoned,
}

// otherDefaultTypes returns the other default shelf types of the exclusive group.
func otherDefaultTypes(t domain.ShelfType) []domain.ShelfType {
	others := make([]domain.ShelfType, 0, len(defaultShelfTypes)-1)
	for _, d := range defaultShelfTypes {
		if d != t {
			others = append(others, d)
		}
	}
	return others
}
