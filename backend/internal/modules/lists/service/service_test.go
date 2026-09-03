package service

import (
	"context"
	"errors"
	"sort"
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
	bookID uuid.UUID
}

type fakeRepository struct {
	lists map[uuid.UUID]*domain.List
	items map[listBookKey]domain.ListItem
	books map[uuid.UUID]*catalogDomain.Book
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		lists: make(map[uuid.UUID]*domain.List),
		items: make(map[listBookKey]domain.ListItem),
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
	f.items[key] = domain.ListItem{ListID: listID, BookID: bookID, Position: position, AddedAt: time.Now().UTC()}
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
	type item struct {
		bookID   uuid.UUID
		position int
		addedAt  time.Time
	}
	var items []item
	for key, it := range f.items {
		if key.listID == listID {
			items = append(items, item{bookID: key.bookID, position: it.Position, addedAt: it.AddedAt})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].position != items[j].position {
			return items[i].position < items[j].position
		}
		if !items[i].addedAt.Equal(items[j].addedAt) {
			return items[i].addedAt.Before(items[j].addedAt)
		}
		return items[i].bookID.String() < items[j].bookID.String()
	})

	var books []catalogDomain.Book
	for _, it := range items {
		if b, ok := f.books[it.bookID]; ok {
			books = append(books, *b)
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
	for key, it := range f.items {
		if key.listID == listID && it.Position > max {
			max = it.Position
		}
	}
	return max, nil
}

func (f *fakeRepository) ReorderBooks(ctx context.Context, listID uuid.UUID, bookIDs []uuid.UUID) error {
	for i, bookID := range bookIDs {
		key := listBookKey{listID: listID, bookID: bookID}
		if it, ok := f.items[key]; ok {
			it.Position = i
			f.items[key] = it
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
	repo.items[listBookKey{listID: list.ID, bookID: bookID}] = domain.ListItem{ListID: list.ID, BookID: bookID, Position: 1}

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
	repo.items[listBookKey{listID: list.ID, bookID: book1}] = domain.ListItem{ListID: list.ID, BookID: book1, Position: 1}
	repo.items[listBookKey{listID: list.ID, bookID: book2}] = domain.ListItem{ListID: list.ID, BookID: book2, Position: 2}
	repo.items[listBookKey{listID: list.ID, bookID: book3}] = domain.ListItem{ListID: list.ID, BookID: book3, Position: 3}

	// Reorder: book3, book1, book2
	err := svc.ReorderBooks(ctx, userID, list.ID, []uuid.UUID{book3, book1, book2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if repo.items[listBookKey{listID: list.ID, bookID: book3}].Position != 0 {
		t.Fatalf("expected book3 at position 0, got %d", repo.items[listBookKey{listID: list.ID, bookID: book3}].Position)
	}
	if repo.items[listBookKey{listID: list.ID, bookID: book1}].Position != 1 {
		t.Fatalf("expected book1 at position 1, got %d", repo.items[listBookKey{listID: list.ID, bookID: book1}].Position)
	}
	if repo.items[listBookKey{listID: list.ID, bookID: book2}].Position != 2 {
		t.Fatalf("expected book2 at position 2, got %d", repo.items[listBookKey{listID: list.ID, bookID: book2}].Position)
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

	pos1 := repo.items[listBookKey{listID: list.ID, bookID: book1}].Position
	pos2 := repo.items[listBookKey{listID: list.ID, bookID: book2}].Position
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
	repo.items[listBookKey{listID: list1.ID, bookID: bookID}] = domain.ListItem{ListID: list1.ID, BookID: bookID, Position: 1}

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

func TestListLists(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	otherID := uuid.New()
	svc, repo := setupTestService()

	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}

	ownerList := &domain.List{ID: uuid.New(), UserID: userID, Title: "Мой", Description: "Описание", IsPublic: false}
	publicList := &domain.List{ID: uuid.New(), UserID: userID, Title: "Публичный", Description: "", IsPublic: true}
	otherList := &domain.List{ID: uuid.New(), UserID: otherID, Title: "Чужой", Description: "", IsPublic: true}
	repo.lists[ownerList.ID] = ownerList
	repo.lists[publicList.ID] = publicList
	repo.lists[otherList.ID] = otherList
	repo.items[listBookKey{listID: ownerList.ID, bookID: bookID}] = domain.ListItem{ListID: ownerList.ID, BookID: bookID, Position: 1}
	repo.items[listBookKey{listID: publicList.ID, bookID: bookID}] = domain.ListItem{ListID: publicList.ID, BookID: bookID, Position: 1}

	lists, _, err := svc.ListLists(ctx, userID, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lists) != 2 {
		t.Fatalf("expected 2 lists for owner, got %d", len(lists))
	}

	ids := make(map[uuid.UUID]struct{}, len(lists))
	counts := make(map[uuid.UUID]int64, len(lists))
	for _, l := range lists {
		ids[l.ID] = struct{}{}
		counts[l.ID] = l.BooksCount
	}
	if _, ok := ids[otherList.ID]; ok {
		t.Fatal("expected other user's list to be excluded")
	}
	if counts[ownerList.ID] != 1 {
		t.Fatalf("expected owner list count 1, got %d", counts[ownerList.ID])
	}
	if counts[publicList.ID] != 1 {
		t.Fatalf("expected public list count 1, got %d", counts[publicList.ID])
	}
}

func TestGetList_Owner(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Мой", Description: "Описание", IsPublic: false}
	repo.lists[list.ID] = list

	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{
		ID:       bookID,
		Title:    "Книга",
		Slug:     "kniga",
		CoverURL: strPtr("https://example.com/cover.jpg"),
		Authors:  []catalogDomain.Author{{Name: "Автор", Slug: "avtor"}},
		Genres:   []catalogDomain.Genre{{Name: "Жанр", Slug: "zhanr"}},
	}
	repo.items[listBookKey{listID: list.ID, bookID: bookID}] = domain.ListItem{ListID: list.ID, BookID: bookID, Position: 1}

	result, books, total, err := svc.GetList(ctx, userID, list.ID, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != list.ID {
		t.Fatal("expected list to be returned")
	}
	if result.UserID != userID {
		t.Fatal("expected list owner")
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(books) != 1 {
		t.Fatalf("expected 1 book, got %d", len(books))
	}
	if books[0].ID != bookID {
		t.Fatalf("expected book %s, got %s", bookID, books[0].ID)
	}
	if len(books[0].Authors) != 1 || books[0].Authors[0].Name != "Автор" {
		t.Fatalf("expected author metadata, got %+v", books[0].Authors)
	}
	if len(books[0].Genres) != 1 || books[0].Genres[0].Name != "Жанр" {
		t.Fatalf("expected genre metadata, got %+v", books[0].Genres)
	}
}

func TestGetList_NotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	_, _, _, err := svc.GetList(ctx, userID, uuid.New(), 1, 20)
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestGetList_PaginationDefaults(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Мой", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	for i := 0; i < 5; i++ {
		bookID := uuid.New()
		repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
		repo.items[listBookKey{listID: list.ID, bookID: bookID}] = domain.ListItem{ListID: list.ID, BookID: bookID, Position: i}
	}

	_, books, total, err := svc.GetList(ctx, userID, list.ID, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total 5, got %d", total)
	}
	if len(books) != 5 {
		t.Fatalf("expected 5 books with default limit, got %d", len(books))
	}

	_, books, _, err = svc.GetList(ctx, userID, list.ID, 2, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(books) != 2 {
		t.Fatalf("expected 2 books on page 2, got %d", len(books))
	}
}

func TestGetPublicList_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	_, _, _, err := svc.GetPublicList(ctx, uuid.New(), 1, 20)
	if !errors.Is(err, ErrListNotFound) {
		t.Fatalf("expected ErrListNotFound, got %v", err)
	}
}

func TestGetPublicList_Pagination(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Публичный", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	for i := 0; i < 5; i++ {
		bookID := uuid.New()
		repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Book"}
		repo.items[listBookKey{listID: list.ID, bookID: bookID}] = domain.ListItem{ListID: list.ID, BookID: bookID, Position: i}
	}

	_, books, total, err := svc.GetPublicList(ctx, list.ID, 2, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total 5, got %d", total)
	}
	if len(books) != 2 {
		t.Fatalf("expected 2 books on page 2, got %d", len(books))
	}

	_, books, _, err = svc.GetPublicList(ctx, list.ID, 10, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(books) != 0 {
		t.Fatalf("expected 0 books beyond last page, got %d", len(books))
	}
}

func TestGetPublicList_Ordering(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Публичный", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	bookA := uuid.New()
	bookB := uuid.New()
	bookC := uuid.New()
	repo.books[bookA] = &catalogDomain.Book{ID: bookA, Title: "A"}
	repo.books[bookB] = &catalogDomain.Book{ID: bookB, Title: "B"}
	repo.books[bookC] = &catalogDomain.Book{ID: bookC, Title: "C"}
	repo.items[listBookKey{listID: list.ID, bookID: bookA}] = domain.ListItem{ListID: list.ID, BookID: bookA, Position: 3}
	repo.items[listBookKey{listID: list.ID, bookID: bookB}] = domain.ListItem{ListID: list.ID, BookID: bookB, Position: 1}
	repo.items[listBookKey{listID: list.ID, bookID: bookC}] = domain.ListItem{ListID: list.ID, BookID: bookC, Position: 2}

	_, books, _, err := svc.GetPublicList(ctx, list.ID, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(books) != 3 {
		t.Fatalf("expected 3 books, got %d", len(books))
	}
	if books[0].Title != "B" || books[1].Title != "C" || books[2].Title != "A" {
		t.Fatalf("expected order B, C, A, got %s, %s, %s", books[0].Title, books[1].Title, books[2].Title)
	}
}

func strPtr(s string) *string {
	return &s
}
