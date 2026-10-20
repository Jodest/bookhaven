package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/bookhaven/backend/internal/modules/screenworks/domain"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// TaskTypeCatalogSync is the asynq task type of the catalog-wide adaptation
// sync job. It runs on the harvest queue (the caller passes its name), so it
// shares the single harvest worker slot and shows up in the admin jobs list.
const TaskTypeCatalogSync = "screenworks:sync_catalog"

// TaskTypeCatalogSyncTick is the tick the worker's scheduler enqueues on the
// scheduled catalog sync cron.
const TaskTypeCatalogSyncTick = "screenworks:sync_catalog_tick"

// ScheduleQueueName is the queue the tick runs on (same queue as the
// scheduled harvest tick; defined here so screenworks does not import harvest).
const ScheduleQueueName = "harvest_schedule"

// Catalog sync triggers (CatalogSyncPayload.Trigger).
const (
	TriggerManual    = "manual"
	TriggerScheduled = "scheduled"
)

const (
	// catalogSyncErrorCap bounds CatalogSyncResult.Errors; Failed is uncapped.
	catalogSyncErrorCap = 50
	// catalogSyncMaxFailedChunks is the number of consecutive chunks whose
	// Wikidata call may fail before the attempt fails (and is retried).
	catalogSyncMaxFailedChunks = 3
	// catalogSyncListPageSize is the inspector page size of CatalogSyncInFlight.
	catalogSyncListPageSize = 100
)

// Reasons of CatalogSyncError.
const (
	ReasonWikidataUnavailable = "wikidata_unavailable"
	ReasonSaveFailed          = "save_failed"
)

// CatalogSyncPayload is the JSON payload of TaskTypeCatalogSync. The trigger
// key matches the harvest payload so the jobs list decodes it unchanged.
type CatalogSyncPayload struct {
	Trigger string `json:"trigger"`
}

// CatalogSyncError is one failed book of a catalog sync.
type CatalogSyncError struct {
	BookID string `json:"book_id"`
	Title  string `json:"title"`
	Reason string `json:"reason"`
}

// CatalogSyncResult is the JSON result of a catalog sync job, also written
// after every chunk as progress.
type CatalogSyncResult struct {
	Total     int                `json:"total"`
	Processed int                `json:"processed"`
	Updated   int                `json:"updated"`
	Added     int                `json:"added"`
	Unchanged int                `json:"unchanged"`
	Skipped   int                `json:"skipped"`
	Failed    int                `json:"failed"`
	Errors    []CatalogSyncError `json:"errors"`
}

// NewCatalogSyncTask builds the catalog sync task. Pass CatalogSyncOptions to
// Enqueue.
func NewCatalogSyncTask(trigger string) (*asynq.Task, error) {
	payload, err := json.Marshal(CatalogSyncPayload{Trigger: trigger})
	if err != nil {
		return nil, fmt.Errorf("catalog adaptation sync: marshal payload: %w", err)
	}
	return asynq.NewTask(TaskTypeCatalogSync, payload), nil
}

// CatalogSyncOptions are the fixed options of a catalog sync job on queue.
func CatalogSyncOptions(queue string) []asynq.Option {
	return []asynq.Option{
		asynq.Queue(queue),
		asynq.MaxRetry(3),
		asynq.Timeout(30 * time.Minute),
		asynq.Retention(24 * time.Hour),
	}
}

// CatalogSyncTickOptions are the fixed options of the tick: no retries (the
// next scheduled run tries again), a short timeout and Unique as a defense in
// depth against duplicate enqueues.
func CatalogSyncTickOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(ScheduleQueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(time.Minute),
		asynq.Unique(time.Hour),
	}
}

// NewCatalogSyncTickTask builds the (payload-less) tick task.
func NewCatalogSyncTickTask() *asynq.Task {
	return asynq.NewTask(TaskTypeCatalogSyncTick, nil)
}

// resultWriter is the subset of *asynq.ResultWriter the job writes to.
type resultWriter interface {
	Write(data []byte) (int, error)
}

// CatalogSyncJob handles TaskTypeCatalogSync.
type CatalogSyncJob struct {
	svc *Service
	log *zap.Logger
	// writer overrides the task's ResultWriter (tests; asynq.NewTask tasks
	// have no usable one).
	writer resultWriter
}

// NewCatalogSyncJob builds the job around the screen works service.
func NewCatalogSyncJob(svc *Service, log *zap.Logger) *CatalogSyncJob {
	if log == nil {
		log = zap.NewNop()
	}
	return &CatalogSyncJob{svc: svc, log: log}
}

// HandleTask is the asynq handler. It never publishes or changes books; it
// only adds adaptation links under the rules of Service.syncBooks.
func (j *CatalogSyncJob) HandleTask(ctx context.Context, t *asynq.Task) error {
	var w resultWriter = j.writer
	if w == nil {
		if rw := t.ResultWriter(); rw != nil {
			w = rw
		}
	}
	return j.run(ctx, w)
}

func (j *CatalogSyncJob) run(ctx context.Context, w resultWriter) error {
	if j.svc.source == nil {
		return fmt.Errorf("catalog adaptation sync: %w: %w", ErrSyncUnavailable, asynq.SkipRetry)
	}
	ids, err := j.svc.repo.ListCatalogSyncBookIDs(ctx)
	if err != nil {
		return fmt.Errorf("catalog adaptation sync: list books: %w", err)
	}

	res := CatalogSyncResult{Total: len(ids), Errors: []CatalogSyncError{}}
	j.write(w, res)

	consecutiveFailed := 0
	for start := 0; start < len(ids); start += SyncCap {
		if err := ctx.Err(); err != nil {
			return err
		}
		chunk := ids[start:min(start+SyncCap, len(ids))]
		report, err := j.svc.syncBooks(ctx, chunk)
		if report == nil {
			// The books could not even be loaded.
			consecutiveFailed++
			res.Processed += len(chunk)
			res.Failed += len(chunk)
			j.log.Warn("catalog adaptation sync: chunk failed", zap.Error(err))
			for _, id := range chunk {
				res.addError(CatalogSyncError{BookID: id.String(), Reason: ReasonSaveFailed})
			}
		} else {
			wikidataDown := err != nil
			if wikidataDown {
				consecutiveFailed++
			} else {
				consecutiveFailed = 0
			}
			res.add(report)
		}
		j.write(w, res)
		if consecutiveFailed >= catalogSyncMaxFailedChunks {
			return fmt.Errorf("catalog adaptation sync: wikidata failed for %d chunks in a row: %w", consecutiveFailed, err)
		}
	}

	j.write(w, res)
	j.log.Info("catalog adaptation sync finished",
		zap.Int("total", res.Total), zap.Int("added", res.Added), zap.Int("failed", res.Failed))
	return nil
}

func (r *CatalogSyncResult) addError(e CatalogSyncError) {
	if len(r.Errors) < catalogSyncErrorCap {
		r.Errors = append(r.Errors, e)
	}
}

// add folds one chunk report into the totals.
func (r *CatalogSyncResult) add(report *SyncReport) {
	for _, item := range report.Items {
		r.Processed++
		switch item.Outcome {
		case domain.OutcomeAdded:
			r.Updated++
		case domain.OutcomeUnchanged:
			r.Unchanged++
		case domain.OutcomeSkipped:
			r.Skipped++
		case domain.OutcomeError:
			if item.Message == domain.ErrBookNotFound.Error() {
				// Deleted mid-run.
				r.Skipped++
				continue
			}
			r.Failed++
			reason := ReasonSaveFailed
			for _, n := range item.Notes {
				if n == domain.NoteWikidataUnavailable {
					reason = ReasonWikidataUnavailable
				}
			}
			r.addError(CatalogSyncError{BookID: item.BookID, Title: item.Title, Reason: reason})
		}
	}
	r.Added += report.Added
}

// write stores the result as job progress; failures are only logged.
func (j *CatalogSyncJob) write(w resultWriter, res CatalogSyncResult) {
	if w == nil {
		return
	}
	data, err := json.Marshal(res)
	if err != nil {
		j.log.Warn("catalog adaptation sync: marshal result", zap.Error(err))
		return
	}
	if _, err := w.Write(data); err != nil {
		j.log.Warn("catalog adaptation sync: write progress", zap.Error(err))
	}
}

// TickInspector is the subset of *asynq.Inspector the in-flight check and
// the tick depend on.
type TickInspector interface {
	ListPendingTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListActiveTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListRetryTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
}

// TickEnqueuer is the subset of *asynq.Client the tick depends on.
type TickEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// CatalogSyncInFlight reports whether a catalog sync task is pending, active
// or retrying on queue. A queue that does not exist yet holds no tasks.
func CatalogSyncInFlight(inspector TickInspector, queue string) (bool, error) {
	listers := []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error){
		inspector.ListActiveTasks,
		inspector.ListRetryTasks,
		inspector.ListPendingTasks,
	}
	for _, list := range listers {
		for page := 1; ; page++ {
			infos, err := list(queue, asynq.PageSize(catalogSyncListPageSize), asynq.Page(page))
			if err != nil {
				if errors.Is(err, asynq.ErrQueueNotFound) {
					break
				}
				return false, err
			}
			for _, info := range infos {
				if info.Type == TaskTypeCatalogSync {
					return true, nil
				}
			}
			if len(infos) < catalogSyncListPageSize {
				break
			}
		}
	}
	return false, nil
}

// CatalogSyncTick handles TaskTypeCatalogSyncTick: it queues one scheduled
// catalog sync job unless one is already queued or running.
type CatalogSyncTick struct {
	enqueuer  TickEnqueuer
	inspector TickInspector
	queue     string
	log       *zap.Logger
}

// NewCatalogSyncTick builds the tick; queue is the harvest queue name.
func NewCatalogSyncTick(enqueuer TickEnqueuer, inspector TickInspector, queue string, log *zap.Logger) *CatalogSyncTick {
	if log == nil {
		log = zap.NewNop()
	}
	return &CatalogSyncTick{enqueuer: enqueuer, inspector: inspector, queue: queue, log: log}
}

// HandleTask runs one tick. Inspector and enqueue failures are returned; the
// tick has no retries, so the next scheduled run tries again.
func (t *CatalogSyncTick) HandleTask(_ context.Context, _ *asynq.Task) error {
	running, err := CatalogSyncInFlight(t.inspector, t.queue)
	if err != nil {
		return fmt.Errorf("catalog adaptation sync schedule: inspect queue: %w", err)
	}
	if running {
		t.log.Info("scheduled adaptation sync skipped: a catalog sync job is still queued or running")
		return nil
	}
	task, err := NewCatalogSyncTask(TriggerScheduled)
	if err != nil {
		return err
	}
	info, err := t.enqueuer.Enqueue(task, CatalogSyncOptions(t.queue)...)
	if err != nil {
		return fmt.Errorf("catalog adaptation sync schedule: enqueue: %w", err)
	}
	t.log.Info("scheduled adaptation sync job queued", zap.String("task_id", info.ID))
	return nil
}
