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
	// FamilyID groups all tokens descended from one login (the login token
	// has FamilyID == ID). Reuse detection revokes one family only.
	FamilyID uuid.UUID `gorm:"type:uuid;not null;index"`
	// ReplacedBy is the successor's id; set only by rotation, never by
	// logout, password reset or family revocation.
	ReplacedBy *uuid.UUID `gorm:"type:uuid"`
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

// PasswordResetToken is a one-time password reset token stored in the
// database. Only the sha256 hex of the token is persisted; Email is the
// address the link was sent to (a mismatch with users.email invalidates it).
type PasswordResetToken struct {
	ID        uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index"`
	Email     string    `gorm:"not null"`
	TokenHash string    `gorm:"not null;uniqueIndex"`
	ExpiresAt time.Time `gorm:"not null"`
	UsedAt    *time.Time
	CreatedAt time.Time `gorm:"not null"`
}

// TableName returns the table name for PasswordResetToken.
func (PasswordResetToken) TableName() string {
	return "password_reset_tokens"
}

// RefreshTokenRetention is how long a refresh token row is kept after it was
// revoked or expired before the daily purge deletes it (auth/session spec,
// "Refresh token retention"). No code path reads such rows; they are a short
// forensic trail only.
const RefreshTokenRetention = 7 * 24 * time.Hour

// RefreshReuseGrace is how long after a rotation a re-presented old token is
// treated as a benign race (answered "superseded", nothing revoked) rather
// than a reuse that revokes its session family (auth/session spec, "Refresh
// token reuse detection").
const RefreshReuseGrace = 30 * time.Second

// OneTimeTokenRetention is how long an email verification or password reset
// token row is kept after its expiry (used or not) before the daily purge
// deletes it (auth/email-verification and auth/password-reset specs). While
// retained, a used/expired link keeps its specific answer; afterwards it is
// unknown (invalid_token).
const OneTimeTokenRetention = 30 * 24 * time.Hour
