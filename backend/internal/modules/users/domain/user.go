package domain

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleUser        Role = "user"
	RoleContributor Role = "contributor"
	RoleModerator   Role = "moderator"
	RoleAdmin       Role = "admin"
)

// IsValidRole reports whether r is one of the user_role enum values.
func IsValidRole(r string) bool {
	switch Role(r) {
	case RoleUser, RoleContributor, RoleModerator, RoleAdmin:
		return true
	}
	return false
}

type ProfileType string

const (
	ProfileTypeReader     ProfileType = "reader"
	ProfileTypeLibrarian  ProfileType = "librarian"
	ProfileTypeBookseller ProfileType = "bookseller"
	ProfileTypeOther      ProfileType = "other"
)

type User struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	Email           string    `gorm:"uniqueIndex;not null"`
	PasswordHash    *string
	Name            *string
	AvatarURL       *string
	Role            Role        `gorm:"type:user_role;not null;default:'user'"`
	ProfileType     ProfileType `gorm:"type:profile_type;not null;default:'reader'"`
	EmailVerifiedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
