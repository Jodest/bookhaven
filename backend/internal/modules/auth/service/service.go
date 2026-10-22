package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/modules/users/repository"
	"github.com/bookhaven/backend/internal/pkg/mailer"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// MaxPasswordBytes is bcrypt's input limit; longer passwords cannot be hashed.
// It counts UTF-8 bytes (len), not runes.
const MaxPasswordBytes = 72

var (
	// ErrPasswordTooLong is returned by Register and ResetPassword for a
	// password longer than MaxPasswordBytes.
	ErrPasswordTooLong = errors.New("password is too long")

	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user with this email already exists")
	ErrTokenInvalid       = errors.New("invalid or expired token")
	ErrTokenRevoked       = errors.New("token has been revoked")

	// Email verification errors.
	ErrVerificationTokenInvalid = errors.New("invalid verification token")
	ErrVerificationTokenExpired = errors.New("verification token expired")
	ErrVerificationTokenUsed    = errors.New("verification token already used")
	ErrRateLimited              = errors.New("too many requests")

	// Password reset errors.
	ErrResetTokenInvalid = errors.New("invalid password reset token")
	ErrResetTokenExpired = errors.New("password reset token expired")
	ErrResetTokenUsed    = errors.New("password reset token already used")

	// ErrEmailNotVerified is returned by Login (after a successful password
	// check) and RefreshTokens when the user's email is not verified.
	ErrEmailNotVerified = errors.New("email not verified")
)

// RateLimitError is returned when an action is rate limited; it matches
// ErrRateLimited via errors.Is and carries the time until a retry is allowed.
type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("too many requests, retry after %s", e.RetryAfter)
}

// Is makes errors.Is(err, ErrRateLimited) true for *RateLimitError.
func (e *RateLimitError) Is(target error) bool {
	return target == ErrRateLimited
}

// Mailer enqueues a rendered email (shape of mailer.Enqueuer).
type Mailer interface {
	Enqueue(ctx context.Context, msg mailer.Message) error
}

// ResendLimiter rate-limits public email-sending requests per key.
type ResendLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
}

// ResendLimiters are the two limiters of a public email-sending endpoint
// (resend-verification, forgot-password): IP is keyed by the client IP,
// Email by the sha256 of the normalised address.
type ResendLimiters struct {
	IP    ResendLimiter
	Email ResendLimiter
}

// LoginLimiter throttles failed login attempts per key. Allow reserves a slot,
// Release gives one back, Reset clears the key.
type LoginLimiter interface {
	Allow(ctx context.Context, key string) (allowed bool, retryAfter time.Duration, err error)
	Release(ctx context.Context, key string) error
	Reset(ctx context.Context, key string) error
}

// LoginLimiters are the two login limiters: IP is keyed by the client IP,
// Email by the sha256 of the normalised address. A nil limiter means "no
// limit".
type LoginLimiters struct {
	IP    LoginLimiter
	Email LoginLimiter
}

// defaultVerificationTokenTTL is used when Config.VerificationTokenTTL is 0.
const defaultVerificationTokenTTL = 24 * time.Hour

// verifyEmailTemplate is the mailer template name for the verification email.
const verifyEmailTemplate = "verify_email"

// resetPasswordTemplate is the mailer template name for the password reset
// email.
const resetPasswordTemplate = "reset_password"

// passwordResetTokenTTL is the lifetime of a password reset token. Not
// configurable: the email text says "1 час".
const passwordResetTokenTTL = time.Hour

// Config holds auth service configuration.
type Config struct {
	JWTSecret       string
	AccessTokenTTL  time.Duration
	RefreshTokenTTL time.Duration
	Issuer          string

	// WebBaseURL is the public web origin (no trailing slash) used to build
	// the /verify-email link.
	WebBaseURL string
	// VerificationTokenTTL is the lifetime of an email verification token
	// (24h when zero).
	VerificationTokenTTL time.Duration
}

// Service provides authentication logic.
type Service struct {
	config   Config
	db       *gorm.DB
	users    repository.Repository
	mailer   Mailer
	limiters ResendLimiters
	// resetLimiters limit the public forgot-password endpoint.
	resetLimiters ResendLimiters
	// loginLimiters throttle failed logins (fail-open on limiter errors).
	loginLimiters LoginLimiters
	logger        *zap.Logger
}

// NewService creates a new auth Service. limiters limit resend-verification,
// resetLimiters limit forgot-password, loginLimiters throttle failed logins
// (empty = no limits). logger may be nil (no logging).
func NewService(cfg Config, db *gorm.DB, users repository.Repository, mailer Mailer, limiters ResendLimiters, resetLimiters ResendLimiters, loginLimiters LoginLimiters, logger *zap.Logger) *Service {
	if cfg.VerificationTokenTTL <= 0 {
		cfg.VerificationTokenTTL = defaultVerificationTokenTTL
	}
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Service{
		config:        cfg,
		db:            db,
		users:         users,
		mailer:        mailer,
		limiters:      limiters,
		resetLimiters: resetLimiters,
		loginLimiters: loginLimiters,
		logger:        logger,
	}
}

// Register creates a new user account.
func (s *Service) Register(ctx context.Context, email, password, name string) (*usersDomain.User, error) {
	if len(password) > MaxPasswordBytes {
		return nil, ErrPasswordTooLong
	}

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

	// The verification email must never fail registration: a user without
	// a (delivered) token can always use resend-verification. Never log the
	// token or the link.
	if err := s.issueVerification(ctx, user); err != nil {
		s.logger.Error("failed to issue email verification on register",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
	}

	return user, nil
}

// Login authenticates a user and returns token pair. The password is
// checked before the verification status, so only someone who knows the
// password learns that the account is unverified (ErrEmailNotVerified, no
// tokens created).
//
// Attempts are throttled per client ip (every attempt counts) and per
// address (failures): each attempt
// reserves a slot in both limiters before the user lookup (so the answer is
// identical for existing and unknown accounts), a failure keeps the
// reservations, a correct password keeps the ip slot (so own-account
// logins cannot keep the ip counter flat) and resets the address counter, an internal error releases both. Limiter errors fail open
// (logged). Errors: *RateLimitError (ErrRateLimited), ErrInvalidCredentials,
// ErrEmailNotVerified.
func (s *Service) Login(ctx context.Context, email, password, ip string) (*usersDomain.User, *domain.TokenPair, error) {
	emailKey := emailRateLimitKey(email)
	ipReserved, emailReserved := false, false

	if lim := s.loginLimiters.IP; lim != nil {
		allowed, retryAfter, err := lim.Allow(ctx, ip)
		switch {
		case err != nil:
			s.logger.Warn("login: ip rate limiter unavailable, failing open", zap.Error(err))
		case !allowed:
			return nil, nil, &RateLimitError{RetryAfter: retryAfter}
		default:
			ipReserved = true
		}
	}
	if lim := s.loginLimiters.Email; lim != nil {
		allowed, retryAfter, err := lim.Allow(ctx, emailKey)
		switch {
		case err != nil:
			s.logger.Warn("login: email rate limiter unavailable, failing open", zap.Error(err))
		case !allowed:
			// The ip slot stays spent: hammering a blocked address costs quota.
			return nil, nil, &RateLimitError{RetryAfter: retryAfter}
		default:
			emailReserved = true
		}
	}

	// settleSuccess: the password was correct. The ip slot is deliberately
	// kept: the per-ip counter counts every attempt.
	settleSuccess := func() {
		if s.loginLimiters.Email != nil {
			if err := s.loginLimiters.Email.Reset(ctx, emailKey); err != nil {
				s.logger.Warn("login: failed to reset email counter", zap.Error(err))
			}
		}
	}
	// settleInternal: an internal error is not a guess; give both slots back.
	settleInternal := func() {
		if ipReserved {
			if err := s.loginLimiters.IP.Release(ctx, ip); err != nil {
				s.logger.Warn("login: failed to release ip slot", zap.Error(err))
			}
		}
		if emailReserved {
			if err := s.loginLimiters.Email.Release(ctx, emailKey); err != nil {
				s.logger.Warn("login: failed to release email slot", zap.Error(err))
			}
		}
	}

	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, ErrInvalidCredentials
		}
		settleInternal()
		return nil, nil, fmt.Errorf("get user by email: %w", err)
	}

	if user.PasswordHash == nil {
		return nil, nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*user.PasswordHash), []byte(password)); err != nil {
		return nil, nil, ErrInvalidCredentials
	}

	if user.EmailVerifiedAt == nil {
		settleSuccess()
		return user, nil, ErrEmailNotVerified
	}

	tokens, err := s.createTokenPair(ctx, user.ID)
	if err != nil {
		settleInternal()
		return nil, nil, fmt.Errorf("create token pair: %w", err)
	}

	settleSuccess()
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

// RefreshTokens validates a refresh token and issues a new token pair. If
// the token's user is not verified, all of the user's refresh tokens are
// revoked and ErrEmailNotVerified is returned.
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

	user, err := s.users.GetByID(ctx, token.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTokenInvalid
		}
		return nil, fmt.Errorf("get user: %w", err)
	}
	if user.EmailVerifiedAt == nil {
		if err := s.Logout(ctx, user.ID); err != nil {
			return nil, fmt.Errorf("revoke refresh tokens: %w", err)
		}
		return nil, ErrEmailNotVerified
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
	return revokeRefreshTokens(s.db.WithContext(ctx), userID, time.Now())
}

// revokeRefreshTokens revokes all active refresh tokens of userID using db
// (a plain handle or a transaction).
func revokeRefreshTokens(db *gorm.DB, userID uuid.UUID, now time.Time) error {
	return db.Model(&domain.RefreshToken{}).
		Where("user_id = ? AND revoked_at IS NULL", userID).
		Update("revoked_at", now.Unix()).Error
}

// VerifyEmail consumes an email verification token. It returns
// alreadyVerified=true when the token's user had already been verified
// (also for used/expired tokens). Errors: ErrVerificationTokenInvalid,
// ErrVerificationTokenExpired, ErrVerificationTokenUsed.
func (s *Service) VerifyEmail(ctx context.Context, plain string) (alreadyVerified bool, err error) {
	if !isTokenFormat(plain) {
		return false, ErrVerificationTokenInvalid
	}

	var token domain.EmailVerificationToken
	if err := s.db.WithContext(ctx).Where("token_hash = ?", hashToken(plain)).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrVerificationTokenInvalid
		}
		return false, fmt.Errorf("find verification token: %w", err)
	}

	user, err := s.users.GetByID(ctx, token.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, ErrVerificationTokenInvalid
		}
		return false, fmt.Errorf("get user: %w", err)
	}
	if user.Email != token.Email {
		return false, ErrVerificationTokenInvalid
	}

	now := time.Now()
	if user.EmailVerifiedAt != nil {
		if token.UsedAt == nil {
			if _, err := markVerificationTokenUsed(s.db.WithContext(ctx), token.ID, now); err != nil {
				return false, err
			}
		}
		return true, nil
	}
	if token.UsedAt != nil {
		return false, ErrVerificationTokenUsed
	}
	if !now.Before(token.ExpiresAt) {
		return false, ErrVerificationTokenExpired
	}

	errLostRace := errors.New("verification token consumed concurrently")
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		marked, err := markVerificationTokenUsed(tx, token.ID, now)
		if err != nil {
			return err
		}
		if !marked {
			return errLostRace
		}
		if err := tx.Exec(
			"UPDATE users SET email_verified_at = ?, updated_at = ? WHERE id = ? AND email_verified_at IS NULL",
			now, now, user.ID,
		).Error; err != nil {
			return fmt.Errorf("mark user verified: %w", err)
		}
		return nil
	})
	if errors.Is(err, errLostRace) {
		// A concurrent request used this token first; re-evaluate.
		current, getErr := s.users.GetByID(ctx, user.ID)
		if getErr != nil {
			return false, fmt.Errorf("get user: %w", getErr)
		}
		if current.EmailVerifiedAt != nil {
			return true, nil
		}
		return false, ErrVerificationTokenUsed
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

// ResendVerification emails a new verification link to email if it belongs
// to an unverified account with a password, invalidating the account's
// unused tokens. It is enumeration-safe: rate limits (per ip, then per
// normalised email) are applied before the user lookup, and every outcome
// after the lookup - unknown email, already verified, no password, or a
// failure to issue the email (logged) - returns nil. Errors: *RateLimitError
// (ErrRateLimited), or a limiter error.
func (s *Service) ResendVerification(ctx context.Context, email, ip string) error {
	if err := checkEmailRequestLimits(ctx, s.limiters, email, ip); err != nil {
		return err
	}

	// The lookup uses the email as given, same as Login/Register.
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("resend verification: failed to look up user", zap.Error(err))
		}
		return nil
	}
	if user.EmailVerifiedAt != nil || user.PasswordHash == nil {
		return nil
	}

	// Old links stop working; used rows are kept so a repeated click on an
	// already-used link still answers already_verified.
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND used_at IS NULL", user.ID).
		Delete(&domain.EmailVerificationToken{}).Error; err != nil {
		s.logger.Error("resend verification: failed to delete unused tokens",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
		return nil
	}

	if err := s.issueVerification(ctx, user); err != nil {
		s.logger.Error("resend verification: failed to issue email verification",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
	}
	return nil
}

// emailRateLimitKey is the per-email rate-limit key: the hex sha256 of the
// trimmed, lowercased address, so no plain addresses are stored in Redis
// and differently-cased spellings share one quota.
func emailRateLimitKey(email string) string {
	return hashToken(strings.ToLower(strings.TrimSpace(email)))
}

// checkEmailRequestLimits applies the per-IP and then the per-email limit of
// a public email-sending endpoint. They depend only on request data, so a
// rate-limit answer never reveals whether the account exists. An IP-limited
// request does not consume the email quota.
func checkEmailRequestLimits(ctx context.Context, limiters ResendLimiters, email, ip string) error {
	if limiters.IP == nil || limiters.Email == nil {
		return errors.New("rate limiters not configured")
	}

	allowed, retryAfter, err := limiters.IP.Allow(ctx, ip)
	if err != nil {
		return fmt.Errorf("ip rate limiter: %w", err)
	}
	if !allowed {
		return &RateLimitError{RetryAfter: retryAfter}
	}

	allowed, retryAfter, err = limiters.Email.Allow(ctx, emailRateLimitKey(email))
	if err != nil {
		return fmt.Errorf("email rate limiter: %w", err)
	}
	if !allowed {
		return &RateLimitError{RetryAfter: retryAfter}
	}
	return nil
}

// ForgotPassword emails a password reset link to email if it belongs to an
// account with a password (verified or not), invalidating the account's
// unused reset tokens. It is enumeration-safe: rate limits (per ip, then per
// normalised email) are applied before the user lookup, and every outcome
// after the lookup - unknown email, no password, or a failure to issue the
// email (logged) - returns nil. Errors: *RateLimitError (ErrRateLimited), or
// a limiter error.
func (s *Service) ForgotPassword(ctx context.Context, email, ip string) error {
	if err := checkEmailRequestLimits(ctx, s.resetLimiters, email, ip); err != nil {
		return err
	}

	// The lookup uses the email as given, same as Login/Register.
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Error("forgot password: failed to look up user", zap.Error(err))
		}
		return nil
	}
	if user.PasswordHash == nil {
		return nil
	}

	// Only the newest link works; used rows are kept so a repeated click on
	// an already-used link still answers token_used.
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND used_at IS NULL", user.ID).
		Delete(&domain.PasswordResetToken{}).Error; err != nil {
		s.logger.Error("forgot password: failed to delete unused reset tokens",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
		return nil
	}

	if err := s.issuePasswordReset(ctx, user); err != nil {
		s.logger.Error("forgot password: failed to issue password reset",
			zap.String("user_id", user.ID.String()),
			zap.Error(err),
		)
	}
	return nil
}

// issuePasswordReset creates a password reset token for user and enqueues
// the reset_password message. The plain token only ever leaves this
// function inside the email payload.
func (s *Service) issuePasswordReset(ctx context.Context, user *usersDomain.User) error {
	if s.mailer == nil {
		return errors.New("mailer not configured")
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("generate password reset token: %w", err)
	}
	plain := hex.EncodeToString(tokenBytes)

	now := time.Now()
	token := &domain.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Email:     user.Email,
		TokenHash: hashToken(plain),
		ExpiresAt: now.Add(passwordResetTokenTTL),
		CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(token).Error; err != nil {
		return fmt.Errorf("save password reset token: %w", err)
	}

	name := ""
	if user.Name != nil {
		name = strings.TrimSpace(*user.Name)
	}
	msg, err := mailer.Render(resetPasswordTemplate, mailer.ResetPasswordData{
		Name:     name,
		ResetURL: s.resetURL(plain),
	})
	if err != nil {
		return fmt.Errorf("render password reset email: %w", err)
	}
	msg.To = user.Email

	if err := s.mailer.Enqueue(ctx, msg); err != nil {
		return fmt.Errorf("enqueue password reset email: %w", err)
	}
	return nil
}

func (s *Service) resetURL(plain string) string {
	return strings.TrimRight(s.config.WebBaseURL, "/") + "/reset-password?" + url.Values{"token": {plain}}.Encode()
}

// ResetPassword consumes a password reset token and sets newPassword (the
// caller validates it first, so a rejected password never burns the link).
// In one transaction it marks the token used, updates the password hash,
// verifies the email if not yet verified (the link proves mailbox
// ownership), deletes the user's other unused reset tokens and revokes all
// of the user's refresh tokens. No tokens are issued. Errors:
// ErrPasswordTooLong (checked first, token untouched), ErrResetTokenInvalid,
// ErrResetTokenExpired, ErrResetTokenUsed.
func (s *Service) ResetPassword(ctx context.Context, plain, newPassword string) error {
	if len(newPassword) > MaxPasswordBytes {
		return ErrPasswordTooLong
	}
	if !isTokenFormat(plain) {
		return ErrResetTokenInvalid
	}

	var token domain.PasswordResetToken
	if err := s.db.WithContext(ctx).Where("token_hash = ?", hashToken(plain)).First(&token).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrResetTokenInvalid
		}
		return fmt.Errorf("find password reset token: %w", err)
	}

	user, err := s.users.GetByID(ctx, token.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrResetTokenInvalid
		}
		return fmt.Errorf("get user: %w", err)
	}
	if user.Email != token.Email {
		return ErrResetTokenInvalid
	}
	if token.UsedAt != nil {
		return ErrResetTokenUsed
	}
	if !time.Now().Before(token.ExpiresAt) {
		return ErrResetTokenExpired
	}

	// bcrypt is slow; keep it outside the transaction.
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	now := time.Now()
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&domain.PasswordResetToken{}).
			Where("id = ? AND used_at IS NULL", token.ID).
			Update("used_at", now)
		if res.Error != nil {
			return fmt.Errorf("mark password reset token used: %w", res.Error)
		}
		if res.RowsAffected == 0 {
			// A concurrent request used this token first.
			return ErrResetTokenUsed
		}
		if err := tx.Exec(
			"UPDATE users SET password_hash = ?, email_verified_at = COALESCE(email_verified_at, ?), updated_at = ? WHERE id = ?",
			string(hash), now, now, user.ID,
		).Error; err != nil {
			return fmt.Errorf("update password: %w", err)
		}
		if err := tx.Where("user_id = ? AND used_at IS NULL", user.ID).
			Delete(&domain.PasswordResetToken{}).Error; err != nil {
			return fmt.Errorf("delete unused password reset tokens: %w", err)
		}
		if err := revokeRefreshTokens(tx, user.ID, now); err != nil {
			return fmt.Errorf("revoke refresh tokens: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	// The reset link proves mailbox ownership, so it clears the login
	// throttle of the address. Failure is logged, not returned.
	if lim := s.loginLimiters.Email; lim != nil {
		if rerr := lim.Reset(ctx, emailRateLimitKey(user.Email)); rerr != nil {
			s.logger.Warn("reset password: failed to reset login throttle", zap.Error(rerr))
		}
	}
	return nil
}

// issueVerification creates a verification token for user and enqueues the
// verify_email message. The plain token only ever leaves this function
// inside the email payload.
func (s *Service) issueVerification(ctx context.Context, user *usersDomain.User) error {
	if s.mailer == nil {
		return errors.New("mailer not configured")
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return fmt.Errorf("generate verification token: %w", err)
	}
	plain := hex.EncodeToString(tokenBytes)

	now := time.Now()
	token := &domain.EmailVerificationToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Email:     user.Email,
		TokenHash: hashToken(plain),
		ExpiresAt: now.Add(s.config.VerificationTokenTTL),
		CreatedAt: now,
	}
	if err := s.db.WithContext(ctx).Create(token).Error; err != nil {
		return fmt.Errorf("save verification token: %w", err)
	}

	name := ""
	if user.Name != nil {
		name = strings.TrimSpace(*user.Name)
	}
	msg, err := mailer.Render(verifyEmailTemplate, mailer.VerifyEmailData{
		Name:      name,
		VerifyURL: s.verifyURL(plain),
		TTLHours:  int(s.config.VerificationTokenTTL / time.Hour),
	})
	if err != nil {
		return fmt.Errorf("render verification email: %w", err)
	}
	msg.To = user.Email

	if err := s.mailer.Enqueue(ctx, msg); err != nil {
		return fmt.Errorf("enqueue verification email: %w", err)
	}
	return nil
}

func (s *Service) verifyURL(plain string) string {
	return strings.TrimRight(s.config.WebBaseURL, "/") + "/verify-email?" + url.Values{"token": {plain}}.Encode()
}

// markVerificationTokenUsed sets used_at on an unused token and reports
// whether this call was the one that marked it.
func markVerificationTokenUsed(db *gorm.DB, id uuid.UUID, now time.Time) (bool, error) {
	res := db.Model(&domain.EmailVerificationToken{}).
		Where("id = ? AND used_at IS NULL", id).
		Update("used_at", now)
	if res.Error != nil {
		return false, fmt.Errorf("mark verification token used: %w", res.Error)
	}
	return res.RowsAffected > 0, nil
}

// isTokenFormat reports whether plain looks like a token issued
// by issueVerification or issuePasswordReset: exactly 64 hex characters.
func isTokenFormat(plain string) bool {
	if len(plain) != 64 {
		return false
	}
	_, err := hex.DecodeString(plain)
	return err == nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ConstantTimeCompare performs constant time comparison of two hashes.
func ConstantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
