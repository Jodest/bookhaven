package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
	usersDomain "github.com/bookhaven/backend/internal/modules/users/domain"
	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// fakeUsersRepository implements the users repository.Repository interface
// as an in-memory map, mirroring the pattern used in
// internal/modules/users/service/service_test.go.
type fakeUsersRepository struct {
	users map[uuid.UUID]*usersDomain.User
}

func newFakeUsersRepository() *fakeUsersRepository {
	return &fakeUsersRepository{users: make(map[uuid.UUID]*usersDomain.User)}
}

func (f *fakeUsersRepository) Create(ctx context.Context, user *usersDomain.User) error {
	copy := *user
	f.users[user.ID] = &copy
	return nil
}

func (f *fakeUsersRepository) GetByID(ctx context.Context, id uuid.UUID) (*usersDomain.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copy := *u
	return &copy, nil
}

func (f *fakeUsersRepository) GetByEmail(ctx context.Context, email string) (*usersDomain.User, error) {
	for _, u := range f.users {
		if u.Email == email {
			copy := *u
			return &copy, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}

func (f *fakeUsersRepository) Update(ctx context.Context, user *usersDomain.User) error {
	if _, ok := f.users[user.ID]; !ok {
		return gorm.ErrRecordNotFound
	}
	copy := *user
	f.users[user.ID] = &copy
	return nil
}

// refreshTokensDDL creates the refresh_tokens table using SQLite-compatible
// syntax. The production schema (backend/migrations/000002_refresh_tokens.up.sql)
// targets Postgres (uuid columns, gen_random_uuid() default), which SQLite's
// DDL parser cannot execute verbatim, so this test recreates the same shape
// (columns, nullability, primary key) with SQLite-native types instead of
// relying on AutoMigrate against the Postgres-tagged domain struct. The id
// default is built to produce a canonical dashed UUID string (rather than a
// bare 32-char hex blob) so that it round-trips correctly through
// google/uuid's Value()/Scan(), which GORM relies on when building the
// primary-key WHERE clause for the revoke Update() in RefreshTokens.
const refreshTokensDDL = `CREATE TABLE refresh_tokens (
	id TEXT PRIMARY KEY DEFAULT (lower(
		hex(randomblob(4)) || '-' || hex(randomblob(2)) || '-' ||
		hex(randomblob(2)) || '-' || hex(randomblob(2)) || '-' || hex(randomblob(6))
	)),
	user_id TEXT NOT NULL,
	token_hash TEXT NOT NULL,
	expires_at INTEGER NOT NULL,
	created_at INTEGER NOT NULL,
	revoked_at INTEGER
)`

// setupTestDB returns an in-memory SQLite-backed *gorm.DB with the
// refresh_tokens table created, for use by the auth Service (which reads
// and writes domain.RefreshToken directly via *gorm.DB).
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite db: %v", err)
	}
	if err := db.Exec(refreshTokensDDL).Error; err != nil {
		t.Fatalf("create refresh_tokens table: %v", err)
	}
	return db
}

func setupTestAuthService(t *testing.T) (*Service, *fakeUsersRepository) {
	t.Helper()
	db := setupTestDB(t)
	users := newFakeUsersRepository()
	cfg := Config{
		JWTSecret:       "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		Issuer:          "bookhaven-test",
	}
	return NewService(cfg, db, users), users
}

func createTestUser(t *testing.T, users *fakeUsersRepository) *usersDomain.User {
	t.Helper()
	name := "Test User"
	user := &usersDomain.User{
		ID:          uuid.New(),
		Email:       "test@example.com",
		Name:        &name,
		Role:        usersDomain.RoleUser,
		ProfileType: usersDomain.ProfileTypeReader,
	}
	if err := users.Create(context.Background(), user); err != nil {
		t.Fatalf("create test user: %v", err)
	}
	return user
}

// TestHashToken_Deterministic guards against a regression to a
// non-deterministic hash (e.g. bcrypt with its random per-call salt), which
// is exactly the bug this fix addresses: hashToken must map the same
// plaintext to the same output every time, since it is used as an exact-match
// SQL lookup key.
func TestHashToken_Deterministic(t *testing.T) {
	h1 := hashToken("some-plaintext-refresh-token")
	h2 := hashToken("some-plaintext-refresh-token")
	if h1 != h2 {
		t.Fatalf("expected hashToken to be deterministic, got %q and %q", h1, h2)
	}
	if h1 == "" {
		t.Fatal("expected a non-empty hash")
	}
}

// TestRefreshTokens_Success is the core regression test: a refresh token
// issued via createTokenPair (through CreateTokenPair) must be immediately
// usable via RefreshTokens. This is the exact scenario that always returned
// ErrTokenInvalid under the old bcrypt-based hashToken.
func TestRefreshTokens_Success(t *testing.T) {
	ctx := context.Background()
	svc, users := setupTestAuthService(t)
	user := createTestUser(t, users)

	issued, err := svc.CreateTokenPair(ctx, user.ID)
	if err != nil {
		t.Fatalf("create token pair: %v", err)
	}

	refreshed, err := svc.RefreshTokens(ctx, issued.RefreshToken)
	if err != nil {
		t.Fatalf("expected refresh to succeed, got error: %v", err)
	}
	if refreshed == nil {
		t.Fatal("expected a token pair to be returned")
	}
	if refreshed.AccessToken == "" || refreshed.RefreshToken == "" {
		t.Fatal("expected non-empty access and refresh tokens")
	}
}

// TestRefreshTokens_RotationRevokesOldToken verifies rotation semantics:
// the old refresh token becomes unusable after a successful refresh, while
// the newly issued refresh token works.
func TestRefreshTokens_RotationRevokesOldToken(t *testing.T) {
	ctx := context.Background()
	svc, users := setupTestAuthService(t)
	user := createTestUser(t, users)

	issued, err := svc.CreateTokenPair(ctx, user.ID)
	if err != nil {
		t.Fatalf("create token pair: %v", err)
	}

	rotated, err := svc.RefreshTokens(ctx, issued.RefreshToken)
	if err != nil {
		t.Fatalf("expected first refresh to succeed: %v", err)
	}

	if _, err := svc.RefreshTokens(ctx, issued.RefreshToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid for the revoked original token, got %v", err)
	}

	if _, err := svc.RefreshTokens(ctx, rotated.RefreshToken); err != nil {
		t.Fatalf("expected refresh with the newly issued token to succeed, got %v", err)
	}
}

// TestRefreshTokens_UnknownTokenRejected guards against overcorrecting into
// "RefreshTokens always succeeds" - a random plaintext with no matching row
// must still fail.
func TestRefreshTokens_UnknownTokenRejected(t *testing.T) {
	ctx := context.Background()
	svc, _ := setupTestAuthService(t)

	if _, err := svc.RefreshTokens(ctx, "totally-unknown-refresh-token"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
}

// TestRefreshTokens_ExpiredTokenRejected verifies expired tokens are still
// rejected under the new hashing scheme.
func TestRefreshTokens_ExpiredTokenRejected(t *testing.T) {
	ctx := context.Background()
	svc, users := setupTestAuthService(t)
	user := createTestUser(t, users)

	plain := "expired-refresh-token-plaintext"
	expired := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashToken(plain),
		ExpiresAt: time.Now().Add(-time.Hour).Unix(),
		CreatedAt: time.Now().Add(-2 * time.Hour).Unix(),
	}
	if err := svc.db.WithContext(ctx).Create(expired).Error; err != nil {
		t.Fatalf("seed expired refresh token: %v", err)
	}

	if _, err := svc.RefreshTokens(ctx, plain); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid for expired token, got %v", err)
	}
}

// TestRefreshTokens_RevokedTokenRejected verifies already-revoked tokens are
// still rejected under the new hashing scheme.
func TestRefreshTokens_RevokedTokenRejected(t *testing.T) {
	ctx := context.Background()
	svc, users := setupTestAuthService(t)
	user := createTestUser(t, users)

	plain := "revoked-refresh-token-plaintext"
	revokedAt := time.Now().Unix()
	revoked := &domain.RefreshToken{
		UserID:    user.ID,
		TokenHash: hashToken(plain),
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
		CreatedAt: time.Now().Unix(),
		RevokedAt: &revokedAt,
	}
	if err := svc.db.WithContext(ctx).Create(revoked).Error; err != nil {
		t.Fatalf("seed revoked refresh token: %v", err)
	}

	if _, err := svc.RefreshTokens(ctx, plain); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid for revoked token, got %v", err)
	}
}
