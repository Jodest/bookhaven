package domain

import (
	"time"

	"github.com/google/uuid"
)

// TokenPair represents access and refresh tokens.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

// RefreshToken represents a refresh token stored in the database.
type RefreshToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	TokenHash string    `gorm:"not null"`
	ExpiresAt int64     `gorm:"not null"`
	CreatedAt int64     `gorm:"not null"`
	RevokedAt *int64
}

// TableName returns the table name for RefreshToken.
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}

// EmailVerificationToken is a one-time email verification token stored in
// the database. Only the sha256 hex of the token is persisted; Email is the
// address the link was sent to (a mismatch with users.email invalidates it).
type EmailVerificationToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Email     string    `gorm:"not null"`
	TokenHash string    `gorm:"not null;uniqueIndex"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	CreatedAt time.Time `gorm:"not null"`
}

// TableName returns the table name for EmailVerificationToken.
func (EmailVerificationToken) TableName() string {
	return "email_verification_tokens"
}
