package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	adminservice "github.com/bookhaven/backend/internal/modules/admin/service"
	catalogDomain "github.com/bookhaven/backend/internal/modules/catalog/domain"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

// fakeAdminRepository stubs for the scheduled-harvest repository methods
// (unused by the harvest handler tests).
func (f *fakeAdminRepository) ListAutoHarvestCandidates(ctx context.Context, limit int, notAfter time.Time) ([]catalogDomain.Author, error) {
	return nil, nil
}

func (f *fakeAdminRepository) MarkAuthorsAutoHarvested(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	return nil
}

type fakeScheduleEnqueuer struct {
	tasks []*asynq.Task
	err   error
}

func (f *fakeScheduleEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{ID: fmt.Sprintf("task-%d", len(f.tasks)), Queue: QueueName}, nil
}

type fakeScheduleInspector struct {
	pending, active, retry []*asynq.TaskInfo
	err                    error
	missingQueue           bool
}

func (f *fakeScheduleInspector) page(infos []*asynq.TaskInfo, opts []asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.missingQueue {
		return nil, fmt.Errorf("asynq: %w", asynq.ErrQueueNotFound)
	}
	size, page := 30, 1
	for _, o := range opts {
		// asynq's list options are unexported int types (pageSizeOpt,
		// pageNumOpt); identify them by reflection.
		v := reflect.ValueOf(o)
		switch v.Type().Name() {
		case "pageSizeOpt":
			size = int(v.Int())
		case "pageNumOpt":
			page = int(v.Int())
		}
	}
	start := (page - 1) * size
	if start >= len(infos) {
		return nil, nil
	}
	end := start + size
	if end > len(infos) {
		end = len(infos)
	}
	return infos[start:end], nil
}

func (f *fakeScheduleInspector) ListPendingTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return f.page(f.pending, opts)
}

func (f *fakeScheduleInspector) ListActiveTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return f.page(f.active, opts)
}

func (f *fakeScheduleInspector) ListRetryTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return f.page(f.retry, opts)
}

type fakeAuthorSource struct {
	candidates []adminservice.AutoHarvestCandidate
	listErr    error
	markErr    error
	lastLimit  int
	lastNow    time.Time
	marked     []uuid.UUID
	markedAt   time.Time
	markCalls  int
}

func (f *fakeAuthorSource) ListAutoHarvestCandidates(ctx context.Context, limit int, now time.Time) ([]adminservice.AutoHarvestCandidate, error) {
	f.lastLimit, f.lastNow = limit, now
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.candidates, nil
}

func (f *fakeAuthorSource) MarkAuthorsAutoHarvested(ctx context.Context, ids []uuid.UUID, at time.Time) error {
	f.markCalls++
	if f.markErr != nil {
		return f.markErr
	}
	f.marked, f.markedAt = ids, at
	return nil
}

func harvestInfo(t *testing.T, meta *HarvestMeta) *asynq.TaskInfo {
	t.Helper()
	p := HarvestPayload{SPARQLQuery: "SELECT"}
	if meta != nil {
		p.Trigger, p.AuthorQIDs = meta.Trigger, meta.AuthorQIDs
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return &asynq.TaskInfo{ID: uuid.NewString(), Queue: QueueName, Type: TaskTypeHarvestWikidataQuery, Payload: raw}
}

var tickNow = time.Date(2026, 10, 13, 4, 0, 0, 0, time.UTC)

func newTestTick(enq *fakeScheduleEnqueuer, insp *fakeScheduleInspector, src *fakeAuthorSource, batch int) *ScheduleTick {
	tick := NewScheduleTick(enq, insp, src, batch, nil)
	tick.now = func() time.Time { return tickNow }
	return tick
}

func TestScheduleTick_SkipsWhileScheduledJobInFlight(t *testing.T) {
	scheduled := &HarvestMeta{Trigger: TriggerScheduled, AuthorQIDs: []string{"Q1"}}
	for _, state := range []string{"pending", "active", "retry"} {
		t.Run(state, func(t *testing.T) {
			insp := &fakeScheduleInspector{}
			info := harvestInfo(t, scheduled)
			switch state {
			case "pending":
				insp.pending = []*asynq.TaskInfo{harvestInfo(t, nil), info}
			case "active":
				insp.active = []*asynq.TaskInfo{info}
			case "retry":
				insp.retry = []*asynq.TaskInfo{info}
			}
			enq := &fakeScheduleEnqueuer{}
			src := &fakeAuthorSource{candidates: []adminservice.AutoHarvestCandidate{{ID: uuid.New(), QID: "Q7200"}}}

			if err := newTestTick(enq, insp, src, 10).HandleTask(context.Background(), NewScheduleTickTask()); err != nil {
				t.Fatalf("HandleTask: %v", err)
			}
			if len(enq.tasks) != 0 || src.markCalls != 0 || src.lastLimit != 0 {
				t.Fatalf("expected a skip, got %d enqueues, %d marks, list limit %d", len(enq.tasks), src.markCalls, src.lastLimit)
			}
		})
	}
}

func TestScheduleTick_FindsScheduledJobBeyondFirstPage(t *testing.T) {
	insp := &fakeScheduleInspector{}
	for i := 0; i < scheduleListPageSize; i++ {
		insp.pending = append(insp.pending, harvestInfo(t, &HarvestMeta{Trigger: TriggerManual}))
	}
	insp.pending = append(insp.pending, harvestInfo(t, &HarvestMeta{Trigger: TriggerScheduled}))
	enq := &fakeScheduleEnqueuer{}
	src := &fakeAuthorSource{candidates: []adminservice.AutoHarvestCandidate{{ID: uuid.New(), QID: "Q7200"}}}

	if err := newTestTick(enq, insp, src, 10).HandleTask(context.Background(), NewScheduleTickTask()); err != nil {
		t.Fatalf("HandleTask: %v", err)
	}
	if len(enq.tasks) != 0 {
		t.Fatalf("expected a skip, got %d enqueues", len(enq.tasks))
	}
}

func TestScheduleTick_ManualJobsDoNotBlock(t *testing.T) {
	insp := &fakeScheduleInspector{
		active:  []*asynq.TaskInfo{harvestInfo(t, &HarvestMeta{Trigger: TriggerManual, AuthorQIDs: []string{"Q1"}})},
		pending: []*asynq.TaskInfo{harvestInfo(t, nil)},
	}
	enq := &fakeScheduleEnqueuer{}
	src := &fakeAuthorSource{candidates: []adminservice.AutoHarvestCandidate{{ID: uuid.New(), QID: "Q7200"}}}

	if err := newTestTick(enq, insp, src, 10).HandleTask(context.Background(), NewScheduleTickTask()); err != nil {
		t.Fatalf("HandleTask: %v", err)
	}
	if len(enq.tasks) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(enq.tasks))
	}
}

func TestScheduleTick_NoCandidates(t *testing.T) {
	enq := &fakeScheduleEnqueuer{}
	src := &fakeAuthorSource{}
	if err := newTestTick(enq, &fakeScheduleInspector{missingQueue: true}, src, 7).HandleTask(context.Background(), NewScheduleTickTask()); err != nil {
		t.Fatalf("HandleTask: %v", err)
	}
	if src.lastLimit != 7 {
		t.Errorf("limit = %d, want 7", src.lastLimit)
	}
	if len(enq.tasks) != 0 || src.markCalls != 0 {
		t.Fatalf("expected no enqueue and no stamp, got %d / %d", len(enq.tasks), src.markCalls)
	}
}

func TestScheduleTick_EnqueuesAndStamps(t *testing.T) {
	id1, id2 := uuid.New(), uuid.New()
	enq := &fakeScheduleEnqueuer{}
	src := &fakeAuthorSource{candidates: []adminservice.AutoHarvestCandidate{{ID: id1, QID: "Q7200"}, {ID: id2, QID: "Q1183296"}}}

	if err := newTestTick(enq, &fakeScheduleInspector{}, src, 10).HandleTask(context.Background(), NewScheduleTickTask()); err != nil {
		t.Fatalf("HandleTask: %v", err)
	}
	if src.lastLimit != 10 || !src.lastNow.Equal(tickNow) {
		t.Errorf("list args = %d, %v", src.lastLimit, src.lastNow)
	}
	if len(enq.tasks) != 1 {
		t.Fatalf("expected 1 enqueue, got %d", len(enq.tasks))
	}
	task := enq.tasks[0]
	if task.Type() != TaskTypeHarvestWikidataQuery {
		t.Errorf("task type = %q", task.Type())
	}
	var p HarvestPayload
	if err := json.Unmarshal(task.Payload(), &p); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if p.Trigger != TriggerScheduled || !reflect.DeepEqual(p.AuthorQIDs, []string{"Q7200", "Q1183296"}) {
		t.Errorf("payload meta = %q %v", p.Trigger, p.AuthorQIDs)
	}
	if !strings.Contains(p.SPARQLQuery, "wd:Q7200 wd:Q1183296") || !strings.Contains(p.SPARQLQuery, "LIMIT 500") {
		t.Errorf("unexpected query: %s", p.SPARQLQuery)
	}
	if !reflect.DeepEqual(src.marked, []uuid.UUID{id1, id2}) || !src.markedAt.Equal(tickNow) {
		t.Errorf("stamped %v at %v", src.marked, src.markedAt)
	}
}

func TestScheduleTick_EnqueueFailureDoesNotStamp(t *testing.T) {
	enq := &fakeScheduleEnqueuer{err: errors.New("redis down")}
	src := &fakeAuthorSource{candidates: []adminservice.AutoHarvestCandidate{{ID: uuid.New(), QID: "Q7200"}}}

	if err := newTestTick(enq, &fakeScheduleInspector{}, src, 10).HandleTask(context.Background(), NewScheduleTickTask()); err == nil {
		t.Fatal("expected the enqueue error")
	}
	if src.markCalls != 0 {
		t.Fatalf("authors must not be stamped after a failed enqueue")
	}
}

func TestScheduleTick_StampFailureIsNotAnError(t *testing.T) {
	enq := &fakeScheduleEnqueuer{}
	src := &fakeAuthorSource{
		candidates: []adminservice.AutoHarvestCandidate{{ID: uuid.New(), QID: "Q7200"}},
		markErr:    errors.New("db down"),
	}
	if err := newTestTick(enq, &fakeScheduleInspector{}, src, 10).HandleTask(context.Background(), NewScheduleTickTask()); err != nil {
		t.Fatalf("a stamp failure must not fail the tick, got %v", err)
	}
	if len(enq.tasks) != 1 || src.markCalls != 1 {
		t.Fatalf("enqueues=%d marks=%d", len(enq.tasks), src.markCalls)
	}
}

func TestScheduleTick_InspectorAndListErrors(t *testing.T) {
	src := &fakeAuthorSource{}
	if err := newTestTick(&fakeScheduleEnqueuer{}, &fakeScheduleInspector{err: errors.New("redis down")}, src, 10).HandleTask(context.Background(), nil); err == nil {
		t.Fatal("expected the inspector error")
	}
	src = &fakeAuthorSource{listErr: errors.New("db down")}
	if err := newTestTick(&fakeScheduleEnqueuer{}, &fakeScheduleInspector{}, src, 10).HandleTask(context.Background(), nil); err == nil {
		t.Fatal("expected the candidate error")
	}
}

func TestScheduleTickOptions(t *testing.T) {
	var queue string
	var maxRetry = -1
	var timeout, unique time.Duration
	for _, o := range ScheduleTickOptions() {
		switch o.Type() {
		case asynq.QueueOpt:
			queue = o.Value().(string)
		case asynq.MaxRetryOpt:
			maxRetry = o.Value().(int)
		case asynq.TimeoutOpt:
			timeout = o.Value().(time.Duration)
		case asynq.UniqueOpt:
			unique = o.Value().(time.Duration)
		}
	}
	if queue != "harvest_schedule" || maxRetry != 0 || timeout != time.Minute || unique != time.Hour {
		t.Fatalf("options: queue=%q maxRetry=%d timeout=%v unique=%v", queue, maxRetry, timeout, unique)
	}
	if NewScheduleTickTask().Type() != "harvest:schedule_tick" {
		t.Fatalf("tick type = %q", NewScheduleTickTask().Type())
	}
}

func TestNewHarvestTask_MetaRoundTripAndOptions(t *testing.T) {
	cases := []struct {
		name        string
		meta        []HarvestMeta
		wantTrigger string
		wantQIDs    []string
		rawTrigger  bool
	}{
		{name: "no meta (cmd/harvest)", wantTrigger: TriggerManual, wantQIDs: []string{}},
		{name: "manual", meta: []HarvestMeta{{Trigger: TriggerManual, AuthorQIDs: []string{"Q7200"}}}, wantTrigger: TriggerManual, wantQIDs: []string{"Q7200"}, rawTrigger: true},
		{name: "scheduled", meta: []HarvestMeta{{Trigger: TriggerScheduled, AuthorQIDs: []string{"Q7200", "Q1"}}}, wantTrigger: TriggerScheduled, wantQIDs: []string{"Q7200", "Q1"}, rawTrigger: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			task, err := NewHarvestTask("SELECT ?item WHERE {}", tc.meta...)
			if err != nil {
				t.Fatalf("NewHarvestTask: %v", err)
			}
			var p HarvestPayload
			if err := json.Unmarshal(task.Payload(), &p); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if p.SPARQLQuery != "SELECT ?item WHERE {}" {
				t.Errorf("query = %q", p.SPARQLQuery)
			}
			if !tc.rawTrigger && strings.Contains(string(task.Payload()), "trigger") {
				t.Errorf("payload without meta must carry the query only: %s", task.Payload())
			}
			trigger, qids := PayloadMeta(task.Payload())
			if trigger != tc.wantTrigger || !reflect.DeepEqual(qids, tc.wantQIDs) {
				t.Errorf("meta = %q %v, want %q %v", trigger, qids, tc.wantTrigger, tc.wantQIDs)
			}

			var queue string
			var maxRetry int
			var timeout, retention time.Duration
			for _, o := range HarvestTaskOptions() {
				switch o.Type() {
				case asynq.QueueOpt:
					queue = o.Value().(string)
				case asynq.MaxRetryOpt:
					maxRetry = o.Value().(int)
				case asynq.TimeoutOpt:
					timeout = o.Value().(time.Duration)
				case asynq.RetentionOpt:
					retention = o.Value().(time.Duration)
				}
			}
			if queue != QueueName || maxRetry != 3 || timeout != 30*time.Minute || retention != 24*time.Hour {
				t.Errorf("options: queue=%q maxRetry=%d timeout=%v retention=%v", queue, maxRetry, timeout, retention)
			}
		})
	}
}

func TestPayloadMeta_LegacyAndUnknown(t *testing.T) {
	for _, raw := range []string{`{"sparql_query":"SELECT"}`, `{"sparql_query":"SELECT","trigger":"cron"}`, `not json`, ``} {
		trigger, qids := PayloadMeta([]byte(raw))
		if trigger != TriggerManual || qids == nil || len(qids) != 0 {
			t.Errorf("%q: meta = %q %v, want manual []", raw, trigger, qids)
		}
	}
}
