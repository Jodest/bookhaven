package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"go.uber.org/zap"
)

// TaskTypeHarvestScheduleTick is the asynq task type the worker's scheduler
// enqueues on the scheduled-harvest cron. Its handler (ScheduleTick) picks a
// batch of flagged authors and queues one ordinary harvest job for them.
const TaskTypeHarvestScheduleTick = "harvest:schedule_tick"

// ScheduleQueueName is the queue the tick runs on. It is served by its own
// small asynq.Server (concurrency 1), not by the harvest server, so ticks
// never show up in the admin jobs list and never wait behind a harvest job.
const ScheduleQueueName = "harvest_schedule"

// scheduleListPageSize is the inspector page size used while looking for a
// scheduled job that is still pending, active or retrying.
const scheduleListPageSize = 100

// NewScheduleTickTask builds the (payload-less) schedule tick task.
func NewScheduleTickTask() *asynq.Task {
	return asynq.NewTask(TaskTypeHarvestScheduleTick, nil)
}

// ScheduleTickOptions are the fixed options of the tick: no retries (the
// next scheduled run tries again), a short timeout, and Unique as a
// defense in depth against duplicate enqueues (e.g. two schedulers).
func ScheduleTickOptions() []asynq.Option {
	return []asynq.Option{
		asynq.Queue(ScheduleQueueName),
		asynq.MaxRetry(0),
		asynq.Timeout(time.Minute),
		asynq.Unique(time.Hour),
	}
}

// ScheduleEnqueuer is the subset of *asynq.Client the tick depends on.
type ScheduleEnqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// ScheduleInspector is the subset of *asynq.Inspector the tick depends on.
type ScheduleInspector interface {
	ListPendingTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListActiveTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
	ListRetryTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error)
}

// AutoHarvestAuthorSource is the subset of *adminservice.Service the tick
// depends on (picking candidates and stamping them).
type AutoHarvestAuthorSource interface {
	ListAutoHarvestCandidates(ctx context.Context, limit int, now time.Time) ([]adminservice.AutoHarvestCandidate, error)
	MarkAuthorsAutoHarvested(ctx context.Context, ids []uuid.UUID, at time.Time) error
}

// ScheduleTick handles TaskTypeHarvestScheduleTick. It only queues a
// harvest job through the normal pipeline: everything that job creates is
// pending and goes through moderation; nothing is published here.
type ScheduleTick struct {
	enqueuer  ScheduleEnqueuer
	inspector ScheduleInspector
	authors   AutoHarvestAuthorSource
	batch     int
	log       *zap.Logger
	now       func() time.Time
}

// NewScheduleTick builds a ScheduleTick that picks at most batch authors
// per run. A nil logger is replaced by a no-op logger.
func NewScheduleTick(enqueuer ScheduleEnqueuer, inspector ScheduleInspector, authors AutoHarvestAuthorSource, batch int, log *zap.Logger) *ScheduleTick {
	if log == nil {
		log = zap.NewNop()
	}
	return &ScheduleTick{
		enqueuer:  enqueuer,
		inspector: inspector,
		authors:   authors,
		batch:     batch,
		log:       log,
		now:       time.Now,
	}
}

// HandleTask runs one scheduled-harvest tick:
//  1. skip if a scheduled harvest job is still pending, active or retrying;
//  2. pick up to batch eligible authors (none → nothing to do);
//  3. queue one harvest job for their QIDs (trigger "scheduled");
//  4. only then stamp the authors' last auto-harvest time (a stamp failure
//     is logged, not returned: the queued job blocks the next run anyway
//     and re-harvesting is idempotent).
//
// An inspector, candidate or enqueue failure is returned; the tick has no
// retries, so the next scheduled run tries again.
func (t *ScheduleTick) HandleTask(ctx context.Context, _ *asynq.Task) error {
	running, err := t.scheduledJobInFlight()
	if err != nil {
		return fmt.Errorf("harvest schedule: inspect harvest queue: %w", err)
	}
	if running {
		t.log.Info("scheduled harvest skipped: previous scheduled job still queued or running")
		return nil
	}

	now := t.now()
	candidates, err := t.authors.ListAutoHarvestCandidates(ctx, t.batch, now)
	if err != nil {
		return fmt.Errorf("harvest schedule: %w", err)
	}
	if len(candidates) == 0 {
		t.log.Info("scheduled harvest: no eligible authors")
		return nil
	}

	qids := make([]string, 0, len(candidates))
	ids := make([]uuid.UUID, 0, len(candidates))
	for _, c := range candidates {
		qids = append(qids, c.QID)
		ids = append(ids, c.ID)
	}

	query, err := bookdata.BuildAuthorListQuery(qids, 0)
	if err != nil {
		return fmt.Errorf("harvest schedule: build query: %w", err)
	}
	task, err := NewHarvestTask(query, HarvestMeta{Trigger: TriggerScheduled, AuthorQIDs: qids})
	if err != nil {
		return fmt.Errorf("harvest schedule: %w", err)
	}
	info, err := t.enqueuer.Enqueue(task)
	if err != nil {
		return fmt.Errorf("harvest schedule: enqueue harvest job: %w", err)
	}

	if err := t.authors.MarkAuthorsAutoHarvested(ctx, ids, now); err != nil {
		t.log.Error("scheduled harvest: failed to stamp authors", zap.Error(err), zap.Strings("author_qids", qids))
		return nil
	}
	t.log.Info("scheduled harvest job queued", zap.String("task_id", info.ID), zap.Strings("author_qids", qids))
	return nil
}

// scheduledJobInFlight reports whether any pending, active or retrying task
// on the harvest queue is a scheduled harvest. A harvest queue that does
// not exist yet (never used) holds no tasks.
func (t *ScheduleTick) scheduledJobInFlight() (bool, error) {
	listers := []func(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error){
		t.inspector.ListActiveTasks,
		t.inspector.ListRetryTasks,
		t.inspector.ListPendingTasks,
	}
	for _, list := range listers {
		for page := 1; ; page++ {
			infos, err := list(QueueName, asynq.PageSize(scheduleListPageSize), asynq.Page(page))
			if err != nil {
				if errors.Is(err, asynq.ErrQueueNotFound) {
					break
				}
				return false, err
			}
			for _, info := range infos {
				if info.Type == TaskTypeHarvestWikidataQuery && PayloadTrigger(info.Payload) == TriggerScheduled {
					return true, nil
				}
			}
			if len(infos) < scheduleListPageSize {
				break
			}
		}
	}
	return false, nil
}

// PayloadTrigger returns the trigger recorded in a harvest task payload:
// TriggerScheduled, or TriggerManual for anything else (absent field,
// unknown value, undecodable payload).
func PayloadTrigger(payload []byte) string {
	trigger, _ := PayloadMeta(payload)
	return trigger
}

// PayloadMeta decodes the job metadata of a harvest task payload: the
// trigger (see PayloadTrigger) and the author QIDs (never nil).
func PayloadMeta(payload []byte) (trigger string, authorQIDs []string) {
	var p HarvestPayload
	if len(payload) > 0 {
		_ = json.Unmarshal(payload, &p)
	}
	trigger = TriggerManual
	if p.Trigger == TriggerScheduled {
		trigger = TriggerScheduled
	}
	authorQIDs = p.AuthorQIDs
	if authorQIDs == nil {
		authorQIDs = []string{}
	}
	return trigger, authorQIDs
}
