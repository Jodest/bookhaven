package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/modules/users/repository"
	"github.com/bookhaven/backend/internal/pkg/mailer"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// usersDDL / emailVerificationTokensDDL recreate the shape of the
// Postgres tables (000001 users, 000011 email_verification_tokens) with
// SQLite-native types, so the real users GormRepository and the raw
// "UPDATE users" inside VerifyEmail's transaction run against the same DB.
const usersDDL = `CREATE TABLE users (
	id TEXT PRIMARY KEY,
	email TEXT NOT NULL UNIQUE,
	password_hash TEXT,
	name TEXT,
	avatar_url TEXT,
	role TEXT NOT NULL DEFAULT 'user',
	profile_type TEXT NOT NULL DEFAULT 'reader',
	email_verified_at DATETIME,
	created_at DATETIME,
	updated_at DATETIME
)`

const emailVerificationTokensDDL = `CREATE TABLE email_verification_tokens (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	email TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at DATETIME NOT NULL,
	used_at DATETIME,
	created_at DATETIME NOT NULL
)`

// passwordResetTokensDDL mirrors migration 000012 password_reset_tokens.
const passwordResetTokensDDL = `CREATE TABLE password_reset_tokens (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	email TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at DATETIME NOT NULL,
	used_at DATETIME,
	created_at DATETIME NOT NULL
)`

type fakeMailer struct {
	mu       sync.Mutex
	messages []mailer.Message
	err      error
}

func (f *fakeMailer) Enqueue(ctx context.Context, msg mailer.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.messages = append(f.messages, msg)
	return nil
}

type fakeLimiter struct {
	keys       []string
	allowed    bool
	retryAfter time.Duration
	err        error
}

func (f *fakeLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	f.keys = append(f.keys, key)
	return f.allowed, f.retryAfter, f.err
}

func (f *fakeLimiter) calls() int { return len(f.keys) }

type verificationEnv struct {
	svc          *Service
	db           *gorm.DB
	users        repository.Repository
	mailer       *fakeMailer
	ipLimiter    *fakeLimiter
	emailLimiter *fakeLimiter
	// resetIPLimiter / resetEmailLimiter limit ForgotPassword.
	resetIPLimiter    *fakeLimiter
	resetEmailLimiter *fakeLimiter
}

func setupVerificationEnv(t *testing.T) *verificationEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	// One connection: every pooled connection to ":memory:" would get its
	// own empty database.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	for _, ddl := range []string{refreshTokensDDL, usersDDL, emailVerificationTokensDDL, passwordResetTokensDDL} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	users := repository.NewGormRepository(db)
	m := &fakeMailer{}
	ipL := &fakeLimiter{allowed: true}
	emailL := &fakeLimiter{allowed: true}
	resetIPL := &fakeLimiter{allowed: true}
	resetEmailL := &fakeLimiter{allowed: true}
	svc := NewService(Config{
		JWTSecret:       "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		Issuer:          "bookhaven-test",
		WebBaseURL:      "http://localhost:3000",
	}, db, users, m, ResendLimiters{IP: ipL, Email: emailL}, ResendLimiters{IP: resetIPL, Email: resetEmailL}, nil)
	return &verificationEnv{
		svc: svc, db: db, users: users, mailer: m,
		ipLimiter: ipL, emailLimiter: emailL,
		resetIPLimiter: resetIPL, resetEmailLimiter: resetEmailL,
	}
}

// tokenFromMessage extracts the plain token from the link in msg's text.
func tokenFromMessage(t *testing.T, msg mailer.Message) string {
	t.Helper()
	const marker = "http://localhost:3000/verify-email?token="
	i := strings.Index(msg.TextBody, marker)
	if i < 0 {
		t.Fatalf("message text has no verify link: %q", msg.TextBody)
	}
	rest := msg.TextBody[i+len(marker):]
	if len(rest) < 64 {
		t.Fatalf("token in link too short: %q", rest)
	}
	return rest[:64]
}

func (e *verificationEnv) register(t *testing.T, email string) (*usersDomain.User, string) {
	t.Helper()
	before := len(e.mailer.messages)
	user, err := e.svc.Register(context.Background(), email, "password123", "Анна")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	if len(e.mailer.messages) != before+1 {
		t.Fatalf("expected one verification email, got %d", len(e.mailer.messages)-before)
	}
	return user, tokenFromMessage(t, e.mailer.messages[len(e.mailer.messages)-1])
}

func (e *verificationEnv) tokens(t *testing.T, userID uuid.UUID) []domain.EmailVerificationToken {
	t.Helper()
	var rows []domain.EmailVerificationToken
	if err := e.db.Where("user_id = ?", userID).Find(&rows).Error; err != nil {
		t.Fatalf("list tokens: %v", err)
	}
	return rows
}

func (e *verificationEnv) isVerified(t *testing.T, userID uuid.UUID) bool {
	t.Helper()
	u, err := e.users.GetByID(context.Background(), userID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}
	return u.EmailVerifiedAt != nil
}

func (e *verificationEnv) insertToken(t *testing.T, userID uuid.UUID, email, plain string, expiresAt time.Time, usedAt *time.Time) {
	t.Helper()
	row := &domain.EmailVerificationToken{
		ID:        uuid.New(),
		UserID:    userID,
		Email:     email,
		TokenHash: hashToken(plain),
		ExpiresAt: expiresAt,
		UsedAt:    usedAt,
		CreatedAt: time.Now(),
	}
	if err := e.db.Create(row).Error; err != nil {
		t.Fatalf("insert token: %v", err)
	}
}

func (e *verificationEnv) markVerified(t *testing.T, userID uuid.UUID) {
	t.Helper()
	if err := e.db.Exec("UPDATE users SET email_verified_at = ? WHERE id = ?", time.Now(), userID).Error; err != nil {
		t.Fatalf("mark verified: %v", err)
	}
}

func plainToken(c byte) string { return strings.Repeat(string(c), 64) }

func TestRegister_IssuesVerificationTokenAndEmail(t *testing.T) {
	env := setupVerificationEnv(t)
	user, plain := env.register(t, "anna@example.com")

	if user.EmailVerifiedAt != nil {
		t.Fatal("new user must not be verified")
	}
	msg := env.mailer.messages[0]
	if msg.To != "anna@example.com" {
		t.Errorf("email sent to %q", msg.To)
	}
	if !strings.Contains(msg.Subject, "Подтвердите") {
		t.Errorf("unexpected subject %q", msg.Subject)
	}
	if !strings.Contains(msg.TextBody, "/verify-email?token="+plain) {
		t.Errorf("text does not contain link: %q", msg.TextBody)
	}

	rows := env.tokens(t, user.ID)
	if len(rows) != 1 {
		t.Fatalf("expected 1 token row, got %d", len(rows))
	}
	row := rows[0]
	if row.TokenHash == plain {
		t.Fatal("token stored in plain text")
	}
	if row.TokenHash != hashToken(plain) {
		t.Fatal("stored hash does not match sha256 of the emailed token")
	}
	if row.Email != user.Email || row.UsedAt != nil {
		t.Fatalf("unexpected token row: %+v", row)
	}
	if d := time.Until(row.ExpiresAt); d < 23*time.Hour+59*time.Minute || d > 24*time.Hour {
		t.Fatalf("expected expiry ~24h from now, got %v", d)
	}
}

func TestRegister_EnqueueFailureDoesNotFailRegistration(t *testing.T) {
	env := setupVerificationEnv(t)
	env.mailer.err = errors.New("redis down")

	user, err := env.svc.Register(context.Background(), "anna@example.com", "password123", "Анна")
	if err != nil {
		t.Fatalf("expected registration to succeed, got %v", err)
	}
	if user == nil {
		t.Fatal("expected user")
	}
	if _, err := env.users.GetByID(context.Background(), user.ID); err != nil {
		t.Fatalf("user not persisted: %v", err)
	}
}

func TestRegister_TokenInsertFailureDoesNotFailRegistration(t *testing.T) {
	env := setupVerificationEnv(t)
	if err := env.db.Exec("DROP TABLE email_verification_tokens").Error; err != nil {
		t.Fatalf("drop table: %v", err)
	}

	user, err := env.svc.Register(context.Background(), "anna@example.com", "password123", "Анна")
	if err != nil {
		t.Fatalf("expected registration to succeed, got %v", err)
	}
	if user == nil {
		t.Fatal("expected user")
	}
	if len(env.mailer.messages) != 0 {
		t.Fatal("no email expected when the token could not be stored")
	}
}

func TestVerifyEmail_Success(t *testing.T) {
	env := setupVerificationEnv(t)
	user, plain := env.register(t, "anna@example.com")

	already, err := env.svc.VerifyEmail(context.Background(), plain)
	if err != nil || already {
		t.Fatalf("expected verified, got already=%v err=%v", already, err)
	}
	if !env.isVerified(t, user.ID) {
		t.Fatal("user not marked verified")
	}
	if rows := env.tokens(t, user.ID); rows[0].UsedAt == nil {
		t.Fatal("token not marked used")
	}

	// Second click on the same link.
	already, err = env.svc.VerifyEmail(context.Background(), plain)
	if err != nil || !already {
		t.Fatalf("expected already_verified, got already=%v err=%v", already, err)
	}
}

func TestVerifyEmail_Expired(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	plain := plainToken('a')
	env.insertToken(t, user.ID, user.Email, plain, time.Now().Add(-time.Minute), nil)

	if _, err := env.svc.VerifyEmail(context.Background(), plain); !errors.Is(err, ErrVerificationTokenExpired) {
		t.Fatalf("expected ErrVerificationTokenExpired, got %v", err)
	}
	if env.isVerified(t, user.ID) {
		t.Fatal("user must not be verified by an expired token")
	}
}

func TestVerifyEmail_InvalidTokens(t *testing.T) {
	env := setupVerificationEnv(t)
	env.register(t, "anna@example.com")

	for _, token := range []string{"", "short", strings.Repeat("z", 64), plainToken('b')} {
		if _, err := env.svc.VerifyEmail(context.Background(), token); !errors.Is(err, ErrVerificationTokenInvalid) {
			t.Fatalf("token %q: expected ErrVerificationTokenInvalid, got %v", token, err)
		}
	}
}

func TestVerifyEmail_EmailMismatch(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	plain := plainToken('c')
	env.insertToken(t, user.ID, "old@example.com", plain, time.Now().Add(time.Hour), nil)

	if _, err := env.svc.VerifyEmail(context.Background(), plain); !errors.Is(err, ErrVerificationTokenInvalid) {
		t.Fatalf("expected ErrVerificationTokenInvalid, got %v", err)
	}
	if env.isVerified(t, user.ID) {
		t.Fatal("user must not be verified")
	}
}

func TestVerifyEmail_UsedTokenUnverifiedUser(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	plain := plainToken('d')
	used := time.Now().Add(-time.Minute)
	env.insertToken(t, user.ID, user.Email, plain, time.Now().Add(time.Hour), &used)

	if _, err := env.svc.VerifyEmail(context.Background(), plain); !errors.Is(err, ErrVerificationTokenUsed) {
		t.Fatalf("expected ErrVerificationTokenUsed, got %v", err)
	}
}

func TestVerifyEmail_VerifiedUserFreshTokenMarkedUsed(t *testing.T) {
	env := setupVerificationEnv(t)
	user, plain := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)

	already, err := env.svc.VerifyEmail(context.Background(), plain)
	if err != nil || !already {
		t.Fatalf("expected already_verified, got already=%v err=%v", already, err)
	}
	if rows := env.tokens(t, user.ID); rows[0].UsedAt == nil {
		t.Fatal("expected token to be marked used")
	}
}

func TestVerifyEmail_VerifiedUserExpiredToken(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	plain := plainToken('e')
	env.insertToken(t, user.ID, user.Email, plain, time.Now().Add(-time.Hour), nil)
	env.markVerified(t, user.ID)

	already, err := env.svc.VerifyEmail(context.Background(), plain)
	if err != nil || !already {
		t.Fatalf("expected already_verified, got already=%v err=%v", already, err)
	}
}
