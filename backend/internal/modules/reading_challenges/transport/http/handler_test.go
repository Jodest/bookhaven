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
	"github.com/bookhaven/backend/internal/modules/reading_challenges/domain"
	"github.com/bookhaven/backend/internal/modules/reading_challenges/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

type challengeKey struct {
	userID uuid.UUID
	year   int
}

type readItem struct {
	userID  uuid.UUID
	bookID  uuid.UUID
	addedAt time.Time
}

type fakeRepository struct {
	challenges map[challengeKey]*domain.ReadingChallenge
	readItems  []readItem
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{challenges: make(map[challengeKey]*domain.ReadingChallenge)}
}

func (f *fakeRepository) GetByUserAndYear(ctx context.Context, userID uuid.UUID, year int) (*domain.ReadingChallenge, error) {
	c, ok := f.challenges[challengeKey{userID: userID, year: year}]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *c
	return &copy, nil
}

func (f *fakeRepository) Upsert(ctx context.Context, challenge *domain.ReadingChallenge) error {
	key := challengeKey{userID: challenge.UserID, year: challenge.Year}
	if existing, ok := f.challenges[key]; ok {
		existing.Goal = challenge.Goal
		return nil
	}
	copy := *challenge
	f.challenges[key] = &copy
	return nil
}

func (f *fakeRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]domain.ReadingChallenge, error) {
	var result []domain.ReadingChallenge
	for _, c := range f.challenges {
		if c.UserID == userID {
			result = append(result, *c)
		}
	}
	return result, nil
}

func (f *fakeRepository) addReadItem(userID, bookID uuid.UUID, addedAt time.Time) {
	f.readItems = append(f.readItems, readItem{userID: userID, bookID: bookID, addedAt: addedAt})
}

func (f *fakeRepository) CountReadBooks(ctx context.Context, userID uuid.UUID, year int) (int64, error) {
	start := time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC)

	seen := make(map[uuid.UUID]bool)
	var count int64
	for _, item := range f.readItems {
		if item.userID != userID {
			continue
		}
		if item.addedAt.Before(start) || !item.addedAt.Before(end) {
			continue
		}
		if !seen[item.bookID] {
			seen[item.bookID] = true
			count++
		}
	}
	return count, nil
}

func setupHandler() (*Handler, *fakeRepository) {
	repo := newFakeRepository()
	return NewHandler(service.NewService(repo)), repo
}

func newContext(method, target string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Validator = validator.New()
	req := httptest.NewRequest(method, target, nil)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func newJSONContext(method, target, body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Validator = validator.New()
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestUpsertGoal_RequiresAuth(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newJSONContext(http.MethodPut, "/api/v1/reading-challenges/2024", `{"goal":10}`)
	c.SetPath("/api/v1/reading-challenges/:year")
	c.SetParamNames("year")
	c.SetParamValues("2024")

	if err := h.UpsertGoal(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestGetChallenge_RequiresAuth(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/reading-challenges/2024")
	c.SetPath("/api/v1/reading-challenges/:year")
	c.SetParamNames("year")
	c.SetParamValues("2024")

	if err := h.GetChallenge(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestListChallenges_RequiresAuth(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newContext(http.MethodGet, "/api/v1/reading-challenges")

	if err := h.ListChallenges(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestUpsertGoal_InvalidYear(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newJSONContext(http.MethodPut, "/api/v1/reading-challenges/abc", `{"goal":10}`)
	c.SetPath("/api/v1/reading-challenges/:year")
	c.SetParamNames("year")
	c.SetParamValues("abc")
	c.Set("user_id", uuid.New().String())

	if err := h.UpsertGoal(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpsertGoal_InvalidGoal(t *testing.T) {
	h, _ := setupHandler()
	c, rec := newJSONContext(http.MethodPut, "/api/v1/reading-challenges/2024", `{"goal":0}`)
	c.SetPath("/api/v1/reading-challenges/:year")
	c.SetParamNames("year")
	c.SetParamValues("2024")
	c.Set("user_id", uuid.New().String())

	if err := h.UpsertGoal(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestUpsertGoal_SetsGoalAndReturnsProgress(t *testing.T) {
	h, repo := setupHandler()
	userID := uuid.New()
	repo.addReadItem(userID, uuid.New(), time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC))

	c, rec := newJSONContext(http.MethodPut, "/api/v1/reading-challenges/2024", `{"goal":4}`)
	c.SetPath("/api/v1/reading-challenges/:year")
	c.SetParamNames("year")
	c.SetParamValues("2024")
	c.Set("user_id", userID.String())

	if err := h.UpsertGoal(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body progressResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Goal == nil || *body.Goal != 4 {
		t.Fatalf("expected goal 4, got %v", body.Goal)
	}
	if body.BooksRead != 1 {
		t.Fatalf("expected 1 book read, got %d", body.BooksRead)
	}
}

func TestUsersHaveIndependentChallenges(t *testing.T) {
	h, repo := setupHandler()
	userA := uuid.New()
	userB := uuid.New()

	repo.addReadItem(userA, uuid.New(), time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	repo.addReadItem(userA, uuid.New(), time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC))
	repo.addReadItem(userB, uuid.New(), time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))

	cA, recA := newJSONContext(http.MethodPut, "/api/v1/reading-challenges/2024", `{"goal":10}`)
	cA.SetPath("/api/v1/reading-challenges/:year")
	cA.SetParamNames("year")
	cA.SetParamValues("2024")
	cA.Set("user_id", userA.String())
	if err := h.UpsertGoal(cA); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cB, recB := newJSONContext(http.MethodGet, "/api/v1/reading-challenges/2024", "")
	cB.SetPath("/api/v1/reading-challenges/:year")
	cB.SetParamNames("year")
	cB.SetParamValues("2024")
	cB.Set("user_id", userB.String())
	if err := h.GetChallenge(cB); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var bodyA, bodyB progressResponse
	if err := json.Unmarshal(recA.Body.Bytes(), &bodyA); err != nil {
		t.Fatalf("failed to decode response A: %v", err)
	}
	if err := json.Unmarshal(recB.Body.Bytes(), &bodyB); err != nil {
		t.Fatalf("failed to decode response B: %v", err)
	}

	if bodyA.BooksRead != 2 {
		t.Fatalf("expected user A to have 2 books read, got %d", bodyA.BooksRead)
	}
	if bodyA.Goal == nil || *bodyA.Goal != 10 {
		t.Fatalf("expected user A goal 10, got %v", bodyA.Goal)
	}
	if bodyB.BooksRead != 1 {
		t.Fatalf("expected user B to have 1 book read, got %d", bodyB.BooksRead)
	}
	if bodyB.Goal != nil {
		t.Fatalf("expected user B to have no goal set, got %v", *bodyB.Goal)
	}
}

func TestListChallenges_ReturnsOnlyCallersHistory(t *testing.T) {
	h, _ := setupHandler()
	userA := uuid.New()
	userB := uuid.New()

	cA, _ := newJSONContext(http.MethodPut, "/api/v1/reading-challenges/2024", `{"goal":5}`)
	cA.SetPath("/api/v1/reading-challenges/:year")
	cA.SetParamNames("year")
	cA.SetParamValues("2024")
	cA.Set("user_id", userA.String())
	if err := h.UpsertGoal(cA); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cB, _ := newJSONContext(http.MethodPut, "/api/v1/reading-challenges/2023", `{"goal":8}`)
	cB.SetPath("/api/v1/reading-challenges/:year")
	cB.SetParamNames("year")
	cB.SetParamValues("2023")
	cB.Set("user_id", userB.String())
	if err := h.UpsertGoal(cB); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	c, rec := newContext(http.MethodGet, "/api/v1/reading-challenges")
	c.Set("user_id", userA.String())
	if err := h.ListChallenges(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var body listProgressResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body.Total != 1 {
		t.Fatalf("expected 1 entry for user A, got %d", body.Total)
	}
	if body.Items[0].Year != 2024 {
		t.Fatalf("expected year 2024, got %d", body.Items[0].Year)
	}
}
