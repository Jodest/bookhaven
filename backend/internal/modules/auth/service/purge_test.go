package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

var purgeNow = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

const purgeDay = 24 * time.Hour

func purgeClock() time.Time { return purgeNow }

func newPurgeJob(env *verificationEnv, batch int) *TokenPurgeJob {
	return NewTokenPurgeJob(env.db, zap.NewNop(), WithPurgeClock(purgeClock), WithPurgeBatchSize(batch))
}

func seedRefresh(t *testing.T, db *gorm.DB, expiresAt time.Time, revokedAt *time.Time) uuid.UUID {
	t.Helper()
	row := &domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		TokenHash: uuid.NewString(),
		ExpiresAt: expiresAt.Unix(),
		CreatedAt: purgeNow.Add(-60 * purgeDay).Unix(),
	}
	if revokedAt != nil {
		v := revokedAt.Unix()
		row.RevokedAt = &v
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("seed refresh: %v", err)
	}
	return row.ID
}

func seedVerification(t *testing.T, db *gorm.DB, expiresAt time.Time, used bool) uuid.UUID {
	t.Helper()
	row := &domain.EmailVerificationToken{
		ID: uuid.New(), UserID: uuid.New(), Email: "a@example.com",
		TokenHash: uuid.NewString(), ExpiresAt: expiresAt.UTC(), CreatedAt: purgeNow.Add(-60 * purgeDay).UTC(),
	}
	if used {
		u := expiresAt.Add(-time.Hour).UTC()
		row.UsedAt = &u
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("seed verification: %v", err)
	}
	return row.ID
}

func seedReset(t *testing.T, db *gorm.DB, expiresAt time.Time, used bool) uuid.UUID {
	t.Helper()
	row := &domain.PasswordResetToken{
		ID: uuid.New(), UserID: uuid.New(), Email: "a@example.com",
		TokenHash: uuid.NewString(), ExpiresAt: expiresAt.UTC(), CreatedAt: purgeNow.Add(-60 * purgeDay).UTC(),
	}
	if used {
		u := expiresAt.Add(-time.Minute).UTC()
		row.UsedAt = &u
	}
	if err := db.Create(row).Error; err != nil {
		t.Fatalf("seed reset: %v", err)
	}
	return row.ID
}

func countRows(t *testing.T, db *gorm.DB, table string) int64 {
	t.Helper()
	var n int64
	if err := db.Table(table).Count(&n).Error; err != nil {
		t.Fatalf("count %s: %v", table, err)
	}
	return n
}

func rowExists(t *testing.T, db *gorm.DB, table string, id uuid.UUID) bool {
	t.Helper()
	var n int64
	if err := db.Table(table).Where("id = ?", id).Count(&n).Error; err != nil {
		t.Fatalf("exists %s: %v", table, err)
	}
	return n == 1
}

func TestPurge_CutoffsPerTable(t *testing.T) {
	env := setupVerificationEnv(t)
	ago := func(d time.Duration) time.Time { return purgeNow.Add(-d) }
	future := purgeNow.Add(30 * purgeDay)

	// id -> must be deleted.
	rev6 := ago(6 * purgeDay)
	rev8 := ago(8 * purgeDay)
	refresh := map[uuid.UUID]bool{
		seedRefresh(t, env.db, future, nil):                 false, // active
		seedRefresh(t, env.db, future, &rev6):               false, // revoked 6d ago
		seedRefresh(t, env.db, future, &rev8):               true,  // revoked 8d ago
		seedRefresh(t, env.db, ago(6*purgeDay), nil):        false, // expired 6d ago
		seedRefresh(t, env.db, ago(8*purgeDay), nil):        true,  // expired 8d ago
		seedRefresh(t, env.db, ago(8*purgeDay), &rev8):      true,  // both old
		seedRefresh(t, env.db, purgeNow.Add(purgeDay), nil): false, // not yet expired
	}
	verif := map[uuid.UUID]bool{
		seedVerification(t, env.db, ago(29*purgeDay), false):        false,
		seedVerification(t, env.db, ago(29*purgeDay), true):         false,
		seedVerification(t, env.db, ago(31*purgeDay), false):        true,
		seedVerification(t, env.db, ago(31*purgeDay), true):         true,
		seedVerification(t, env.db, purgeNow.Add(time.Hour), false): false,
	}
	reset := map[uuid.UUID]bool{
		seedReset(t, env.db, ago(29*purgeDay), false):        false,
		seedReset(t, env.db, ago(29*purgeDay), true):         false,
		seedReset(t, env.db, ago(31*purgeDay), false):        true,
		seedReset(t, env.db, ago(31*purgeDay), true):         true,
		seedReset(t, env.db, purgeNow.Add(time.Hour), false): false,
	}

	stats, err := newPurgeJob(env, 2).Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.RefreshDeleted != 3 || stats.VerificationDeleted != 2 || stats.ResetDeleted != 2 {
		t.Fatalf("stats = %+v, want 3/2/2", stats)
	}
	check := func(table string, m map[uuid.UUID]bool) {
		for id, wantDeleted := range m {
			if gone := !rowExists(t, env.db, table, id); gone != wantDeleted {
				t.Errorf("%s %s: deleted=%v, want %v", table, id, gone, wantDeleted)
			}
		}
	}
	check("refresh_tokens", refresh)
	check("email_verification_tokens", verif)
	check("password_reset_tokens", reset)

	// Second run is a no-op.
	stats, err = newPurgeJob(env, 2).Run(context.Background())
	if err != nil || stats != (PurgeStats{}) {
		t.Fatalf("second run = %+v, %v; want zeros", stats, err)
	}
}

func TestPurge_MoreRowsThanBatch(t *testing.T) {
	env := setupVerificationEnv(t)
	old := purgeNow.Add(-40 * purgeDay)
	for i := 0; i < 7; i++ {
		seedRefresh(t, env.db, old, nil)
		seedVerification(t, env.db, old, i%2 == 0)
		seedReset(t, env.db, old, i%2 == 0)
	}
	stats, err := newPurgeJob(env, 2).Run(context.Background())
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if stats.RefreshDeleted != 7 || stats.VerificationDeleted != 7 || stats.ResetDeleted != 7 {
		t.Fatalf("stats = %+v, want 7/7/7", stats)
	}
	for _, tbl := range []string{"refresh_tokens", "email_verification_tokens", "password_reset_tokens"} {
		if n := countRows(t, env.db, tbl); n != 0 {
			t.Errorf("%s has %d rows left", tbl, n)
		}
	}
}

func TestPurge_CancelledContextStopsWithError(t *testing.T) {
	env := setupVerificationEnv(t)
	old := purgeNow.Add(-40 * purgeDay)
	for i := 0; i < 5; i++ {
		seedRefresh(t, env.db, old, nil)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newPurgeJob(env, 2).Run(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if n := countRows(t, env.db, "refresh_tokens"); n != 5 {
		t.Errorf("rows = %d, want 5 untouched", n)
	}
}

func TestPurge_TableFailureDoesNotStopOthers(t *testing.T) {
	env := setupVerificationEnv(t)
	old := purgeNow.Add(-40 * purgeDay)
	seedVerification(t, env.db, old, false)
	seedReset(t, env.db, old, false)
	if err := env.db.Exec("DROP TABLE refresh_tokens").Error; err != nil {
		t.Fatal(err)
	}
	stats, err := newPurgeJob(env, 2).Run(context.Background())
	if err == nil {
		t.Fatal("expected an aggregated error")
	}
	if stats.VerificationDeleted != 1 || stats.ResetDeleted != 1 {
		t.Fatalf("stats = %+v, other tables must still be purged", stats)
	}
}

func TestPurgeTask_TypeAndOptions(t *testing.T) {
	if TaskTypePurgeTokens != "auth:purge_tokens" || NewPurgeTokensTask().Type() != "auth:purge_tokens" {
		t.Errorf("task type = %q", NewPurgeTokensTask().Type())
	}
	var queue string
	var unique, timeout time.Duration
	var retry int
	for _, o := range PurgeTaskOptions() {
		switch o.Type() {
		case asynq.QueueOpt:
			queue = o.Value().(string)
		case asynq.UniqueOpt:
			unique = o.Value().(time.Duration)
		case asynq.TimeoutOpt:
			timeout = o.Value().(time.Duration)
		case asynq.MaxRetryOpt:
			retry = o.Value().(int)
		}
	}
	if queue != "auth_maintenance" || unique != 6*time.Hour || timeout != 10*time.Minute || retry != 2 {
		t.Errorf("queue=%q unique=%v timeout=%v retry=%d", queue, unique, timeout, retry)
	}
}

func TestPurge_VerifyEmailAfterPurge(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)
	now := time.Now().UTC()
	oldPlain, freshPlain := plainToken('a'), plainToken('b')
	used := now.Add(-45 * purgeDay)
	env.insertToken(t, user.ID, user.Email, oldPlain, now.Add(-40*purgeDay), &used)
	usedRecent := now.Add(-10 * purgeDay)
	env.insertToken(t, user.ID, user.Email, freshPlain, now.Add(-9*purgeDay), &usedRecent)

	// Before the purge the old link still reports "already verified".
	if already, err := env.svc.VerifyEmail(context.Background(), oldPlain); err != nil || !already {
		t.Fatalf("before purge: already=%v err=%v", already, err)
	}
	if _, err := NewTokenPurgeJob(env.db, zap.NewNop()).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := env.svc.VerifyEmail(context.Background(), oldPlain); !errors.Is(err, ErrVerificationTokenInvalid) {
		t.Fatalf("after purge err = %v, want ErrVerificationTokenInvalid", err)
	}
	// Inside the retention window the link keeps its answer.
	if already, err := env.svc.VerifyEmail(context.Background(), freshPlain); err != nil || !already {
		t.Fatalf("retained token: already=%v err=%v", already, err)
	}
}

func TestPurge_ResetPasswordAfterPurge(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)
	readHash := func() string {
		var hash string
		if err := env.db.Raw("SELECT COALESCE(password_hash, '') FROM users WHERE id = ?", user.ID).Scan(&hash).Error; err != nil {
			t.Fatalf("read hash: %v", err)
		}
		return hash
	}
	before := readHash()
	now := time.Now().UTC()
	plain := plainToken('c')
	usedAt := now.Add(-45 * purgeDay)
	row := &domain.PasswordResetToken{
		ID: uuid.New(), UserID: user.ID, Email: user.Email, TokenHash: hashToken(plain),
		ExpiresAt: now.Add(-44 * purgeDay), UsedAt: &usedAt, CreatedAt: now.Add(-45 * purgeDay),
	}
	if err := env.db.Create(row).Error; err != nil {
		t.Fatal(err)
	}
	if err := env.svc.ResetPassword(context.Background(), plain, "newpassword123"); !errors.Is(err, ErrResetTokenUsed) {
		t.Fatalf("before purge err = %v, want ErrResetTokenUsed", err)
	}
	if _, err := NewTokenPurgeJob(env.db, zap.NewNop()).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if err := env.svc.ResetPassword(context.Background(), plain, "newpassword123"); !errors.Is(err, ErrResetTokenInvalid) {
		t.Fatalf("after purge err = %v, want ErrResetTokenInvalid", err)
	}
	if readHash() != before {
		t.Fatal("password must be unchanged")
	}
}

func TestPurge_ActiveRefreshTokenStillWorks(t *testing.T) {
	env := setupVerificationEnv(t)
	user, _ := env.register(t, "anna@example.com")
	env.markVerified(t, user.ID)
	pair, err := env.svc.CreateTokenPair(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("create pair: %v", err)
	}
	if _, err := NewTokenPurgeJob(env.db, zap.NewNop()).Run(context.Background()); err != nil {
		t.Fatalf("run: %v", err)
	}
	if _, err := env.svc.RefreshTokens(context.Background(), pair.RefreshToken); err != nil {
		t.Fatalf("refresh after purge: %v", err)
	}
}
