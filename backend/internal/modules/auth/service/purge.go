package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/modules/auth/domain"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// TaskTypePurgeTokens is the asynq task type of the daily auth token purge
// (empty payload).
const TaskTypePurgeTokens = "auth:purge_tokens"

// PurgeQueue is the dedicated asynq queue of the purge task, served by its
// own asynq.Server (concurrency 1) in cmd/worker.
const PurgeQueue = "auth_maintenance"

// defaultPurgeBatchSize is the number of rows deleted per statement.
const defaultPurgeBatchSize = 1000

// PurgeTaskOptions are the fixed options of the purge task. Unique is only a
// defense in depth against duplicate enqueues (the worker runs one replica).
func PurgeTaskOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(PurgeQueue),
		asynq.MaxRetry(2),
		asynq.Timeout(10 * time.Minute),
		asynq.Unique(6 * time.Hour),
	}
}

// NewPurgeTokensTask builds the purge task.
func NewPurgeTokensTask() *asynq.Task {
	return asynq.NewTask(TaskTypePurgeTokens, nil)
}

// PurgeStats are the numbers of rows deleted per table in one run.
type PurgeStats struct {
	RefreshDeleted      int64
	VerificationDeleted int64
	ResetDeleted        int64
}

// TokenPurgeJob deletes expired/revoked auth token rows in bounded batches.
type TokenPurgeJob struct {
	db        *gorm.DB
	logger    *zap.Logger
	now       func() time.Time
	batchSize int
}

// PurgeOption customises a TokenPurgeJob (used by tests).
type PurgeOption func(*TokenPurgeJob)

// WithPurgeClock sets the clock of the job.
func WithPurgeClock(now func() time.Time) PurgeOption {
	return func(j *TokenPurgeJob) { j.now = now }
}

// WithPurgeBatchSize sets the number of rows deleted per statement.
func WithPurgeBatchSize(n int) PurgeOption {
	return func(j *TokenPurgeJob) {
		if n > 0 {
			j.batchSize = n
		}
	}
}

// NewTokenPurgeJob builds the purge job.
func NewTokenPurgeJob(db *gorm.DB, logger *zap.Logger, opts ...PurgeOption) *TokenPurgeJob {
	if logger == nil {
		logger = zap.NewNop()
	}
	j := &TokenPurgeJob{db: db, logger: logger, now: time.Now, batchSize: defaultPurgeBatchSize}
	for _, o := range opts {
		o(j)
	}
	return j
}

// Run purges all three tables. Each table is independent: a failure on one is
// logged and the others still run; the errors are returned aggregated so
// asynq retries (the purge is idempotent).
func (j *TokenPurgeJob) Run(ctx context.Context) (PurgeStats, error) {
	now := j.now().UTC()
	var stats PurgeStats
	var errs []error

	refreshCutoff := now.Add(-domain.RefreshTokenRetention).Unix()
	oneTimeCutoff := now.Add(-domain.OneTimeTokenRetention)

	// Refresh tokens: two loops so each predicate uses its own index.
	n, err := j.purge(ctx, "refresh_tokens", "expires_at < ?", refreshCutoff)
	stats.RefreshDeleted += n
	if err == nil {
		var m int64
		m, err = j.purge(ctx, "refresh_tokens", "revoked_at IS NOT NULL AND revoked_at < ?", refreshCutoff)
		stats.RefreshDeleted += m
	}
	if err != nil {
		j.logger.Warn("auth token purge failed", zap.String("table", "refresh_tokens"), zap.Error(err))
		errs = append(errs, fmt.Errorf("refresh_tokens: %w", err))
	}

	stats.VerificationDeleted, err = j.purge(ctx, "email_verification_tokens", "expires_at < ?", oneTimeCutoff)
	if err != nil {
		j.logger.Warn("auth token purge failed", zap.String("table", "email_verification_tokens"), zap.Error(err))
		errs = append(errs, fmt.Errorf("email_verification_tokens: %w", err))
	}

	stats.ResetDeleted, err = j.purge(ctx, "password_reset_tokens", "expires_at < ?", oneTimeCutoff)
	if err != nil {
		j.logger.Warn("auth token purge failed", zap.String("table", "password_reset_tokens"), zap.Error(err))
		errs = append(errs, fmt.Errorf("password_reset_tokens: %w", err))
	}

	// Logged even when zero: the daily health signal. No hashes, emails or ids.
	j.logger.Info("auth tokens purged",
		zap.Int64("refresh_deleted", stats.RefreshDeleted),
		zap.Int64("verification_deleted", stats.VerificationDeleted),
		zap.Int64("reset_deleted", stats.ResetDeleted))
	return stats, errors.Join(errs...)
}

// purge deletes rows of table matching predicate by primary key in batches
// until a batch deletes fewer rows than the batch size. table and predicate
// are constants of this file, never user input.
func (j *TokenPurgeJob) purge(ctx context.Context, table, predicate string, arg any) (int64, error) {
	query := fmt.Sprintf("DELETE FROM %s WHERE id IN (SELECT id FROM %s WHERE %s LIMIT ?)", table, table, predicate)
	var total int64
	for {
		if err := ctx.Err(); err != nil {
			return total, err
		}
		res := j.db.WithContext(ctx).Exec(query, arg, j.batchSize)
		if res.Error != nil {
			return total, res.Error
		}
		total += res.RowsAffected
		if res.RowsAffected < int64(j.batchSize) {
			return total, nil
		}
	}
}

// HandleTask runs the purge. The error is returned so asynq retries it.
func (j *TokenPurgeJob) HandleTask(ctx context.Context, _ *asynq.Task) error {
	_, err := j.Run(ctx)
	return err
}
