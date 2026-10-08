package repository

import (
	"context"
	"time"

	"github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Repository defines operations for working with users.
type Repository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
	// UpdateProfile persists only the profile columns (name, avatar_url,
	// profile_type, updated_at) of user. It never writes role, email,
	// password_hash or email_verified_at, so a concurrent admin role change
	// cannot be reverted by a profile save. Returns gorm.ErrRecordNotFound
	// when no row has user.ID.
	UpdateProfile(ctx context.Context, user *domain.User) error
}

// GormRepository is a GORM-based implementation of Repository.
type GormRepository struct {
	db *gorm.DB
}

// NewGormRepository creates a new GormRepository.
func NewGormRepository(db *gorm.DB) Repository {
	return &GormRepository{db: db}
}

func (r *GormRepository) Create(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *GormRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	if err := r.db.WithContext(ctx).First(&user, "email = ?", email).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *GormRepository) UpdateProfile(ctx context.Context, user *domain.User) error {
	now := time.Now()
	res := r.db.WithContext(ctx).Model(&domain.User{}).
		Where("id = ?", user.ID).
		Updates(map[string]any{
			"name":         user.Name,
			"avatar_url":   user.AvatarURL,
			"profile_type": user.ProfileType,
			"updated_at":   now,
		})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	user.UpdatedAt = now
	return nil
}
