package http

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/infrastructure/validator"
	authDomain "github.com/bookhaven/backend/internal/modules/auth/domain"
	"github.com/bookhaven/backend/internal/modules/auth/service"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/bookhaven/backend/internal/modules/users/repository"
	"github.com/bookhaven/backend/internal/pkg/mailer"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const testUsersDDL = `CREATE TABLE users (
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

const testEmailVerificationTokensDDL = `CREATE TABLE email_verification_tokens (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	email TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at DATETIME NOT NULL,
	used_at DATETIME,
	created_at DATETIME NOT NULL
)`

type recordingMailer struct {
	messages []mailer.Message
	err      error
}

func (m *recordingMailer) Enqueue(ctx context.Context, msg mailer.Message) error {
	if m.err != nil {
		return m.err
	}
	m.messages = append(m.messages, msg)
	return nil
}

type stubLimiter struct {
	calls      int
	lastKey    string
	allowed    bool
	retryAfter time.Duration
	err        error
}

func (l *stubLimiter) Allow(ctx context.Context, key string) (bool, time.Duration, error) {
	l.calls++
	l.lastKey = key
	return l.allowed, l.retryAfter, l.err
}

// testRefreshTokensDDL mirrors service_test.go's SQLite refresh_tokens table.
const testRefreshTokensDDL = `CREATE TABLE refresh_tokens (
	id TEXT PRIMARY KEY DEFAULT (lower(
		hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' ||
		hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))
	)),
	user_id TEXT NOT NULL,
	token_hash TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	revoked_at INTEGER,
	family_id TEXT NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
	replaced_by TEXT
)`

type verificationHandlerEnv struct {
	h            *Handler
	svc          *service.Service
	db           *gorm.DB
	users        repository.Repository
	mailer       *recordingMailer
	ipLimiter    *stubLimiter
	emailLimiter *stubLimiter
	// resetIPLimiter / resetEmailLimiter limit forgot-password.
	resetIPLimiter    *stubLimiter
	resetEmailLimiter *stubLimiter
	user              *usersDomain.User
}

const testPasswordResetTokensDDL = `CREATE TABLE password_reset_tokens (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	email TEXT NOT NULL,
	token_hash TEXT NOT NULL UNIQUE,
	expires_at DATETIME NOT NULL,
	used_at DATETIME,
	created_at DATETIME NOT NULL
)`

func setupVerificationHandler(t *testing.T) *verificationHandlerEnv {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	for _, ddl := range []string{testUsersDDL, testEmailVerificationTokensDDL, testRefreshTokensDDL, testPasswordResetTokensDDL} {
		if err := db.Exec(ddl).Error; err != nil {
			t.Fatalf("create table: %v", err)
		}
	}
	users := repository.NewGormRepository(db)
	m := &recordingMailer{}
	ipL := &stubLimiter{allowed: true}
	emailL := &stubLimiter{allowed: true}
	resetIPL := &stubLimiter{allowed: true}
	resetEmailL := &stubLimiter{allowed: true}
	svc := service.NewService(service.Config{
		JWTSecret:       "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		Issuer:          "bookhaven-test",
		WebBaseURL:      "http://localhost:3000",
	}, db, users, m, service.ResendLimiters{IP: ipL, Email: emailL}, service.ResendLimiters{IP: resetIPL, Email: resetEmailL}, service.LoginLimiters{}, nil)

	name := "Анна"
	user := &usersDomain.User{
		ID:          uuid.New(),
		Email:       "anna@example.com",
		Name:        &name,
		Role:        usersDomain.RoleUser,
		ProfileType: usersDomain.ProfileTypeReader,
	}
	if err := users.Create(context.Background(), user); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return &verificationHandlerEnv{
		h: NewHandler(svc), svc: svc, db: db, users: users, mailer: m,
		ipLimiter: ipL, emailLimiter: emailL,
		resetIPLimiter: resetIPL, resetEmailLimiter: resetEmailL,
		user: user,
	}
}

func (e *verificationHandlerEnv) insertToken(t *testing.T, plain string, expiresAt time.Time, usedAt *time.Time) {
	t.Helper()
	sum := sha256Hex(plain)
	row := &authDomain.EmailVerificationToken{
		ID:        uuid.New(),
		UserID:    e.user.ID,
		Email:     e.user.Email,
		TokenHash: sum,
		ExpiresAt: expiresAt,
		UsedAt:    usedAt,
		CreatedAt: time.Now(),
	}
	if err := e.db.Create(row).Error; err != nil {
		t.Fatalf("insert token: %v", err)
	}
}

func (e *verificationHandlerEnv) markVerified(t *testing.T) {
	t.Helper()
	if err := e.db.Exec("UPDATE users SET email_verified_at = ? WHERE id = ?", time.Now(), e.user.ID).Error; err != nil {
		t.Fatalf("mark verified: %v", err)
	}
}

func postJSON(path, body string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Validator = validator.New()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}
	return body
}

func TestVerifyEmailHandler(t *testing.T) {
	valid := strings.Repeat("a", 64)
	cases := []struct {
		name       string
		setup      func(t *testing.T, env *verificationHandlerEnv)
		body       string
		wantStatus int
		wantKey    string
		wantValue  string
	}{
		{
			name: "valid token",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				env.insertToken(t, valid, time.Now().Add(time.Hour), nil)
			},
			body:       `{"token":"` + valid + `"}`,
			wantStatus: http.StatusOK, wantKey: "status", wantValue: "verified",
		},
		{
			name: "already verified",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				used := time.Now()
				env.insertToken(t, valid, time.Now().Add(-time.Hour), &used)
				env.markVerified(t)
			},
			body:       `{"token":"` + valid + `"}`,
			wantStatus: http.StatusOK, wantKey: "status", wantValue: "already_verified",
		},
		{
			name:       "malformed token",
			body:       `{"token":"abc"}`,
			wantStatus: http.StatusBadRequest, wantKey: "code", wantValue: "invalid_token",
		},
		{
			name:       "missing token",
			body:       `{}`,
			wantStatus: http.StatusBadRequest, wantKey: "code", wantValue: "invalid_token",
		},
		{
			name:       "unknown token",
			body:       `{"token":"` + strings.Repeat("b", 64) + `"}`,
			wantStatus: http.StatusBadRequest, wantKey: "code", wantValue: "invalid_token",
		},
		{
			name: "expired token",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				env.insertToken(t, valid, time.Now().Add(-time.Minute), nil)
			},
			body:       `{"token":"` + valid + `"}`,
			wantStatus: http.StatusBadRequest, wantKey: "code", wantValue: "token_expired",
		},
		{
			name: "used token, unverified user",
			setup: func(t *testing.T, env *verificationHandlerEnv) {
				used := time.Now()
				env.insertToken(t, valid, time.Now().Add(time.Hour), &used)
			},
			body:       `{"token":"` + valid + `"}`,
			wantStatus: http.StatusBadRequest, wantKey: "code", wantValue: "token_used",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupVerificationHandler(t)
			if tc.setup != nil {
				tc.setup(t, env)
			}
			c, rec := postJSON("/api/v1/auth/verify-email", tc.body)
			if err := env.h.VerifyEmail(c); err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tc.wantStatus, rec.Body.String())
			}
			body := decodeBody(t, rec)
			if body[tc.wantKey] != tc.wantValue {
				t.Fatalf("%s = %v, want %s (body %s)", tc.wantKey, body[tc.wantKey], tc.wantValue, rec.Body.String())
			}
			if rec.Code >= 400 && body["error"] == nil {
				t.Fatalf("error responses must keep the error field: %s", rec.Body.String())
			}
			if len(rec.Result().Cookies()) != 0 {
				t.Fatal("verify-email must not set cookies")
			}
		})
	}
}

func TestMeHandler_IncludesEmailVerified(t *testing.T) {
	env := setupVerificationHandler(t)
	for _, verified := range []bool{false, true} {
		user := *env.user
		if verified {
			now := time.Now()
			user.EmailVerifiedAt = &now
		}
		e := echo.New()
		rec := httptest.NewRecorder()
		c := e.NewContext(httptest.NewRequest(http.MethodGet, "/api/v1/auth/me", nil), rec)
		c.Set("user", &user)
		if err := env.h.Me(c); err != nil {
			t.Fatalf("handler error: %v", err)
		}
		body := decodeBody(t, rec)
		if body["email_verified"] != verified {
			t.Fatalf("email_verified = %v, want %v (body %s)", body["email_verified"], verified, rec.Body.String())
		}
	}
}

// sha256Hex mirrors the service's token hashing (sha256, hex).
func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}
