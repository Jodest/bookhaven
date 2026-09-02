package service

import (
	"context"
	"errors"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/lists/domain"
	"github.com/bookhaven/backend/internal/modules/lists/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type listBookKey struct {
	listID uuid.UUID
	bookID  uuid.UUID
}

type fakeRepository struct {
	lists      map[uuid.UUID]*domain.List
	items      map[listBookKey]int // position
	books      map[uuid.UUID]*catalogDomain.Book
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		lists: make(map[uuid.UUID]*domain.List),
		items: make(map[listBookKey]int),
		books: make(map[uuid.UUID]*catalogDomain.Book),
	}
}

func (f *fakeRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.List, error) {
	var result []domain.List
	for _, l := range f.lists {
		if l.UserID == userID {
			copy := *l
			var count int64
			for key := range f.items {
				if key.listID == copy.ID {
					count++
				}
			}
			copy.BooksCount = count
			result = append(result, copy)
		}
	}
	return result, nil
}

func (f *fakeRepository) GetByID(ctx context.Context, listID uuid.UUID) (*domain.List, error) {
	l, ok := f.lists[listID]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *l
	return &copy, nil
}

func (f *fakeRepository) GetPublicByID(ctx context.Context, listID uuid.UUID) (*domain.List, error) {
	l, ok := f.lists[listID]
	if !ok || !l.IsPublic {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *l
	return &copy, nil
}

func (f *fakeRepository) Create(ctx context.Context, list *domain.List) error {
	copy := *list
	f.lists[list.ID] = &copy
	return nil
}

func (f *fakeRepository) Update(ctx context.Context, list *domain.List) error {
	if _, ok := f.lists[list.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	copy := *list
	f.lists[list.ID] = &copy
	return nil
}

func (f *fakeRepository) Delete(ctx context.Context, listID uuid.UUID) error {
	if _, ok := f.lists[listID]; !ok {
		return gorm.ErrRecordNotFound
	}
	delete(f.lists, listID)
	for key := range f.items {
		if key.listID == listID {
			delete(f.items, key)
		}
	}
	return nil
}

func (f *fakeRepository) AddBook(ctx context.Context, listID, bookID uuid.UUID, position int) error {
	key := listBookKey{listID: listID, bookID: bookID}
	if _, ok := f.items[key]; ok {
		return repository.ErrBookAlreadyOnList
	}
	f.items[key] = position
	return nil
}

func (f *fakeRepository) RemoveBook(ctx context.Context, listID, bookID uuid.UUID) error {
	key := listBookKey{listID: listID, bookID: bookID}
	if _, ok := f.items[key]; !ok {
		return repository.ErrBookNotOnList
	}
	delete(f.items, key)
	return nil
}

func (f *fakeRepository) BookOnList(ctx context.Context, listID, bookID uuid.UUID) (bool, error) {
	_, ok := f.items[listBookKey{listID: listID, bookID: bookID}]
	return ok, nil
}

func (f *fakeRepository) ListBooks(ctx context.Context, listID uuid.UUID, limit, offset int) ([]catalogDomain.Book, int64, error) {
	var books []catalogDomain.Book
	for key := range f.items {
		if key.listID == listID {
			if b, ok := f.books[key.bookID]; ok {
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
	_, ok := f.books[bookID]
	return ok, nil
}

func (f *fakeRepository) MaxPosition(ctx context.Context, listID uuid.UUID) (int, error) {
	max := 0
	for key, pos := range f.items {
		if key.listID == listID && pos > max {
			max = pos
		}
	}
	return max, nil
}

func (f *fakeRepository) ReorderBooks(ctx context.Context, listID uuid.UUID, bookIDs []uuid.UUID) error {
	for i, bookID := range bookIDs {
		key := listBookKey{listID: listID, bookID: bookID}
		if _, ok := f.items[key]; ok {
			f.items[key] = i
		}
	}
	return nil
}

func (f *fakeRepository) UserListsWithBook(ctx context.Context, userID, bookID uuid.UUID) (map[uuid.UUID]struct{}, error) {
	result := make(map[uuid.UUID]struct{})
	for key := range f.items {
		if key.bookID == bookID {
			if l, ok := f.lists[key.listID]; ok && l.UserID == userID {
				result[key.listID] = struct{}{}
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

func TestCreateList(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	desc := "Мои любимые книги"
	list, err := svc.CreateList(ctx, userID, "Классика", &desc, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if list.Title != "Классика" {
		t.Fatalf("expected title Классика, got %s", list.Title)
	}
	if list.Description != desc {
		t.Fatalf("expected description %q, got %q", desc, list.Description)
	}
	if !list.IsPublic {
		t.Fatal("expected public list")
	}
}

func TestCreateList_InvalidTitle(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	cases := []string{"", "a", "   "}
	for _, title := range cases {
		_, err := svc.CreateList(ctx, userID, title, nil, true)
		if !errors.Is(err, ErrInvalidTitle) {
			t.Fatalf("expected ErrInvalidTitle for %q, got %v", title, err)
		}
	}
}

func TestCreateList_DescriptionTooLong(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	longDesc := ""
	for i := 0; i < 2001; i++ {
		longDesc += "a"
	}
	_, err := svc.CreateList(ctx, userID, "Классика", &longDesc, true)
	if !errors.Is(err, ErrInvalidDescription) {
		t.Fatalf("expected ErrInvalidDescription, got %v", err)
	}
}

func TestUpdateList(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Старое", Description: "Старое описание", IsPublic: true}
	repo.lists[list.ID] = list

	newTitle := "Новое"
	newDesc := "Новое описание"
	isPublic := false
	updated, err := svc.UpdateList(ctx, userID, list.ID, &newTitle, &newDesc, &isPublic)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Title != "Новое" {
		t.Fatalf("expected title Новое, got %s", updated.Title)
	}
	if updated.Description != "Новое описание" {
		t.Fatalf("expected description Новое описание, got %s", updated.Description)
	}
	if updated.IsPublic {
		t.Fatal("expected private list")
	}
}

func TestUpdateList_NotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	newTitle := "Новое"
	_, err := svc.UpdateList(ctx, userID, uuid.New(), &newTitle, nil, nil)
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestUpdateList_Forbidden(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: ownerID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	newTitle := "Новое"
	_, err := svc.UpdateList(ctx, otherID, list.ID, &newTitle, nil, nil)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestDeleteList(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	if err := svc.DeleteList(ctx, userID, list.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.lists[list.ID]; ok {
		t.Fatal("expected list to be deleted")
	}
}

func TestDeleteList_Forbidden(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: ownerID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	err := svc.DeleteList(ctx, otherID, list.ID)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestAddBook(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list
	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	if err := svc.AddBook(ctx, userID, list.ID, bookID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := repo.items[listBookKey{listID: list.ID, bookID: bookID}]; !ok {
		t.Fatal("expected book to be added")
	}
}

func TestAddBook_Duplicate(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list
	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	if err := svc.AddBook(ctx, userID, list.ID, bookID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err := svc.AddBook(ctx, userID, list.ID, bookID)
	if !errors.Is(err, ErrBookAlreadyOnList) {
		t.Fatalf("expected ErrBookAlreadyOnList, got %v", err)
	}
}

func TestAddBook_BookNotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	err := svc.AddBook(ctx, userID, list.ID, uuid.New())
	if !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected ErrBookNotFound, got %v", err)
	}
}

func TestRemoveBook(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list
	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
	repo.items[listBookKey{listID: list.ID, bookID: bookID}] = 1

	if err := svc.RemoveBook(ctx, userID, list.ID, bookID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := repo.items[listBookKey{listID: list.ID, bookID: bookID}]; ok {
		t.Fatal("expected book to be removed")
	}
}

func TestRemoveBook_NotInList(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	err := svc.RemoveBook(ctx, userID, list.ID, uuid.New())
	if !errors.Is(err, ErrBookNotOnList) {
		t.Fatalf("expected ErrBookNotOnList, got %v", err)
	}
}

func TestReorderBooks(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	book1 := uuid.New()
	book2 := uuid.New()
	book3 := uuid.New()
	repo.books[book1] = &catalogDomain.Book{ID: book1, Title: "Book1"}
	repo.books[book2] = &catalogDomain.Book{ID: book2, Title: "Book2"}
	repo.books[book3] = &catalogDomain.Book{ID: book3, Title: "Book3"}
	repo.items[listBookKey{listID: list.ID, bookID: book1}] = 1
	repo.items[listBookKey{listID: list.ID, bookID: book2}] = 2
	repo.items[listBookKey{listID: list.ID, bookID: book3}] = 3

	// Reorder: book3, book1, book2
	err := svc.ReorderBooks(ctx, userID, list.ID, []uuid.UUID{book3, book1, book2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.items[listBookKey{listID: list.ID, bookID: book3}] != 0 {
		t.Fatalf("expected book3 at position 0, got %d", repo.items[listBookKey{listID: list.ID, bookID: book3}])
	}
	if repo.items[listBookKey{listID: list.ID, bookID: book1}] != 1 {
		t.Fatalf("expected book1 at position 1, got %d", repo.items[listBookKey{listID: list.ID, bookID: book1}])
	}
	if repo.items[listBookKey{listID: list.ID, bookID: book2}] != 2 {
		t.Fatalf("expected book2 at position 2, got %d", repo.items[listBookKey{listID: list.ID, bookID: book2}])
	}
}

func TestReorderBooks_Forbidden(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: ownerID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	err := svc.ReorderBooks(ctx, otherID, list.ID, []uuid.UUID{})
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestGetPublicList_Public(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	result, _, _, err := svc.GetPublicList(ctx, list.ID, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != list.ID {
		t.Fatal("expected to get public list")
	}
}

func TestGetPublicList_PrivateForbidden(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: false}
	repo.lists[list.ID] = list

	_, _, _, err := svc.GetPublicList(ctx, list.ID, 1, 20)
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound for private list, got %v", err)
	}
}

func TestGetList_Forbidden(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: ownerID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	_, _, _, err := svc.GetList(ctx, otherID, list.ID, 1, 20)
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestAddBook_Position(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Моя", Description: "", IsPublic: true}
	repo.lists[list.ID] = list
	book1 := uuid.New()
	book2 := uuid.New()
	repo.books[book1] = &catalogDomain.Book{ID: book1, Title: "Book1"}
	repo.books[book2] = &catalogDomain.Book{ID: book2, Title: "Book2"}

	if err := svc.AddBook(ctx, userID, list.ID, book1); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := svc.AddBook(ctx, userID, list.ID, book2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	pos1 := repo.items[listBookKey{listID: list.ID, bookID: book1}]
	pos2 := repo.items[listBookKey{listID: list.ID, bookID: book2}]
	if pos1 != 1 {
		t.Fatalf("expected book1 at position 1, got %d", pos1)
	}
	if pos2 != 2 {
		t.Fatalf("expected book2 at position 2, got %d", pos2)
	}
}

func TestListLists_WithBookMarking(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	list1 := &domain.List{ID: uuid.New(), UserID: userID, Title: "List1", Description: "", IsPublic: true}
	list2 := &domain.List{ID: uuid.New(), UserID: userID, Title: "List2", Description: "", IsPublic: true}
	repo.lists[list1.ID] = list1
	repo.lists[list2.ID] = list2
	repo.items[listBookKey{listID: list1.ID, bookID: bookID}] = 1

	lists, inLists, err := svc.ListLists(ctx, userID, &bookID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lists) != 2 {
		t.Fatalf("expected 2 lists, got %d", len(lists))
	}
	if len(inLists) != 1 {
		t.Fatalf("expected 1 list containing book, got %d", len(inLists))
	}
	if _, ok := inLists[list1.ID]; !ok {
		t.Fatalf("expected list1 to contain the book")
	}
}

// Ensure time import is used
var _ = time.Now
