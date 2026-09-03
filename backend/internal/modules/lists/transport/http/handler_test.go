package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/lists/domain"
	"github.com/bookhaven/backend/internal/modules/lists/repository"
	"github.com/bookhaven/backend/internal/modules/lists/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
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
			copy.BooksCount = f.countBooks(copy.ID)
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
	copy.BooksCount = f.countBooks(copy.ID)
	return &copy, nil
}

func (f *fakeRepository) GetPublicByID(ctx context.Context, listID uuid.UUID) (*domain.List, error) {
	l, ok := f.lists[listID]
	if !ok || !l.IsPublic {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *l
	copy.BooksCount = f.countBooks(copy.ID)
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
	f.items[key] = domain.ListItem{ListID: listID, BookID: bookID, Position: position}
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
		addedAt  int64
	}
	var items []item
	for key, it := range f.items {
		if key.listID == listID {
			if b, ok := f.books[key.bookID]; ok && !b.IsHidden {
				items = append(items, item{bookID: key.bookID, position: it.Position, addedAt: it.AddedAt.UnixNano()})
			}
		}
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].position > items[j].position ||
				(items[i].position == items[j].position && items[i].addedAt > items[j].addedAt) ||
				(items[i].position == items[j].position && items[i].addedAt == items[j].addedAt && items[i].bookID.String() > items[j].bookID.String()) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	var books []catalogDomain.Book
	for _, it := range items {
		books = append(books, *f.books[it.bookID])
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

func (f *fakeRepository) countBooks(listID uuid.UUID) int64 {
	var count int64
	for key := range f.items {
		if key.listID == listID {
			count++
		}
	}
	return count
}

func setupHandler() (*Handler, *fakeRepository) {
	repo := newFakeRepository()
	return NewHandler(service.NewService(repo)), repo
}

func newContext(method, target string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestListLists_RequiresAuth(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/lists")

	if err := h.ListLists(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestListLists_OwnerSeesLists(t *testing.T) {
	h, repo := setupHandler()
	userID := uuid.New()
	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Мой", Description: "Описание", IsPublic: true}
	repo.lists[list.ID] = list

	c, rec := newContext(http.MethodGet, "/api/v1/lists")
	c.Set("user_id", userID.String())

	if err := h.ListLists(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	var items []map[string]interface{}
	if err := json.Unmarshal(body["items"], &items); err != nil {
		t.Fatalf("failed to decode items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 list, got %d", len(items))
	}
	if items[0]["title"] != "Мой" {
		t.Fatalf("expected title Мой, got %v", items[0]["title"])
	}
}

func TestGetList_InvalidID(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/lists/bad-id")
	c.SetPath("/api/v1/lists/:id")
	c.SetParamNames("id")
	c.SetParamValues("bad-id")
	c.Set("user_id", uuid.New().String())

	if err := h.GetList(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetList_PrivateDeniedForOtherUser(t *testing.T) {
	h, repo := setupHandler()
	ownerID := uuid.New()
	otherID := uuid.New()
	list := &domain.List{ID: uuid.New(), UserID: ownerID, Title: "Мой", Description: "", IsPublic: false}
	repo.lists[list.ID] = list

	c, rec := newContext(http.MethodGet, "/api/v1/lists/"+list.ID.String())
	c.SetPath("/api/v1/lists/:id")
	c.SetParamNames("id")
	c.SetParamValues(list.ID.String())
	c.Set("user_id", otherID.String())

	if err := h.GetList(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

func TestGetList_Pagination(t *testing.T) {
	h, repo := setupHandler()
	userID := uuid.New()
	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Мой", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	for i := 0; i < 5; i++ {
		bookID := uuid.New()
		repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: strings.Repeat("B", i+1), Slug: strings.Repeat("b", i+1)}
		repo.items[listBookKey{listID: list.ID, bookID: bookID}] = domain.ListItem{ListID: list.ID, BookID: bookID, Position: i}
	}

	c, rec := newContext(http.MethodGet, "/api/v1/lists/"+list.ID.String()+"?page=2&limit=2")
	c.SetPath("/api/v1/lists/:id")
	c.SetParamNames("id")
	c.SetParamValues(list.ID.String())
	c.Set("user_id", userID.String())

	if err := h.GetList(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body listDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Page != 2 {
		t.Fatalf("expected page 2, got %d", body.Page)
	}
	if body.Limit != 2 {
		t.Fatalf("expected limit 2, got %d", body.Limit)
	}
	if body.Total != 5 {
		t.Fatalf("expected total 5, got %d", body.Total)
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(body.Items))
	}
}

func TestGetPublicList_PublicAccessWithoutAuth(t *testing.T) {
	h, repo := setupHandler()
	ownerID := uuid.New()
	list := &domain.List{ID: uuid.New(), UserID: ownerID, Title: "Публичный", Description: "Описание", IsPublic: true}
	repo.lists[list.ID] = list

	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Книга", Slug: "kniga"}
	repo.items[listBookKey{listID: list.ID, bookID: bookID}] = domain.ListItem{ListID: list.ID, BookID: bookID, Position: 1}

	c, rec := newContext(http.MethodGet, "/api/v1/lists/public/"+list.ID.String())
	c.SetPath("/api/v1/lists/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(list.ID.String())

	if err := h.GetPublicList(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body publicListDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.List.Title != "Публичный" {
		t.Fatalf("expected title Публичный, got %s", body.List.Title)
	}
	if body.Total != 1 {
		t.Fatalf("expected total 1, got %d", body.Total)
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(body.Items))
	}
}

func TestGetPublicList_PrivateNotFound(t *testing.T) {
	h, repo := setupHandler()
	ownerID := uuid.New()
	list := &domain.List{ID: uuid.New(), UserID: ownerID, Title: "Приватный", Description: "", IsPublic: false}
	repo.lists[list.ID] = list

	c, rec := newContext(http.MethodGet, "/api/v1/lists/public/"+list.ID.String())
	c.SetPath("/api/v1/lists/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(list.ID.String())

	if err := h.GetPublicList(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetPublicList_ResponsePrivacy(t *testing.T) {
	h, repo := setupHandler()
	ownerID := uuid.New()
	list := &domain.List{ID: uuid.New(), UserID: ownerID, Title: "Публичный", Description: "Описание", IsPublic: true}
	repo.lists[list.ID] = list

	c, rec := newContext(http.MethodGet, "/api/v1/lists/public/"+list.ID.String())
	c.SetPath("/api/v1/lists/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(list.ID.String())

	if err := h.GetPublicList(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	var listObj map[string]interface{}
	if err := json.Unmarshal(raw["list"], &listObj); err != nil {
		t.Fatalf("failed to decode list: %v", err)
	}
	if _, ok := listObj["user_id"]; ok {
		t.Fatal("public response must not expose user_id")
	}
	if listObj["title"] != "Публичный" {
		t.Fatalf("expected title metadata, got %v", listObj["title"])
	}
	if listObj["books_count"] == nil {
		t.Fatal("expected books_count metadata")
	}
}

func TestGetPublicList_InvalidID(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/lists/public/bad-id")
	c.SetPath("/api/v1/lists/public/:id")
	c.SetParamNames("id")
	c.SetParamValues("bad-id")

	if err := h.GetPublicList(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetList_OrderingByPositionAndAddedAt(t *testing.T) {
	h, repo := setupHandler()
	userID := uuid.New()
	list := &domain.List{ID: uuid.New(), UserID: userID, Title: "Ordered", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	now := time.Now()

	bookA := uuid.New()
	repo.books[bookA] = &catalogDomain.Book{ID: bookA, Title: "A", Slug: "a"}
	repo.items[listBookKey{listID: list.ID, bookID: bookA}] = domain.ListItem{
		ListID: list.ID, BookID: bookA, Position: 0, AddedAt: now.Add(-2 * time.Hour),
	}

	bookB := uuid.New()
	repo.books[bookB] = &catalogDomain.Book{ID: bookB, Title: "B", Slug: "b"}
	repo.items[listBookKey{listID: list.ID, bookID: bookB}] = domain.ListItem{
		ListID: list.ID, BookID: bookB, Position: 0, AddedAt: now.Add(-1 * time.Hour),
	}

	bookC := uuid.New()
	repo.books[bookC] = &catalogDomain.Book{ID: bookC, Title: "C", Slug: "c"}
	repo.items[listBookKey{listID: list.ID, bookID: bookC}] = domain.ListItem{
		ListID: list.ID, BookID: bookC, Position: 0, AddedAt: now,
	}

	bookD := uuid.New()
	repo.books[bookD] = &catalogDomain.Book{ID: bookD, Title: "D", Slug: "d"}
	repo.items[listBookKey{listID: list.ID, bookID: bookD}] = domain.ListItem{
		ListID: list.ID, BookID: bookD, Position: 1, AddedAt: now.Add(-3 * time.Hour),
	}

	c, rec := newContext(http.MethodGet, "/api/v1/lists/"+list.ID.String()+"?page=1&limit=2")
	c.SetPath("/api/v1/lists/:id")
	c.SetParamNames("id")
	c.SetParamValues(list.ID.String())
	c.Set("user_id", userID.String())

	if err := h.GetList(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body listDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Total != 4 {
		t.Fatalf("expected total 4, got %d", body.Total)
	}
	if len(body.Items) != 2 {
		t.Fatalf("expected 2 items on page 1, got %d", len(body.Items))
	}
	if body.Items[0].Title != "A" || body.Items[1].Title != "B" {
		t.Fatalf("expected page 1 order A, B, got %s, %s", body.Items[0].Title, body.Items[1].Title)
	}

	c2, rec2 := newContext(http.MethodGet, "/api/v1/lists/"+list.ID.String()+"?page=2&limit=2")
	c2.SetPath("/api/v1/lists/:id")
	c2.SetParamNames("id")
	c2.SetParamValues(list.ID.String())
	c2.Set("user_id", userID.String())

	if err := h.GetList(c2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body2 listDetailResponse
	if err := json.Unmarshal(rec2.Body.Bytes(), &body2); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(body2.Items) != 2 {
		t.Fatalf("expected 2 items on page 2, got %d", len(body2.Items))
	}
	if body2.Items[0].Title != "C" || body2.Items[1].Title != "D" {
		t.Fatalf("expected page 2 order C, D, got %s, %s", body2.Items[0].Title, body2.Items[1].Title)
	}
}

func TestGetPublicList_Pagination(t *testing.T) {
	h, repo := setupHandler()
	ownerID := uuid.New()
	list := &domain.List{ID: uuid.New(), UserID: ownerID, Title: "Публичный", Description: "", IsPublic: true}
	repo.lists[list.ID] = list

	for i := 0; i < 4; i++ {
		bookID := uuid.New()
		repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: strings.Repeat("B", i+1), Slug: strings.Repeat("b", i+1)}
		repo.items[listBookKey{listID: list.ID, bookID: bookID}] = domain.ListItem{ListID: list.ID, BookID: bookID, Position: i}
	}

	c, rec := newContext(http.MethodGet, "/api/v1/lists/public/"+list.ID.String()+"?page=3&limit=2")
	c.SetPath("/api/v1/lists/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(list.ID.String())

	if err := h.GetPublicList(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body publicListDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Page != 3 {
		t.Fatalf("expected page 3, got %d", body.Page)
	}
	if body.Total != 4 {
		t.Fatalf("expected total 4, got %d", body.Total)
	}
	if len(body.Items) != 0 {
		t.Fatalf("expected 0 items beyond last page, got %d", len(body.Items))
	}
}
