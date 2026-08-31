package service

import (
	"context"
	"errors"
	"testing"

	ratingsDomain "github.com/bookhaven/backend/internal/modules/ratings/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type bookUserKey struct {
	bookID uuid.UUID
	userID uuid.UUID
}

type fakeRepository struct {
	ratings map[bookUserKey]*ratingsDomain.Rating
	books   map[uuid.UUID]bool
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		ratings: make(map[bookUserKey]*ratingsDomain.Rating),
		books:   make(map[uuid.UUID]bool),
	}
}

func (f *fakeRepository) UpsertAndRecalc(ctx context.Context, bookID, userID uuid.UUID, value int) (bool, float64, int, error) {
	key := bookUserKey{bookID: bookID, userID: userID}
	created := false
	if r, ok := f.ratings[key]; ok {
		r.Value = value
	} else {
		f.ratings[key] = &ratingsDomain.Rating{
			ID:     uuid.New(),
			BookID: bookID,
			UserID: userID,
			Value:  value,
		}
		created = true
	}

	var sum int64
	count := 0
	for _, r := range f.ratings {
		if r.BookID == bookID {
			sum += int64(r.Value)
			count++
		}
	}

	rating := 0.0
	if count > 0 {
		rating = float64(sum) / float64(count)
	}
	return created, rating, count, nil
}

func (f *fakeRepository) GetByUserAndBook(ctx context.Context, userID, bookID uuid.UUID) (*ratingsDomain.Rating, error) {
	key := bookUserKey{bookID: bookID, userID: userID}
	if r, ok := f.ratings[key]; ok {
		copy := *r
		return &copy, nil
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) BookExists(ctx context.Context, bookID uuid.UUID) (bool, error) {
	return f.books[bookID], nil
}

func setupTestService() (*Service, *fakeRepository) {
	repo := newFakeRepository()
	return NewService(repo), repo
}

func TestSetRating_Create(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = true

	result, err := svc.SetRating(ctx, userID, bookID, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Created {
		t.Fatal("expected created to be true")
	}
	if result.Value != 4 {
		t.Fatalf("expected value 4, got %d", result.Value)
	}
	if result.Rating != 4.0 {
		t.Fatalf("expected rating 4.0, got %f", result.Rating)
	}
	if result.RatingsCount != 1 {
		t.Fatalf("expected ratings count 1, got %d", result.RatingsCount)
	}
}

func TestSetRating_Update(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = true

	if _, err := svc.SetRating(ctx, userID, bookID, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	result, err := svc.SetRating(ctx, userID, bookID, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Created {
		t.Fatal("expected created to be false on update")
	}
	if result.RatingsCount != 1 {
		t.Fatalf("expected ratings count to stay 1, got %d", result.RatingsCount)
	}
	if result.Rating != 2.0 {
		t.Fatalf("expected rating 2.0, got %f", result.Rating)
	}
}

func TestSetRating_InvalidValue(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()
	bookID := uuid.New()
	repo.books[bookID] = true

	cases := []int{0, 6, -1}
	for _, v := range cases {
		_, err := svc.SetRating(ctx, uuid.New(), bookID, v)
		if !errors.Is(err, ErrInvalidValue) {
			t.Fatalf("expected ErrInvalidValue for %d, got %v", v, err)
		}
	}
}

func TestSetRating_BookNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	_, err := svc.SetRating(ctx, uuid.New(), uuid.New(), 3)
	if !errors.Is(err, ErrBookNotFound) {
		t.Fatalf("expected ErrBookNotFound, got %v", err)
	}
}

func TestGetMyRating_Found(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	bookID := uuid.New()
	svc, repo := setupTestService()
	repo.books[bookID] = true

	if _, err := svc.SetRating(ctx, userID, bookID, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	value, err := svc.GetMyRating(ctx, userID, bookID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != 5 {
		t.Fatalf("expected value 5, got %d", value)
	}
}

func TestGetMyRating_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	_, err := svc.GetMyRating(ctx, uuid.New(), uuid.New())
	if !errors.Is(err, ErrRatingNotFound) {
		t.Fatalf("expected ErrRatingNotFound, got %v", err)
	}
}
