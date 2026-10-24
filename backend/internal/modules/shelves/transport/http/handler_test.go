package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/infrastructure/validator"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/domain"
	"github.com/bookhaven/backend/internal/modules/shelves/repository"
	"github.com/bookhaven/backend/internal/modules/shelves/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type shelfBookKey struct {
	shelfID uuid.UUID
	bookID  uuid.UUID
}

type fakeRepository struct {
	shelves map[uuid.UUID]*domain.Shelf
	items   map[shelfBookKey]time.Time
	books   map[uuid.UUID]*catalogDomain.Book
	reads   []domain.BookRead
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		shelves: make(map[uuid.UUID]*domain.Shelf),
		items:   make(map[shelfBookKey]time.Time),
		books:   make(map[uuid.UUID]*catalogDomain.Book),
	}
}

func (f *fakeRepository) EnsureDefaultShelves(ctx context.Context, userID uuid.UUID) error {
	return nil
}

func (f *fakeRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error) {
	var result []domain.Shelf
	for _, s := range f.shelves {
		if s.UserID == userID {
			copy := *s
			result = append(result, copy)
		}
	}
	return result, nil
}

func (f *fakeRepository) ListByUserVisible(ctx context.Context, userID uuid.UUID) ([]domain.Shelf, error) {
	return f.ListByUser(ctx, userID)
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
			if b, ok := f.books[key.bookID]; ok && (b.Status == "" || b.IsPublished()) {
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
	return ok && (b.Status == "" || b.IsPublished()), nil
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

func TestGetPublicShelf_AnyOwnerWithoutAuth(t *testing.T) {
	h, repo := setupHandler()
	ownerID := uuid.New()
	shelf := &domain.Shelf{ID: uuid.New(), UserID: ownerID, Name: "Чужая полка", Type: domain.ShelfTypeCustom, IsPublic: true}
	repo.shelves[shelf.ID] = shelf

	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Книга", Slug: "kniga"}
	repo.items[shelfBookKey{shelfID: shelf.ID, bookID: bookID}] = time.Now()

	// No user_id set on context: anonymous caller.
	c, rec := newContext(http.MethodGet, "/api/v1/shelves/public/"+shelf.ID.String())
	c.SetPath("/api/v1/shelves/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(shelf.ID.String())

	if err := h.GetPublicShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body shelfDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Shelf.Name != "Чужая полка" {
		t.Fatalf("expected shelf name Чужая полка, got %s", body.Shelf.Name)
	}
	if body.Total != 1 {
		t.Fatalf("expected total 1, got %d", body.Total)
	}
	if len(body.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(body.Items))
	}
}

func TestGetPublicShelf_InvalidID(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/shelves/public/bad-id")
	c.SetPath("/api/v1/shelves/public/:id")
	c.SetParamNames("id")
	c.SetParamValues("bad-id")

	if err := h.GetPublicShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestGetPublicShelf_NotFound(t *testing.T) {
	h, _ := setupHandler()
	missingID := uuid.New()
	c, rec := newContext(http.MethodGet, "/api/v1/shelves/public/"+missingID.String())
	c.SetPath("/api/v1/shelves/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(missingID.String())

	if err := h.GetPublicShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetPublicShelf_Pagination(t *testing.T) {
	h, repo := setupHandler()
	userID := uuid.New()
	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom, IsPublic: true}
	repo.shelves[shelf.ID] = shelf

	for i := 0; i < 5; i++ {
		bookID := uuid.New()
		repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Книга", Slug: "kniga-" + bookID.String()}
		repo.items[shelfBookKey{shelfID: shelf.ID, bookID: bookID}] = time.Now()
	}

	c, rec := newContext(http.MethodGet, "/api/v1/shelves/public/"+shelf.ID.String()+"?page=2&limit=2")
	c.SetPath("/api/v1/shelves/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(shelf.ID.String())

	if err := h.GetPublicShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body shelfDetailResponse
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

func TestGetPublicShelf_HiddenBooksExcluded(t *testing.T) {
	h, repo := setupHandler()
	userID := uuid.New()
	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom, IsPublic: true}
	repo.shelves[shelf.ID] = shelf

	visibleBook := uuid.New()
	hiddenBook := uuid.New()
	repo.books[visibleBook] = &catalogDomain.Book{ID: visibleBook, Title: "Видимая", Slug: "vidimaya"}
	repo.books[hiddenBook] = &catalogDomain.Book{ID: hiddenBook, Title: "Скрытая", Slug: "skrytaya", Status: catalogDomain.BookStatusRejected}
	repo.items[shelfBookKey{shelfID: shelf.ID, bookID: visibleBook}] = time.Now()
	repo.items[shelfBookKey{shelfID: shelf.ID, bookID: hiddenBook}] = time.Now()

	c, rec := newContext(http.MethodGet, "/api/v1/shelves/public/"+shelf.ID.String())
	c.SetPath("/api/v1/shelves/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(shelf.ID.String())

	if err := h.GetPublicShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body shelfDetailResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Total != 1 {
		t.Fatalf("expected total 1, got %d", body.Total)
	}
	if len(body.Items) != 1 || body.Items[0].Title != "Видимая" {
		t.Fatalf("expected only visible book, got %+v", body.Items)
	}
}

func jsonContext(method, target, body, userID string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Validator = validator.New()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("user_id", userID)
	return c, rec
}

func TestCreateShelf_WithDescriptionAndPrivacy(t *testing.T) {
	h, repo := setupHandler()
	userID := uuid.New()

	c, rec := jsonContext(http.MethodPost, "/api/v1/shelves", `{"name":"Отпуск","description":"Для пляжа","is_public":false}`, userID.String())
	if err := h.CreateShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
	var body shelfResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Description != "Для пляжа" || body.IsPublic {
		t.Fatalf("unexpected body %+v", body)
	}
	stored := repo.shelves[uuid.MustParse(body.ID)]
	if stored == nil || stored.IsPublic {
		t.Fatal("private shelf must be stored as private")
	}
}

func TestCreateShelf_DefaultsToPublic(t *testing.T) {
	h, _ := setupHandler()
	c, rec := jsonContext(http.MethodPost, "/api/v1/shelves", `{"name":"Классика"}`, uuid.New().String())
	if err := h.CreateShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body shelfResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if rec.Code != http.StatusCreated || !body.IsPublic || body.Description != "" {
		t.Fatalf("unexpected response %d %+v", rec.Code, body)
	}
}

func patchShelf(h *Handler, shelfID, userID uuid.UUID, body string) *httptest.ResponseRecorder {
	c, rec := jsonContext(http.MethodPatch, "/api/v1/shelves/"+shelfID.String(), body, userID.String())
	c.SetPath("/api/v1/shelves/:id")
	c.SetParamNames("id")
	c.SetParamValues(shelfID.String())
	_ = h.UpdateShelf(c)
	return rec
}

func TestUpdateShelf_TogglePrivacy(t *testing.T) {
	h, repo := setupHandler()
	userID := uuid.New()
	shelf := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom, IsPublic: true}
	repo.shelves[shelf.ID] = shelf

	rec := patchShelf(h, shelf.ID, userID, `{"is_public":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var body shelfResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	if body.IsPublic || body.Name != "Моя" {
		t.Fatalf("unexpected body %+v", body)
	}
}

func TestUpdateShelf_DefaultShelfAndEmptyBodyRejected(t *testing.T) {
	h, repo := setupHandler()
	userID := uuid.New()
	def := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Читаю", Type: domain.ShelfTypeReading, IsPublic: true}
	custom := &domain.Shelf{ID: uuid.New(), UserID: userID, Name: "Моя", Type: domain.ShelfTypeCustom, IsPublic: true}
	repo.shelves[def.ID] = def
	repo.shelves[custom.ID] = custom

	if rec := patchShelf(h, def.ID, userID, `{"is_public":false}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("default shelf: expected 400, got %d", rec.Code)
	}
	if rec := patchShelf(h, custom.ID, userID, `{}`); rec.Code != http.StatusBadRequest {
		t.Fatalf("empty body: expected 400, got %d", rec.Code)
	}
	if rec := patchShelf(h, custom.ID, uuid.New(), `{"is_public":false}`); rec.Code != http.StatusForbidden {
		t.Fatalf("other user: expected 403, got %d", rec.Code)
	}
}

func TestGetPublicShelf_PrivateReturns404EvenForOwner(t *testing.T) {
	h, repo := setupHandler()
	ownerID := uuid.New()
	shelf := &domain.Shelf{ID: uuid.New(), UserID: ownerID, Name: "Тайная", Type: domain.ShelfTypeCustom, IsPublic: false}
	repo.shelves[shelf.ID] = shelf

	c, rec := newContext(http.MethodGet, "/api/v1/shelves/public/"+shelf.ID.String())
	c.Set("user_id", ownerID.String())
	c.SetPath("/api/v1/shelves/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(shelf.ID.String())
	if err := h.GetPublicShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestGetPublicShelf_ReadsCountOfOwnerNotCaller(t *testing.T) {
	h, repo := setupHandler()
	owner, caller := uuid.New(), uuid.New()
	shelf := &domain.Shelf{ID: uuid.New(), UserID: owner, Name: "Прочитано", Type: domain.ShelfTypeRead, IsPublic: true}
	repo.shelves[shelf.ID] = shelf
	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Книга", Slug: "kniga"}
	repo.items[shelfBookKey{shelfID: shelf.ID, bookID: bookID}] = time.Now()
	now := time.Now()
	repo.reads = append(repo.reads,
		domain.BookRead{ID: uuid.New(), UserID: owner, BookID: bookID, FinishedAt: &now, CreatedAt: now},
		domain.BookRead{ID: uuid.New(), UserID: owner, BookID: bookID, FinishedAt: &now, CreatedAt: now},
		domain.BookRead{ID: uuid.New(), UserID: caller, BookID: bookID, FinishedAt: &now, CreatedAt: now},
	)

	c, rec := newContext(http.MethodGet, "/api/v1/shelves/public/"+shelf.ID.String())
	c.Set("user_id", caller.String())
	c.SetPath("/api/v1/shelves/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(shelf.ID.String())
	if err := h.GetPublicShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body struct {
		Items []struct {
			ReadsCount int64      `json:"reads_count"`
			LastReadAt *time.Time `json:"last_read_at"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].ReadsCount != 2 || body.Items[0].LastReadAt == nil {
		t.Fatalf("expected owner's 2 reads, got %+v", body.Items)
	}
}

func TestGetPublicShelf_NeverFinishedHasZeroCountAndNullDate(t *testing.T) {
	h, repo := setupHandler()
	owner := uuid.New()
	shelf := &domain.Shelf{ID: uuid.New(), UserID: owner, Name: "Читаю", Type: domain.ShelfTypeReading, IsPublic: true}
	repo.shelves[shelf.ID] = shelf
	bookID := uuid.New()
	repo.books[bookID] = &catalogDomain.Book{ID: bookID, Title: "Книга", Slug: "kniga"}
	repo.items[shelfBookKey{shelfID: shelf.ID, bookID: bookID}] = time.Now()

	c, rec := newContext(http.MethodGet, "/api/v1/shelves/public/"+shelf.ID.String())
	c.SetPath("/api/v1/shelves/public/:id")
	c.SetParamNames("id")
	c.SetParamValues(shelf.ID.String())
	if err := h.GetPublicShelf(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(rec.Body.String(), `"reads_count":0`) || !strings.Contains(rec.Body.String(), `"last_read_at":null`) {
		t.Fatalf("expected zero counter and null date, got %s", rec.Body.String())
	}
}

func TestListShelves_ReadsSummaryOnlyWithBookID(t *testing.T) {
	h, repo := setupHandler()
	user := uuid.New()
	bookID := uuid.New()
	earlier := time.Date(2025, 5, 1, 0, 0, 0, 0, time.UTC)
	open := time.Now()
	repo.reads = append(repo.reads,
		domain.BookRead{ID: uuid.New(), UserID: user, BookID: bookID, FinishedAt: &earlier, CreatedAt: earlier},
		domain.BookRead{ID: uuid.New(), UserID: user, BookID: bookID, StartedAt: &open, CreatedAt: open},
	)

	c, rec := newContext(http.MethodGet, "/api/v1/shelves?book_id="+bookID.String())
	c.Set("user_id", user.String())
	if err := h.ListShelves(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var body struct {
		Reads *struct {
			ReadsCount int64      `json:"reads_count"`
			LastReadAt *time.Time `json:"last_read_at"`
			Rereading  bool       `json:"rereading"`
		} `json:"reads"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Reads == nil || body.Reads.ReadsCount != 1 || !body.Reads.Rereading || body.Reads.LastReadAt == nil {
		t.Fatalf("unexpected reads %+v", body.Reads)
	}

	c, rec = newContext(http.MethodGet, "/api/v1/shelves")
	c.Set("user_id", user.String())
	if err := h.ListShelves(c); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rec.Body.String(), `"reads"`) {
		t.Fatalf("reads must be omitted without book_id: %s", rec.Body.String())
	}
}
