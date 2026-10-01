package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/notifications/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeRepository struct {
	notifications map[uuid.UUID]*domain.Notification
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{notifications: make(map[uuid.UUID]*domain.Notification)}
}

func (f *fakeRepository) add(n domain.Notification) {
	copy := n
	f.notifications[n.ID] = &copy
}

func (f *fakeRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]domain.Notification, int64, error) {
	var owned []domain.Notification
	for _, n := range f.notifications {
		if n.UserID == userID {
			owned = append(owned, *n)
		}
	}
	sort.Slice(owned, func(i, j int) bool { return owned[i].CreatedAt.After(owned[j].CreatedAt) })

	total := int64(len(owned))
	if offset >= len(owned) {
		return []domain.Notification{}, total, nil
	}
	end := offset + limit
	if end > len(owned) {
		end = len(owned)
	}
	return owned[offset:end], total, nil
}

func (f *fakeRepository) UnreadCount(ctx context.Context, userID uuid.UUID) (int64, error) {
	var count int64
	for _, n := range f.notifications {
		if n.UserID == userID && !n.IsRead {
			count++
		}
	}
	return count, nil
}

func (f *fakeRepository) MarkRead(ctx context.Context, userID, notificationID uuid.UUID) error {
	n, ok := f.notifications[notificationID]
	if !ok || n.UserID != userID {
		return gorm.ErrRecordNotFound
	}
	n.IsRead = true
	return nil
}

func (f *fakeRepository) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	for _, n := range f.notifications {
		if n.UserID == userID {
			n.IsRead = true
		}
	}
	return nil
}

func setupTestService() (*Service, *fakeRepository) {
	repo := newFakeRepository()
	return NewService(repo), repo
}

func TestList_ScopedToRequestingUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	otherUserID := uuid.New()
	svc, repo := setupTestService()

	repo.add(domain.Notification{ID: uuid.New(), UserID: userID, ActorID: uuid.New(), Type: domain.TypeReviewLike, CreatedAt: time.Now()})
	repo.add(domain.Notification{ID: uuid.New(), UserID: userID, ActorID: uuid.New(), Type: domain.TypeReviewLike, CreatedAt: time.Now()})
	repo.add(domain.Notification{ID: uuid.New(), UserID: otherUserID, ActorID: uuid.New(), Type: domain.TypeReviewLike, CreatedAt: time.Now()})

	result, err := svc.List(ctx, userID, 1, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Total != 2 {
		t.Fatalf("expected 2 notifications scoped to the requesting user, got %d", result.Total)
	}
	for _, n := range result.Items {
		if n.UserID != userID {
			t.Fatalf("expected only the requesting user's notifications, got one for %s", n.UserID)
		}
	}
}

func TestList_DefaultsAndCapsPagination(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()
	for i := 0; i < 5; i++ {
		repo.add(domain.Notification{ID: uuid.New(), UserID: userID, ActorID: uuid.New(), Type: domain.TypeReviewLike, CreatedAt: time.Now()})
	}

	result, err := svc.List(ctx, userID, 0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Page != 1 || result.Limit != 20 {
		t.Fatalf("expected default page=1 limit=20, got page=%d limit=%d", result.Page, result.Limit)
	}

	result, err = svc.List(ctx, userID, 1, 1000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Limit != 100 {
		t.Fatalf("expected limit capped at 100, got %d", result.Limit)
	}
}

func TestUnreadCount_ScopedToRequestingUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	otherUserID := uuid.New()
	svc, repo := setupTestService()

	repo.add(domain.Notification{ID: uuid.New(), UserID: userID, ActorID: uuid.New(), Type: domain.TypeReviewLike, IsRead: false, CreatedAt: time.Now()})
	repo.add(domain.Notification{ID: uuid.New(), UserID: userID, ActorID: uuid.New(), Type: domain.TypeReviewLike, IsRead: true, CreatedAt: time.Now()})
	repo.add(domain.Notification{ID: uuid.New(), UserID: otherUserID, ActorID: uuid.New(), Type: domain.TypeReviewLike, IsRead: false, CreatedAt: time.Now()})

	count, err := svc.UnreadCount(ctx, userID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected unread count 1 for the requesting user, got %d", count)
	}
}

func TestMarkRead_OwnNotification(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	svc, repo := setupTestService()
	id := uuid.New()
	repo.add(domain.Notification{ID: id, UserID: userID, ActorID: uuid.New(), Type: domain.TypeReviewLike, IsRead: false, CreatedAt: time.Now()})

	if err := svc.MarkRead(ctx, userID, id); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !repo.notifications[id].IsRead {
		t.Fatal("expected notification to be marked read")
	}
}

func TestMarkRead_OtherUsersNotificationReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	ownerID := uuid.New()
	otherID := uuid.New()
	svc, repo := setupTestService()
	id := uuid.New()
	repo.add(domain.Notification{ID: id, UserID: ownerID, ActorID: uuid.New(), Type: domain.TypeReviewLike, IsRead: false, CreatedAt: time.Now()})

	err := svc.MarkRead(ctx, otherID, id)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if repo.notifications[id].IsRead {
		t.Fatal("expected the owner's notification to remain unread after a non-owner's mark-read attempt")
	}
}

func TestMarkRead_NonexistentReturnsNotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	err := svc.MarkRead(ctx, uuid.New(), uuid.New())
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestMarkAllRead_ScopedToRequestingUser(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	otherUserID := uuid.New()
	svc, repo := setupTestService()

	idA := uuid.New()
	idB := uuid.New()
	idOther := uuid.New()
	repo.add(domain.Notification{ID: idA, UserID: userID, ActorID: uuid.New(), Type: domain.TypeReviewLike, IsRead: false, CreatedAt: time.Now()})
	repo.add(domain.Notification{ID: idB, UserID: userID, ActorID: uuid.New(), Type: domain.TypeReviewLike, IsRead: false, CreatedAt: time.Now()})
	repo.add(domain.Notification{ID: idOther, UserID: otherUserID, ActorID: uuid.New(), Type: domain.TypeReviewLike, IsRead: false, CreatedAt: time.Now()})

	if err := svc.MarkAllRead(ctx, userID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.notifications[idA].IsRead || !repo.notifications[idB].IsRead {
		t.Fatal("expected all of the requesting user's notifications to be marked read")
	}
	if repo.notifications[idOther].IsRead {
		t.Fatal("expected another user's notification to remain untouched")
	}
}
