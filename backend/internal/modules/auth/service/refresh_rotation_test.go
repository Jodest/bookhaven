package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func setupObservedAuthService(t *testing.T) (*Service, *fakeUsersRepository, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.WarnLevel)
	db := setupTestDB(t)
	users := newFakeUsersRepository()
	cfg := Config{
		JWTSecret:       "test-secret",
		AccessTokenTTL:  15 * time.Minute,
		RefreshTokenTTL: 30 * 24 * time.Hour,
		Issuer:          "bookhaven-test",
	}
	return NewService(cfg, db, users, nil, ResendLimiters{}, ResendLimiters{}, LoginLimiters{}, RegisterLimiters{}, zap.New(core)), users, logs
}

func findToken(t *testing.T, svc *Service, plain string) domain.RefreshToken {
	t.Helper()
	var tok domain.RefreshToken
	if err := svc.db.Where("token_hash = ?", hashToken(plain)).First(&tok).Error; err != nil {
		t.Fatalf("find token: %v", err)
	}
	return tok
}

// setRevokedAgo backdates the revocation of a token row.
func setRevokedAgo(t *testing.T, svc *Service, plain string, ago time.Duration) {
	t.Helper()
	if err := svc.db.Model(&domain.RefreshToken{}).Where("token_hash = ?", hashToken(plain)).
		Update("revoked_at", time.Now().Add(-ago).Unix()).Error; err != nil {
		t.Fatalf("backdate revoked_at: %v", err)
	}
}

func TestRefresh_LoginStartsFamilyAndRotationKeepsIt(t *testing.T) {
	ctx := context.Background()
	svc, users, _ := setupObservedAuthService(t)
	user := createTestUser(t, users)

	issued, err := svc.CreateTokenPair(ctx, user.ID)
	if err != nil {
		t.Fatal(err)
	}
	parent := findToken(t, svc, issued.RefreshToken)
	if parent.FamilyID != parent.ID {
		t.Fatalf("login token must have family_id = id, got %s vs %s", parent.FamilyID, parent.ID)
	}

	next, err := svc.RefreshTokens(ctx, issued.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	child := findToken(t, svc, next.RefreshToken)
	parent = findToken(t, svc, issued.RefreshToken)
	if child.FamilyID != parent.FamilyID {
		t.Fatal("rotation must keep the family")
	}
	if parent.ReplacedBy == nil || *parent.ReplacedBy != child.ID {
		t.Fatal("parent.replaced_by must point at the successor")
	}
	if parent.RevokedAt == nil || child.RevokedAt != nil {
		t.Fatal("parent must be revoked and the child active")
	}
}

func TestRefresh_StaleSnapshotRotationIsSuperseded(t *testing.T) {
	ctx := context.Background()
	svc, users, _ := setupObservedAuthService(t)
	user := createTestUser(t, users)

	issued, _ := svc.CreateTokenPair(ctx, user.ID)
	stale := findToken(t, svc, issued.RefreshToken)

	if _, err := svc.RefreshTokens(ctx, issued.RefreshToken); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.rotate(ctx, &stale); !errors.Is(err, ErrRefreshSuperseded) {
		t.Fatalf("expected ErrRefreshSuperseded, got %v", err)
	}
	var n int64
	svc.db.Model(&domain.RefreshToken{}).Where("family_id = ?", stale.FamilyID).Count(&n)
	if n != 2 {
		t.Fatalf("expected exactly one successor (2 rows in family), got %d", n)
	}
}

func TestRefresh_ReplayWithinGraceDoesNotRevoke(t *testing.T) {
	ctx := context.Background()
	svc, users, logs := setupObservedAuthService(t)
	user := createTestUser(t, users)

	issued, _ := svc.CreateTokenPair(ctx, user.ID)
	next, err := svc.RefreshTokens(ctx, issued.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	setRevokedAgo(t, svc, issued.RefreshToken, 5*time.Second)

	if _, err := svc.RefreshTokens(ctx, issued.RefreshToken); !errors.Is(err, ErrRefreshSuperseded) {
		t.Fatalf("expected ErrRefreshSuperseded, got %v", err)
	}
	if _, err := svc.RefreshTokens(ctx, next.RefreshToken); err != nil {
		t.Fatalf("successor must still work: %v", err)
	}
	if logs.Len() != 0 {
		t.Fatal("no warning expected within the grace window")
	}
}

func TestRefresh_ReuseAfterGraceRevokesOnlyFamily(t *testing.T) {
	ctx := context.Background()
	svc, users, logs := setupObservedAuthService(t)
	user := createTestUser(t, users)

	laptop, _ := svc.CreateTokenPair(ctx, user.ID)
	phone, _ := svc.CreateTokenPair(ctx, user.ID)
	laptopNext, err := svc.RefreshTokens(ctx, laptop.RefreshToken)
	if err != nil {
		t.Fatal(err)
	}
	setRevokedAgo(t, svc, laptop.RefreshToken, time.Minute)

	if _, err := svc.RefreshTokens(ctx, laptop.RefreshToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
	if _, err := svc.RefreshTokens(ctx, laptopNext.RefreshToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("laptop session must be dead, got %v", err)
	}
	if _, err := svc.RefreshTokens(ctx, phone.RefreshToken); err != nil {
		t.Fatalf("phone session must survive: %v", err)
	}

	entries := logs.FilterMessage("refresh token reuse detected").All()
	if len(entries) != 1 {
		t.Fatalf("expected one reuse warning, got %d", len(entries))
	}
	dump := entries[0].Message
	for _, f := range entries[0].Context {
		dump += " " + f.String
	}
	if strings.Contains(dump, laptop.RefreshToken) || strings.Contains(dump, hashToken(laptop.RefreshToken)) {
		t.Fatal("warning must not contain the token or its hash")
	}
}

func TestRefresh_LogoutRevokedTokenIsNotReuse(t *testing.T) {
	ctx := context.Background()
	svc, users, logs := setupObservedAuthService(t)
	user := createTestUser(t, users)

	issued, _ := svc.CreateTokenPair(ctx, user.ID)
	if err := svc.Logout(ctx, user.ID); err != nil {
		t.Fatal(err)
	}
	setRevokedAgo(t, svc, issued.RefreshToken, time.Minute)

	if _, err := svc.RefreshTokens(ctx, issued.RefreshToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
	if logs.Len() != 0 {
		t.Fatal("no warning expected for a logout-revoked token")
	}
}

func TestRefresh_UnknownTokenNoWarning(t *testing.T) {
	svc, _, logs := setupObservedAuthService(t)
	if _, err := svc.RefreshTokens(context.Background(), "purged-token"); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
	if logs.Len() != 0 {
		t.Fatal("no warning expected for an unknown token")
	}
}

func TestRefresh_LegacyRowRefreshes(t *testing.T) {
	ctx := context.Background()
	svc, users, _ := setupObservedAuthService(t)
	user := createTestUser(t, users)

	id := uuid.New()
	plain := "legacy-token"
	row := &domain.RefreshToken{
		ID: id, FamilyID: id, UserID: user.ID, TokenHash: hashToken(plain),
		ExpiresAt: time.Now().Add(time.Hour).Unix(), CreatedAt: time.Now().Unix(),
	}
	if err := svc.db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := svc.RefreshTokens(ctx, plain); err != nil {
		t.Fatalf("legacy token must refresh: %v", err)
	}
}

func TestRefresh_UnverifiedEmailRevokesWithoutRotationLink(t *testing.T) {
	ctx := context.Background()
	svc, users, logs := setupObservedAuthService(t)
	user := createTestUser(t, users)

	issued, _ := svc.CreateTokenPair(ctx, user.ID)
	users.users[user.ID].EmailVerifiedAt = nil

	if _, err := svc.RefreshTokens(ctx, issued.RefreshToken); !errors.Is(err, ErrEmailNotVerified) {
		t.Fatalf("expected ErrEmailNotVerified, got %v", err)
	}
	tok := findToken(t, svc, issued.RefreshToken)
	if tok.RevokedAt == nil || tok.ReplacedBy != nil {
		t.Fatal("token must be revoked without replaced_by")
	}
	setRevokedAgo(t, svc, issued.RefreshToken, time.Minute)
	if _, err := svc.RefreshTokens(ctx, issued.RefreshToken); !errors.Is(err, ErrTokenInvalid) {
		t.Fatalf("expected ErrTokenInvalid, got %v", err)
	}
	if logs.Len() != 0 {
		t.Fatal("no warning expected")
	}
}
