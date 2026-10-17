package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/modules/aigenres/domain"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// Task types and queue. The queue is served by its own asynq.Server with
// Concurrency 1, so all model calls are serialized.
const (
	TaskTypeAnalyze   = "ai_genres:analyze"
	TaskTypeScheduled = "ai_genres:scheduled"
	QueueName         = "ai_genres"
)

// AnalyzePayload is the payload of TaskTypeAnalyze.
type AnalyzePayload struct {
	RunID string `json:"run_id"`
}

// NewAnalyzeTask builds the task that computes the queued run.
func NewAnalyzeTask(runID uuid.UUID) (*asynq.Task, error) {
	payload, err := json.Marshal(AnalyzePayload{RunID: runID.String()})
	if err != nil {
		return nil, fmt.Errorf("aigenres: marshal analyze payload: %w", err)
	}
	return asynq.NewTask(TaskTypeAnalyze, payload), nil
}

// NewScheduledTask builds the periodic task (no payload).
func NewScheduledTask() *asynq.Task {
	return asynq.NewTask(TaskTypeScheduled, nil)
}

// AnalyzeOptions: no retries (the next tick or a manual run continues), a 2 h
// timeout and Unique(2h) as a defense in depth (single worker replica); the
// real "one run at a time" rule is the run table.
func AnalyzeOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(2 * time.Hour),
		asynq.Unique(2 * time.Hour),
	}
}

// ScheduledOptions: like AnalyzeOptions, Unique(1h).
func ScheduledOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(QueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(2 * time.Hour),
		asynq.Unique(time.Hour),
	}
}

// Handlers runs the AI genre tasks.
type Handlers struct {
	analyzer *Analyzer
	repo     Repository
	catalog  GenreCatalog
	model    string
	log      *zap.Logger
	now      func() time.Time
}

// NewHandlers builds the task handlers. A nil logger is replaced by a no-op
// logger.
func NewHandlers(analyzer *Analyzer, repo Repository, catalog GenreCatalog, model string, log *zap.Logger) *Handlers {
	if log == nil {
		log = zap.NewNop()
	}
	return &Handlers{analyzer: analyzer, repo: repo, catalog: catalog, model: model, log: log, now: time.Now}
}

// HandleAnalyze computes the run named in the payload (a manual run created
// by the API).
func (h *Handlers) HandleAnalyze(ctx context.Context, t *asynq.Task) error {
	var p AnalyzePayload
	if err := json.Unmarshal(t.Payload(), &p); err != nil {
		return fmt.Errorf("aigenres: decode analyze payload: %v: %w", err, asynq.SkipRetry)
	}
	id, err := uuid.Parse(p.RunID)
	if err != nil {
		return fmt.Errorf("aigenres: bad run id: %v: %w", err, asynq.SkipRetry)
	}
	return h.analyzer.Run(ctx, id)
}

// HandleScheduled starts a scheduled run unless one is active or nothing
// changed since the last clean run: it compares the fingerprint of the
// candidate genres (and the prompt version) with the one of that run, and
// makes no model call (and creates no run) when they are equal.
func (h *Handlers) HandleScheduled(ctx context.Context, _ *asynq.Task) error {
	now := h.now().UTC()
	if _, err := h.repo.InterruptStaleRuns(ctx, now.Add(-domain.ActiveStaleAfter), now); err != nil {
		return fmt.Errorf("aigenres scheduled: %w", err)
	}
	active, err := h.repo.ActiveRun(ctx)
	if err != nil {
		return fmt.Errorf("aigenres scheduled: %w", err)
	}
	if active != nil {
		h.log.Info("AI genres scheduled run skipped: a run is active")
		return nil
	}

	cands, err := h.catalog.ListGenreAnalysisCandidates(ctx)
	if err != nil {
		return fmt.Errorf("aigenres scheduled: %w", err)
	}
	last, err := h.repo.LastCleanFingerprint(ctx)
	if err != nil {
		return fmt.Errorf("aigenres scheduled: %w", err)
	}
	if last != "" && last == Fingerprint(cands) {
		h.log.Info("AI genres scheduled run skipped: the genres are unchanged")
		return nil
	}

	run := &domain.Run{
		Trigger: domain.TriggerScheduled, Status: domain.RunQueued, Model: h.model,
		PromptVersion: PromptVersion, CreatedAt: now,
	}
	if err := h.repo.CreateRun(ctx, run); err != nil {
		return fmt.Errorf("aigenres scheduled: %w", err)
	}
	return h.analyzer.Run(ctx, run.ID)
}
