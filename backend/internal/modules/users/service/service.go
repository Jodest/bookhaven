package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/modules/users/repository"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidName        = errors.New("name must be between 2 and 255 characters")
	ErrInvalidAvatarURL   = errors.New("avatar_url must be a valid http or https URL")
	ErrInvalidProfileType = errors.New("profile_type must be one of reader, librarian, bookseller, or other")
)

var validProfileTypes = map[domain.ProfileType]struct{}{
	domain.ProfileTypeReader:     {},
	domain.ProfileTypeLibrarian:  {},
	domain.ProfileTypeBookseller: {},
	domain.ProfileTypeOther:      {},
}

// Service provides user profile logic.
type Service struct {
	repo repository.Repository
}

// NewService creates a new users Service.
func NewService(repo repository.Repository) *Service {
	return &Service{repo: repo}
}

// GetByID returns a user by ID.
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

// UpdateMe updates the authenticated user's profile fields.
func (s *Service) UpdateMe(ctx context.Context, userID uuid.UUID, name, avatarURL, profileType *string) (*domain.User, error) {
	user, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user: %w", err)
	}

	if name != nil {
		n := strings.TrimSpace(*name)
		if l := utf8.RuneCountInString(n); l < 2 || l > 255 {
			return nil, ErrInvalidName
		}
		user.Name = &n
	}

	if avatarURL != nil {
		a := strings.TrimSpace(*avatarURL)
		if a == "" {
			user.AvatarURL = nil
		} else {
			u, err := url.Parse(a)
			if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return nil, ErrInvalidAvatarURL
			}
			user.AvatarURL = &a
		}
	}

	if profileType != nil {
		pt := domain.ProfileType(*profileType)
		if _, ok := validProfileTypes[pt]; !ok {
			return nil, ErrInvalidProfileType
		}
		user.ProfileType = pt
	}

	if err := s.repo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	return user, nil
}
