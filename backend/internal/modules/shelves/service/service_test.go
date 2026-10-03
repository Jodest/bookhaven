package service

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type shelfBookKey struct {
	shelfID uuid.UUID
	bookID  uuid.UUID
}

type fakeRepository struct {
	shelves        map[uuid.UUID]*domain.Shelf
	items          map[shelfBookKey]time.Time
	books          map[uuid.UUID]*catalogDomain.Book
	defaultEnsured map[uuid.UUID]bool
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		shelves:        make(map[uuid.UUID]*domain.Shelf),
		items:          make(map[shelfBookKey]time.Time),
		books:          make(map[uuid.UUID]*catalogDomain.Book),
		defaultEnsured: make(map[uuid.UUID]bool),
	}
}

func (f *fakeRepository) EnsureDefaultShelves(ctx context.Context, userID uuid.UUID) error {
	if f.defaultEnsured[userID] {
		return nil
	}
	defaults := []struct {
		name string
		typ  domain.ShelfType
	}{
		{"Хочу прочитать", domain.ShelfTypeWantToRead},
		{"Читаю", domain.ShelfTypeReading},
		{"Прочитано", domain.ShelfTypeRead},
	}
	for _, d := range defaults {
		exists := false
		for _, s := range f.shelves {
			if s.UserID == userID && s.Type == d.typ {
				exists = true
				break
			}
		}
		if !exists {
			s := &domain.Shelf{
				ID:     uuid.New(),
				UserID: userID,
				Name:   d.name,
				Type:   d.typ,
			}
			f.shelves[s.ID] = s
		}
	}
	f.defaultEnsured[userID] = true
	return nil
}

func (f *fakeRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error) {
	var result []domain.Shelf
	for _, s := range f.shelves {
		if s.UserID == userID {
			copy := *s
			for key := range f.items {
				if key.shelfID == copy.ID {
					copy.BooksCount++
				}
			}
			result = append(result, copy)
		}
	}
	return result, nil
}

// ListByUserVisible mirrors ListByUser but only counts books that are not
// hidden, matching the GormRepository implementation used in production.
func (f *fakeRepository) ListByUserVisible(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error) {
	var result []domain.Shelf
	for _, s := range f.shelves {
		if s.UserID == userID {
			copy := *s
			for key := range f.items {
				if key.shelfID != copy.ID {
					continue
				}
				if b, ok := f.books[key.bookID]; ok && !b.IsHidden {
					copy.BooksCount++
				}
			}
			result = append(result, copy)
		}
	}
	return result, nil
}

func (f *fakeRepository) GetByID(ctx context.Context, shelfID uuid.UUID) (*domain.Shelf, error) {
	s, ok := f.shelves[shelfID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *s
	return &copy, nil
}

func (f *fakeRepository) GetByUserAndType(ctx context.Context, userID uuid.UUID, shelfType domain.ShelfType) (*domain.Shelf, error) {
	for _, s := range f.shelves {
		if s.UserID == userID && s.Type == shelfType {
			copy := *s
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) Create(ctx context.Context, shelf *domain.Shelf) error {
	copy := *shelf
	f.shelves[shelf.ID] = &copy
	return nil
}

func (f *fakeRepository) Update(ctx context.Context, shelf *domain.Shelf) error {
	if _, ok := f.shelves[shelf.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	copy := *shelf
	f.shelves[shelf.ID] = &copy
	return nil
}

func (f *fakeRepository) Delete(ctx context.Context, shelfID uuid.UUID) error {
	if _, ok := f.shelves[shelfID]; !ok {
		return gorm.ErrRecordNotFound
	}
	delete(f.shelves, shelfID)
	for key := range f.items {
		if key.shelfID == shelfID {
			delete(f.items, key)
		}
	}
	return nil
}

func (f *fakeRepository) AddBook(ctx context.Context, shelfID, bookID uuid.UUID) error {
	key := shelfBookKey{shelfID: shelfID, bookID: bookID}
	if _, ok := f.items[key]; ok {
		return repository.ErrBookAlreadyOnShelf
	}
	f.items[key] = time.Now()
	return nil
}

func (f *fakeRepository) RemoveBook(ctx context.Context, shelfID, bookID uuid.UUID) error {
	key := shelfBookKey{shelfID: shelfID, bookID: bookID}
	if _, ok := f.items[key]; !ok {
		return repository.ErrBookNotOnShelf
	}
	delete(f.items, key)
	return nil
}

func (f *fakeRepository) BookOnShelf(ctx context.Context, shelfID, bookID uuid.UUID) (bool, error) {
	_, ok := f.items[shelfBookKey{shelfID: shelfID, bookID: bookID}]
	return ok, nil
}

func (f *fakeRepository) ListBooks(ctx context.Context, shelfID uuid.UUID, limit, offset int) ([]catalogDomain.Book, int64, error) {
	var books []catalogDomain.Book
	for key := range f.items {
		if key.shelfID == shelfID {
			if b, ok := f.books[key.bookID]; ok && !b.IsHidden {
				books = append(books, *b)
			}
		}
	}
	total := int64(len(books))
	if offset >= len(books) {
		return []catalogDomain.Book{}, total, nil
	}
	end := offset + limit
	if end > len(books) {
		end = len(books)
	}
	return books[offset:end], total, nil
}

func (f *fakeRepository) BookExists(ctx context.Context, bookID uuid.UUID) (bool, error) {
	b, ok := f.books[bookID]
	return ok && !b.IsHidden, nil
}

func (f *fakeRepository) UserShelvesWithBook(ctx context.Context, userID, bookID uuid.UUID) (map[uuid.UUID]struct{}, error) {
	result := make(map[uuid.UUID]struct{})
	for key := range f.items {
		if key.bookID == bookID {
			if s, ok := f.shelves[key.shelfID]; ok && s.UserID == userID {
				result[key.shelfID] = struct{}{}
			}
		}
	}
	return result, nil
}

func (f *fakeRepository) Transaction(ctx context.Context, fn func(repo repository.Repository) error) error {
	return fn(f)
}

func setupTestService() (*Service, *fakeRepository) {
	repo := newFakeRepository()
	return NewService(repo), repo
}

func TestListShelves_CreatesDefaultsOnce(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelves, err := svc.ListShelves(ctx, userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shelves) != 3 {
		t.Fatalf("expected 3 default shelves, got %d", len(shelves))
	}

	shelves2, err := svc.ListShelves(ctx, userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shelves2) != 3 {
		t.Fatalf("expected 3 default shelves on second call, got %d", len(shelves2))
	}

	count := 0
	for _, s := range repo.shelves {
		if s.UserID == userID {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("expected 3 total shelves for user, got %d", count)
	}
}

func TestCreateShelf(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	shelf, err := svc.CreateShelf(ctx, userID, "Классика")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shelf.Type != domain.ShelfTypeCustom {
		t.Fatalf("expected custom type, got %s", shelf.Type)
	}
	if shelf.Name != "Классика" {
		t.Fatalf("expected name Классика, got %s", shelf.Name)
	}
}

func TestCreateShelf_InvalidName(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	cases := []string{"", "a", "   a   "}
	for _, name := range cases {
		_, err := svc.CreateShelf(ctx, userID, name)
		if !errors.Is(err, ErrInvalidName) {
			t.Fatalf("expected ErrInvalidName for %q, got %v", name, err)
		}
	}
}

func TestGetShelf_Forbidden(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: ownerID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf

	_, _, _, err := svc.GetShelf(ctx, otherID, shelf.ID, 1, 20)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestGetShelf_NotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	_, _, _, err := svc.GetShelf(ctx, userID, uuid.New(), 1, 20)
	if !errors.Is(err, ErrShelfNotFound) {
		t.Fatalf("expected ErrShelfNotFound, got %v", err)
	}
}

func TestRenameShelf(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf

	renamed, err := svc.RenameShelf(ctx, userID, shelf.ID, "Новое имя")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if renamed.Name != "Новое имя" {
		t.Fatalf("expected name Новое имя, got %s", renamed.Name)
	}
}

func TestRenameShelf_DefaultImmutable(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Хочу прочитать", Type: domain.ShelfTypeWantToRead}
	repo.shelves[shelf.ID] = shelf

	_, err := svc.RenameShelf(ctx, userID, shelf.ID, "Новое имя")
	if !errors.Is(err, ErrDefaultShelfImmutable) {
		t.Fatalf("expected ErrDefaultShelfImmutable, got %v", err)
	}
}

func TestDeleteShelf(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf

	if err := svc.DeleteShelf(ctx, userID, shelf.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.shelves[shelf.ID]; ok {
		t.Fatal("expected shelf to be deleted")
	}
}

func TestDeleteShelf_DefaultImmutable(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Читаю", Type: domain.ShelfTypeReading}
	repo.shelves[shelf.ID] = shelf

	err := svc.DeleteShelf(ctx, userID, shelf.ID)
	if !errors.Is(err, ErrDefaultShelfImmutable) {
		t.Fatalf("expected ErrDefaultShelfImmutable, got %v", err)
	}
}

func TestAddBook(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf
	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	if err := svc.AddBook(ctx, userID, shelf.ID, bookID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := repo.items[shelfBookKey{shelfID: shelf.ID, bookID: bookID}]; !ok {
		t.Fatal("expected book to be added")
	}
}

func TestAddBook_BookNotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf

	err := svc.AddBook(ctx, userID, shelf.ID, uuid.New())
	if !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected ErrBookNotFound, got %v", err)
	}
}

func TestAddBook_Duplicate(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf
	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	if err := svc.AddBook(ctx, userID, shelf.ID, bookID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := svc.AddBook(ctx, userID, shelf.ID, bookID)
	if !errors.Is(err, ErrBookAlreadyOnShelf) {
		t.Fatalf("expected ErrBookAlreadyOnShelf, got %v", err)
	}
}

func TestAddBook_DefaultExclusivity(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	wantToRead := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Хочу прочитать", Type: domain.ShelfTypeWantToRead}
	reading := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Читаю", Type: domain.ShelfTypeReading}
	read := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Прочитано", Type: domain.ShelfTypeRead}
	repo.shelves[wantToRead.ID] = wantToRead
	repo.shelves[reading.ID] = reading
	repo.shelves[read.ID] = read

	custom := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Custom", Type: domain.ShelfTypeCustom}
	repo.shelves[custom.ID] = custom

	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	if err := svc.AddBook(ctx, userID, wantToRead.ID, bookID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.AddBook(ctx, userID, custom.ID, bookID); err != nil {
		t.Fatalf("unexpected error adding to custom: %v", err)
	}
	if err := svc.AddBook(ctx, userID, reading.ID, bookID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := repo.items[shelfBookKey{shelfID: wantToRead.ID, bookID: bookID}]; ok {
		t.Fatal("expected book removed from want_to_read")
	}
	if _, ok := repo.items[shelfBookKey{shelfID: reading.ID, bookID: bookID}]; !ok {
		t.Fatal("expected book on reading")
	}
	if _, ok := repo.items[shelfBookKey{shelfID: custom.ID, bookID: bookID}]; !ok {
		t.Fatal("expected book still on custom")
	}
}

func TestRemoveBook(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf
	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
	repo.items[shelfBookKey{shelfID: shelf.ID, bookID: bookID}] = time.Now()

	if err := svc.RemoveBook(ctx, userID, shelf.ID, bookID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.items[shelfBookKey{shelfID: shelf.ID, bookID: bookID}]; ok {
		t.Fatal("expected book to be removed")
	}
}

func TestRemoveBook_NotOnShelf(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf

	err := svc.RemoveBook(ctx, userID, shelf.ID, uuid.New())
	if !errors.Is(err, ErrBookNotOnShelf) {
		t.Fatalf("expected ErrBookNotOnShelf, got %v", err)
	}
}

func TestAddBook_Forbidden(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: ownerID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf
	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	err := svc.AddBook(ctx, otherID, shelf.ID, bookID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestListShelves_HasBook(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	shelves, err := svc.ListShelves(ctx, userID, &bookID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantToRead := findShelf(shelves, domain.ShelfTypeWantToRead)
	if wantToRead == nil {
		t.Fatal("want_to_read shelf not found")
	}

	repo.items[shelfBookKey{shelfID: wantToRead.ID, bookID: bookID}] = time.Now()

	shelves, err = svc.ListShelves(ctx, userID, &bookID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantToRead = findShelf(shelves, domain.ShelfTypeWantToRead)
	if wantToRead == nil || !wantToRead.HasBook {
		t.Fatal("expected has_book true for want_to_read")
	}
}

func TestGetShelf_PaginationDefaults(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf

	for i := 0; i < 5; i++ {
		bookID := uuid.New()
		repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
		repo.items[shelfBookKey{shelfID: shelf.ID, bookID: bookID}] = time.Now()
	}

	_, books, total, err := svc.GetShelf(ctx, userID, shelf.ID, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total 5, got %d", total)
	}
	if len(books) != 5 {
		t.Fatalf("expected 5 books with default limit, got %d", len(books))
	}
}

func TestListPublicByUser_ExcludesEmptyAndHiddenOnlyShelves(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	visibleBook := uuid.New()
	hiddenBook := uuid.New()
	repo.books[visibleBook] = &catalogDomain.Book{ID: visibleBook, Title: "Visible"}
	repo.books[hiddenBook] = &catalogDomain.Book{ID: hiddenBook, Title: "Hidden", IsHidden: true}

	withVisible := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "С видимой книгой", Type: domain.ShelfTypeCustom}
	onlyHidden := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Только скрытая", Type: domain.ShelfTypeCustom}
	empty := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Пустая", Type: domain.ShelfTypeCustom}
	repo.shelves[withVisible.ID] = withVisible
	repo.shelves[onlyHidden.ID] = onlyHidden
	repo.shelves[empty.ID] = empty

	repo.items[shelfBookKey{shelfID: withVisible.ID, bookID: visibleBook}] = time.Now()
	repo.items[shelfBookKey{shelfID: onlyHidden.ID, bookID: hiddenBook}] = time.Now()

	shelves, err := svc.ListPublicByUser(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(shelves) != 1 {
		t.Fatalf("expected 1 public shelf, got %d", len(shelves))
	}
	if shelves[0].ID != withVisible.ID {
		t.Fatalf("expected shelf with visible book, got %s", shelves[0].ID)
	}
	if shelves[0].BooksCount != 1 {
		t.Fatalf("expected books count 1, got %d", shelves[0].BooksCount)
	}
}

func TestListPublicByUser_NoShelves(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	shelves, err := svc.ListPublicByUser(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shelves == nil {
		t.Fatal("expected empty slice, not nil")
	}
	if len(shelves) != 0 {
		t.Fatalf("expected 0 shelves, got %d", len(shelves))
	}
}

func TestGetPublicShelf_AnyOwner(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: ownerID, Name: "Чужая", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf

	result, _, _, err := svc.GetPublicShelf(ctx, shelf.ID, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != shelf.ID {
		t.Fatal("expected to get shelf regardless of owner")
	}
}

func TestGetPublicShelf_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	_, _, _, err := svc.GetPublicShelf(ctx, uuid.New(), 1, 20)
	if !errors.Is(err, ErrShelfNotFound) {
		t.Fatalf("expected ErrShelfNotFound, got %v", err)
	}
}

func TestGetPublicShelf_ExcludesHiddenBooks(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom}
	repo.shelves[shelf.ID] = shelf

	visibleBook := uuid.New()
	hiddenBook := uuid.New()
	repo.books[visibleBook] = &catalogDomain.Book{ID: visibleBook, Title: "Visible"}
	repo.books[hiddenBook] = &catalogDomain.Book{ID: hiddenBook, Title: "Hidden", IsHidden: true}
	repo.items[shelfBookKey{shelfID: shelf.ID, bookID: visibleBook}] = time.Now()
	repo.items[shelfBookKey{shelfID: shelf.ID, bookID: hiddenBook}] = time.Now()

	_, books, total, err := svc.GetPublicShelf(ctx, shelf.ID, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(books) != 1 || books[0].ID != visibleBook {
		t.Fatalf("expected only visible book, got %+v", books)
	}
}

func findShelf(shelves []domain.Shelf, typ domain.ShelfType) *domain.Shelf {
	for i := range shelves {
		if shelves[i].Type == typ {
			return &shelves[i]
		}
	}
	return nil
}
