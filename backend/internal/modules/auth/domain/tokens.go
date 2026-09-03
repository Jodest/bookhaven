package domain

import "github.com/google/uuid"

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
