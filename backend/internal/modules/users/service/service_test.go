package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type fakeRepository struct {
	users map[uuid.UUID]*domain.User
}

func newFakeRepository() *fakeRepository {
	return &fakeRepository{
		users: make(map[uuid.UUID]*domain.User),
	}
}

func (f *fakeRepository) Create(ctx context.Context, user *domain.User) error {
	copy := *user
	f.users[user.ID] = &copy
	return nil
}

func (f *fakeRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *u
	return &copy, nil
}

func (f *fakeRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			copy := *u
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeRepository) Update(ctx context.Context, user *domain.User) error {
	if _, ok := f.users[user.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	copy := *user
	copy.UpdatedAt = time.Now()
	f.users[user.ID] = &copy
	return nil
}

func setupTestService() (*Service, *fakeRepository) {
	repo := newFakeRepository()
	return NewService(repo), repo
}

func TestGetByID(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	result, err := svc.GetByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != user.ID {
		t.Fatal("expected user to be returned")
	}
}

func TestGetByID_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	_, err := svc.GetByID(ctx, uuid.New())
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUpdateMe(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	newName := "Alice Smith"
	avatar := "https://example.com/avatar.jpg"
	profileType := "librarian"

	updated, err := svc.UpdateMe(ctx, user.ID, &newName, &avatar, &profileType)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *updated.Name != "Alice Smith" {
		t.Fatalf("expected name Alice Smith, got %v", *updated.Name)
	}
	if updated.AvatarURL == nil || *updated.AvatarURL != avatar {
		t.Fatal("expected avatar URL to be updated")
	}
	if updated.ProfileType != domain.ProfileTypeLibrarian {
		t.Fatalf("expected profile type librarian, got %s", updated.ProfileType)
	}
}

func TestUpdateMe_ClearAvatar(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	avatar := "https://example.com/avatar.jpg"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, AvatarURL: &avatar, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	emptyAvatar := ""
	updated, err := svc.UpdateMe(ctx, user.ID, nil, &emptyAvatar, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.AvatarURL != nil {
		t.Fatalf("expected avatar to be cleared, got %v", *updated.AvatarURL)
	}
}

func TestUpdateMe_InvalidName(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	cases := []string{"", "a", "   "}
	for _, n := range cases {
		_, err := svc.UpdateMe(ctx, user.ID, &n, nil, nil)
		if !errors.Is(err, ErrInvalidName) {
			t.Fatalf("expected ErrInvalidName for %q, got %v", n, err)
		}
	}
}

func TestUpdateMe_NameTooLong(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	longName := strings.Repeat("a", 256)
	_, err := svc.UpdateMe(ctx, user.ID, &longName, nil, nil)
	if !errors.Is(err, ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}

func TestUpdateMe_NameMaxLengthValid(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	maxName := strings.Repeat("é", 255)
	updated, err := svc.UpdateMe(ctx, user.ID, &maxName, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if *updated.Name != maxName {
		t.Fatalf("expected name to be updated to max-length value")
	}
}

func TestUpdateMe_NameTooLongUnicode(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	longName := strings.Repeat("é", 256)
	_, err := svc.UpdateMe(ctx, user.ID, &longName, nil, nil)
	if !errors.Is(err, ErrInvalidName) {
		t.Fatalf("expected ErrInvalidName, got %v", err)
	}
}

func TestUpdateMe_InvalidAvatarURL(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	cases := []string{"ftp://example.com/avatar.jpg", "not-a-url", "http://"}
	for _, url := range cases {
		_, err := svc.UpdateMe(ctx, user.ID, nil, &url, nil)
		if !errors.Is(err, ErrInvalidAvatarURL) {
			t.Fatalf("expected ErrInvalidAvatarURL for %q, got %v", url, err)
		}
	}
}

func TestUpdateMe_InvalidProfileType(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	invalid := "bot"
	_, err := svc.UpdateMe(ctx, user.ID, nil, nil, &invalid)
	if !errors.Is(err, ErrInvalidProfileType) {
		t.Fatalf("expected ErrInvalidProfileType, got %v", err)
	}
}

func TestUpdateMe_ValidProfileTypes(t *testing.T) {
	ctx := context.Background()
	svc, repo := setupTestService()

	name := "Alice"
	user := &domain.User{ID: uuid.New(), Email: "alice@example.com", Name: &name, ProfileType: domain.ProfileTypeReader}
	repo.users[user.ID] = user

	cases := []string{"reader", "librarian", "bookseller", "other"}
	for _, pt := range cases {
		updated, err := svc.UpdateMe(ctx, user.ID, nil, nil, &pt)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", pt, err)
		}
		if string(updated.ProfileType) != pt {
			t.Fatalf("expected profile type %s, got %s", pt, updated.ProfileType)
		}
	}
}

func TestUpdateMe_NotFound(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestService()

	name := "Alice"
	_, err := svc.UpdateMe(ctx, uuid.New(), &name, nil, nil)
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("expected ErrUserNotFound, got %v", err)
	}
}
