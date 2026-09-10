package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/reading_challenges/domain"
	"github.com/google/uuid"
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
		existing.UpdatedAt = time.Now()
		return nil
	}
	copy := *challenge
	copy.CreatedAt = time.Now()
	copy.UpdatedAt = time.Now()
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
	sort.Slice(result, func(i, j int) bool { return result[i].Year > result[j].Year })
	return result, nil
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

func (f *fakeRepository) addReadItem(userID, bookID uuid.UUID, addedAt time.Time) {
	f.readItems = append(f.readItems, readItem{userID: userID, bookID: bookID, addedAt: addedAt})
}

func (f *fakeRepository) removeReadItem(userID, bookID uuid.UUID) {
	filtered := make([]readItem, 0, len(f.readItems))
	for _, item := range f.readItems {
		if item.userID == userID && item.bookID == bookID {
			continue
		}
		filtered = append(filtered, item)
	}
	f.readItems = filtered
}

func setupTestService() (*Service, *fakeRepository) {
	repo := newFakeRepository()
	return NewService(repo), repo
}

func TestUpsertGoal_CreatesThenUpdates(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	progress, err := svc.UpsertGoal(ctx, userID, 2024, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.Goal == nil || *progress.Goal != 20 {
		t.Fatalf("expected goal 20, got %v", progress.Goal)
	}
	if len(repo.challenges) != 1 {
		t.Fatalf("expected 1 stored challenge, got %d", len(repo.challenges))
	}

	progress, err = svc.UpsertGoal(ctx, userID, 2024, 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.Goal == nil || *progress.Goal != 30 {
		t.Fatalf("expected updated goal 30, got %v", progress.Goal)
	}
	if len(repo.challenges) != 1 {
		t.Fatalf("expected no duplicate row after update, got %d rows", len(repo.challenges))
	}
}

func TestUpsertGoal_GoalValidationBoundaries(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	invalidGoals := []int{0, -1, 10001}
	for _, goal := range invalidGoals {
		_, err := svc.UpsertGoal(ctx, userID, 2024, goal)
		if !errors.Is(err, ErrInvalidGoal) {
			t.Fatalf("expected ErrInvalidGoal for goal %d, got %v", goal, err)
		}
	}

	validGoals := []int{1, 10000}
	for _, goal := range validGoals {
		_, err := svc.UpsertGoal(ctx, userID, 2024, goal)
		if err != nil {
			t.Fatalf("unexpected error for goal %d: %v", goal, err)
		}
	}
}

func TestUpsertGoal_YearValidationBoundaries(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	currentYear := time.Now().UTC().Year()

	invalidYears := []int{1999, currentYear + 2}
	for _, year := range invalidYears {
		_, err := svc.UpsertGoal(ctx, userID, year, 10)
		if !errors.Is(err, ErrInvalidYear) {
			t.Fatalf("expected ErrInvalidYear for year %d, got %v", year, err)
		}
	}

	validYears := []int{2000, currentYear, currentYear + 1}
	for _, year := range validYears {
		_, err := svc.UpsertGoal(ctx, userID, year, 10)
		if err != nil {
			t.Fatalf("unexpected error for year %d: %v", year, err)
		}
	}
}

func TestGetProgress_ComputesFromReadShelfWithinYear(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	if _, err := svc.UpsertGoal(ctx, userID, 2024, 5); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	repo.addReadItem(userID, uuid.New(), time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))
	repo.addReadItem(userID, uuid.New(), time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC))
	// Added in a different year: must not count toward 2024.
	repo.addReadItem(userID, uuid.New(), time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC))
	repo.addReadItem(userID, uuid.New(), time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC))

	progress, err := svc.GetProgress(ctx, userID, 2024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.BooksRead != 2 {
		t.Fatalf("expected 2 books read in 2024, got %d", progress.BooksRead)
	}
	if progress.ProgressPercent == nil || *progress.ProgressPercent != 40 {
		t.Fatalf("expected 40%% progress, got %v", progress.ProgressPercent)
	}
	if progress.Completed {
		t.Fatal("expected not completed")
	}
}

func TestGetProgress_RemovedBookNoLongerCounts(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	bookID := uuid.New()
	repo.addReadItem(userID, bookID, time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))

	progress, err := svc.GetProgress(ctx, userID, 2024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.BooksRead != 1 {
		t.Fatalf("expected 1 book read, got %d", progress.BooksRead)
	}

	repo.removeReadItem(userID, bookID)

	progress, err = svc.GetProgress(ctx, userID, 2024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.BooksRead != 0 {
		t.Fatalf("expected 0 books read after removal, got %d", progress.BooksRead)
	}
}

func TestGetProgress_NoGoalSet(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	repo.addReadItem(userID, uuid.New(), time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC))

	progress, err := svc.GetProgress(ctx, userID, 2024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if progress.Goal != nil {
		t.Fatalf("expected nil goal, got %v", *progress.Goal)
	}
	if progress.BooksRead != 1 {
		t.Fatalf("expected 1 book read, got %d", progress.BooksRead)
	}
	if progress.ProgressPercent != nil {
		t.Fatalf("expected nil progress percent when no goal set, got %v", *progress.ProgressPercent)
	}
	if progress.Completed {
		t.Fatal("expected not completed when no goal set")
	}
}

func TestGetProgress_Completed(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()

	if _, err := svc.UpsertGoal(ctx, userID, 2024, 2); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	repo.addReadItem(userID, uuid.New(), time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC))
	repo.addReadItem(userID, uuid.New(), time.Date(2024, 2, 1, 0, 0, 0, 0, time.UTC))
	repo.addReadItem(userID, uuid.New(), time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC))

	progress, err := svc.GetProgress(ctx, userID, 2024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !progress.Completed {
		t.Fatal("expected completed when books read exceeds goal")
	}
	if progress.ProgressPercent == nil || *progress.ProgressPercent != 100 {
		t.Fatalf("expected progress capped at 100%%, got %v", progress.ProgressPercent)
	}
}

func TestListHistory_NewestFirst(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, _ := setupTestService()

	if _, err := svc.UpsertGoal(ctx, userID, 2022, 10); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.UpsertGoal(ctx, userID, 2024, 20); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.UpsertGoal(ctx, userID, 2023, 15); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	history, err := svc.ListHistory(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(history))
	}
	if history[0].Year != 2024 || history[1].Year != 2023 || history[2].Year != 2022 {
		t.Fatalf("expected newest-first ordering, got %d, %d, %d", history[0].Year, history[1].Year, history[2].Year)
	}
}
