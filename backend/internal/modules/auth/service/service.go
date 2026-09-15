package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/modules/users/repository"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user with this email already exists")
	ErrTokenInvalid       = errors.New("invalid or expired token")
	ErrTokenRevoked       = errors.New("token has been revoked")
)

// Config holds auth service configuration.
type Config struct {
	JWTSecret        string
	AccessTokenTTL   time.Duration
	RefreshTokenTTL  time.Duration
	Issuer           string
}

// Service provides authentication logic.
type Service struct {
	config   Config
	db       *gorm.DB
	users    repository.Repository
}

// NewService creates a new auth Service.
func NewService(cfg Config, db *gorm.DB, users repository.Repository) *Service {
	return &Service{
		config:   cfg,
		db:       db,
		users:    users,
	}
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, email, password, name string) (*usersDomain.User, error) {
	// Check if user exists
	existing, err := s.users.GetByEmail(ctx, email)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("check existing user: %w", err)
	}
	if existing != nil {
		return nil, ErrUserExists
	}

	// Hash password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	hashStr := string(hash)
	user := &usersDomain.User{
		ID:           uuid.New(),
		Email:        email,
		PasswordHash: &hashStr,
		Name:         &name,
		Role:         usersDomain.RoleUser,
		ProfileType:  usersDomain.ProfileTypeReader,
	}

	if err := s.users.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	return user, nil
}

// Login authenticates a user and returns token pair.
func (s *Service) Login(ctx context.Context, email, password string) (*usersDomain.User, *domain.TokenPair, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		return nil, nil, fmt.Errorf("get user by email: %w", err)
	}

	if user.PasswordHash == nil {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	tokens, err := s.createTokenPair(ctx, user.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("create token pair: %w", err)
	}

	return user, tokens, nil
}

// CreateTokenPair creates a new access and refresh token pair for a user.
func (s *Service) CreateTokenPair(ctx context.Context, userID uuid.UUID) (*domain.TokenPair, error) {
	return s.createTokenPair(ctx, userID)
}

func (s *Service) createTokenPair(ctx context.Context, userID uuid.UUID) (*domain.TokenPair, error) {
	now := time.Now()

	// Access token
	accessExpiresAt := now.Add(s.config.AccessTokenTTL)
	accessClaims := jwt.MapClaims{
		"sub": userID.String(),
		"iat": now.Unix(),
		"exp": accessExpiresAt.Unix(),
		"iss": s.config.Issuer,
		"typ": "access",
	}
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims)
	accessTokenString, err := accessToken.SignedString([]byte(s.config.JWTSecret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	// Refresh token
	refreshTokenBytes := make([]byte, 32)
	if _, err := rand.Read(refreshTokenBytes); err != nil {
		return nil, fmt.Errorf("generate refresh token: %w", err)
	}
	refreshTokenPlain := hex.EncodeToString(refreshTokenBytes)
	refreshTokenHash := hashToken(refreshTokenPlain)

	refreshExpiresAt := now.Add(s.config.RefreshTokenTTL).Unix()
	refresh := &domain.RefreshToken{
		UserID:    userID,
		TokenHash: refreshTokenHash,
		ExpiresAt: refreshExpiresAt,
		CreatedAt: now.Unix(),
	}

	if err := s.db.WithContext(ctx).Create(refresh).Error; err != nil {
		return nil, fmt.Errorf("save refresh token: %w", err)
	}

	return &domain.TokenPair{
		AccessToken:  accessTokenString,
		RefreshToken: refreshTokenPlain,
		ExpiresIn:    int64(s.config.AccessTokenTTL.Seconds()),
	}, nil
}

// ValidateAccessToken validates an access token and returns user ID.
func (s *Service) ValidateAccessToken(tokenString string) (uuid.UUID, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.JWTSecret), nil
	}, jwt.WithIssuer(s.config.Issuer))
	if err != nil {
		return uuid.Nil, ErrTokenInvalid
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return uuid.Nil, ErrTokenInvalid
	}

	if typ, ok := claims["typ"].(string); !ok || typ != "access" {
		return uuid.Nil, ErrTokenInvalid
	}

	sub, ok := claims["sub"].(string)
	if !ok {
		return uuid.Nil, ErrTokenInvalid
	}

	userID, err := uuid.Parse(sub)
	if err != nil {
		return uuid.Nil, ErrTokenInvalid
	}

	return userID, nil
}

// RefreshTokens validates a refresh token and issues a new token pair.
func (s *Service) RefreshTokens(ctx context.Context, refreshTokenPlain string) (*domain.TokenPair, error) {
	refreshTokenHash := hashToken(refreshTokenPlain)

	var token domain.RefreshToken
	if err := s.db.WithContext(ctx).
		Where("token_hash = ? AND revoked_at IS NULL AND expires_at > ?", refreshTokenHash, time.Now().Unix()).
		First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenInvalid
		}
		return nil, fmt.Errorf("find refresh token: %w", err)
	}

	// Revoke old token
	now := time.Now().Unix()
	if err := s.db.WithContext(ctx).Model(&token).Update("revoked_at", now).Error; err != nil {
		return nil, fmt.Errorf("revoke old token: %w", err)
	}

	return s.createTokenPair(ctx, token.UserID)
}

// Logout revokes all refresh tokens for a user.
func (s *Service) Logout(ctx context.Context, userID uuid.UUID) error {
	now := time.Now().Unix()
	return s.db.WithContext(ctx).
		Model(&domain.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now).Error
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ConstantTimeCompare performs constant time comparison of two hashes.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
