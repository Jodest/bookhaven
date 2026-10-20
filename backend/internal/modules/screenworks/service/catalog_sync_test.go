package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/bookhaven/backend/internal/pkg/bookdata"
	"github.com/google/uuid"
	"github.com/hibiken/asynq"
)

type fakeWriter struct {
	writes []CatalogSyncResult
	err    error
}

func (f *fakeWriter) Write(data []byte) (int, error) {
	var r CatalogSyncResult
	if err := json.Unmarshal(data, &r); err != nil {
		return 0, err
	}
	f.writes = append(f.writes, r)
	if f.err != nil {
		return 0, f.err
	}
	return len(data), nil
}

func runJob(t *testing.T, src *fakeSource, w *fakeWriter, setupDB func(db func(status, qid string) string)) (*CatalogSyncJob, error) {
	t.Helper()
	svc, db := setup(t, src)
	setupDB(func(status, qid string) string { return mkBook(t, db, status, qid).String() })
	job := NewCatalogSyncJob(svc, nil)
	job.writer = w
	return job, job.HandleTask(context.Background(), asynq.NewTask(TaskTypeCatalogSync, nil))
}

func TestCatalogSync_OnlyPublishedWithQID(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{
		"Q1": {work("Q10", "Фильм", year(1994))},
	}}
	w := &fakeWriter{}
	_, err := runJob(t, src, w, func(mk func(string, string) string) {
		mk("published", "Q1")
		mk("published", "Q2")
		mk("published", "Q3")
		mk("published", "")
		mk("pending", "Q4")
		mk("rejected", "Q5")
	})
	if err != nil {
		t.Fatal(err)
	}
	last := w.writes[len(w.writes)-1]
	if last.Total != 3 || last.Processed != 3 || last.Updated != 1 || last.Added != 1 || last.Unchanged != 2 || last.Failed != 0 {
		t.Fatalf("result = %+v", last)
	}
	if len(src.calls) != 1 || len(src.calls[0]) != 3 {
		t.Fatalf("wikidata calls = %v", src.calls)
	}
	if w.writes[0].Total != 3 || w.writes[0].Processed != 0 {
		t.Fatalf("initial write = %+v", w.writes[0])
	}
}

func TestCatalogSync_ProgressPerChunkAndHiddenNotCounted(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{}}
	w := &fakeWriter{}
	svc, db := setup(t, src)
	for i := 0; i < 120; i++ {
		mkBook(t, db, "published", "Q1")
	}
	hidden := mkBook(t, db, "published", "Q9")
	src.works["Q9"] = []bookdata.ExternalScreenWork{work("Q90", "Скрытый", year(2000))}
	if _, err := svc.SyncBooks(context.Background(), []uuid.UUID{hidden}); err != nil {
		t.Fatal(err)
	}
	if err := db.Exec("UPDATE book_screen_works SET hidden = 1").Error; err != nil {
		t.Fatal(err)
	}
	job := NewCatalogSyncJob(svc, nil)
	job.writer = w
	if err := job.HandleTask(context.Background(), asynq.NewTask(TaskTypeCatalogSync, nil)); err != nil {
		t.Fatal(err)
	}
	// initial + 3 chunks (121 books) + final.
	if len(w.writes) != 5 {
		t.Fatalf("writes = %d", len(w.writes))
	}
	if w.writes[1].Processed != 50 || w.writes[2].Processed != 100 {
		t.Fatalf("progress = %+v", w.writes)
	}
	last := w.writes[4]
	if last.Total != 121 || last.Processed != 121 || last.Added != 0 {
		t.Fatalf("result = %+v", last)
	}
	var shown int
	db.Raw("SELECT COUNT(*) FROM book_screen_works WHERE hidden = 0").Scan(&shown)
	if shown != 0 {
		t.Fatal("hidden link was shown again")
	}
}

func TestCatalogSync_OneFailedChunkDoesNotAbort(t *testing.T) {
	src := &flakySource{failOn: map[int]bool{1: true}}
	w := &fakeWriter{}
	svc, db := setup(t, nil)
	svc.source = src
	for i := 0; i < 120; i++ {
		mkBook(t, db, "published", "Q1")
	}
	job := NewCatalogSyncJob(svc, nil)
	job.writer = w
	if err := job.HandleTask(context.Background(), asynq.NewTask(TaskTypeCatalogSync, nil)); err != nil {
		t.Fatal(err)
	}
	last := w.writes[len(w.writes)-1]
	if last.Failed != 50 || last.Processed != 120 || last.Unchanged != 70 {
		t.Fatalf("result = %+v", last)
	}
	if len(last.Errors) != 50 || last.Errors[0].Reason != ReasonWikidataUnavailable {
		t.Fatalf("errors = %d %+v", len(last.Errors), last.Errors[0])
	}
}

func TestCatalogSync_ErrorCapKeepsFailedCount(t *testing.T) {
	src := &flakySource{failOn: map[int]bool{1: true, 2: true}}
	w := &fakeWriter{}
	svc, db := setup(t, nil)
	svc.source = src
	for i := 0; i < 150; i++ {
		mkBook(t, db, "published", "Q1")
	}
	job := NewCatalogSyncJob(svc, nil)
	job.writer = w
	if err := job.HandleTask(context.Background(), asynq.NewTask(TaskTypeCatalogSync, nil)); err != nil {
		t.Fatal(err)
	}
	last := w.writes[len(w.writes)-1]
	if last.Failed != 100 || len(last.Errors) != catalogSyncErrorCap {
		t.Fatalf("failed=%d errors=%d", last.Failed, len(last.Errors))
	}
}

func TestCatalogSync_ThreeFailedChunksInARowFail(t *testing.T) {
	src := &fakeSource{err: errors.New("boom")}
	w := &fakeWriter{}
	_, err := runJob(t, src, w, func(mk func(string, string) string) {
		for i := 0; i < 200; i++ {
			mk("published", "Q1")
		}
	})
	if err == nil {
		t.Fatal("expected an error after 3 failed chunks")
	}
	if len(src.calls) != 3 {
		t.Fatalf("calls = %d, want 3", len(src.calls))
	}
}

func TestCatalogSync_WriteFailureDoesNotFailJob(t *testing.T) {
	src := &fakeSource{works: map[string][]bookdata.ExternalScreenWork{}}
	w := &fakeWriter{err: errors.New("redis down")}
	_, err := runJob(t, src, w, func(mk func(string, string) string) { mk("published", "Q1") })
	if err != nil {
		t.Fatal(err)
	}
}

// flakySource fails the 1-based call numbers in failOn and answers empty
// otherwise.
type flakySource struct {
	failOn map[int]bool
	n      int
}

func (f *flakySource) FetchAdaptations(_ context.Context, _ []string) (map[string][]bookdata.ExternalScreenWork, error) {
	f.n++
	if f.failOn[f.n] {
		return nil, errors.New("boom")
	}
	return map[string][]bookdata.ExternalScreenWork{}, nil
}

type fakeInspector struct {
	active, retry, pending []*asynq.TaskInfo
	err                    error
}

func (f *fakeInspector) ListPendingTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return f.pending, f.err
}
func (f *fakeInspector) ListActiveTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return f.active, f.err
}
func (f *fakeInspector) ListRetryTasks(string, ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	return f.retry, f.err
}

type fakeEnqueuer struct {
	tasks []*asynq.Task
	err   error
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, _ ...asynq.Option) (*asynq.TaskInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	f.tasks = append(f.tasks, task)
	return &asynq.TaskInfo{ID: "t1"}, nil
}

func TestCatalogSyncInFlight(t *testing.T) {
	sync := &asynq.TaskInfo{Type: TaskTypeCatalogSync}
	harvest := &asynq.TaskInfo{Type: "harvest:wikidata_query"}
	cases := []struct {
		name string
		in   *fakeInspector
		want bool
	}{
		{"idle", &fakeInspector{}, false},
		{"harvest ignored", &fakeInspector{active: []*asynq.TaskInfo{harvest}}, false},
		{"active", &fakeInspector{active: []*asynq.TaskInfo{sync}}, true},
		{"retry", &fakeInspector{retry: []*asynq.TaskInfo{harvest, sync}}, true},
		{"pending", &fakeInspector{pending: []*asynq.TaskInfo{sync}}, true},
		{"queue missing", &fakeInspector{err: asynq.ErrQueueNotFound}, false},
	}
	for _, c := range cases {
		got, err := CatalogSyncInFlight(c.in, "harvest")
		if err != nil || got != c.want {
			t.Errorf("%s: got %v, %v", c.name, got, err)
		}
	}
	if _, err := CatalogSyncInFlight(&fakeInspector{err: errors.New("redis")}, "harvest"); err == nil {
		t.Error("inspector error must be returned")
	}
}

func TestCatalogSyncTick(t *testing.T) {
	enq := &fakeEnqueuer{}
	tick := NewCatalogSyncTick(enq, &fakeInspector{active: []*asynq.TaskInfo{{Type: TaskTypeCatalogSync}}}, "harvest", nil)
	if err := tick.HandleTask(context.Background(), NewCatalogSyncTickTask()); err != nil || len(enq.tasks) != 0 {
		t.Fatalf("in flight: err=%v queued=%d", err, len(enq.tasks))
	}

	enq = &fakeEnqueuer{}
	tick = NewCatalogSyncTick(enq, &fakeInspector{active: []*asynq.TaskInfo{{Type: "harvest:wikidata_query"}}}, "harvest", nil)
	if err := tick.HandleTask(context.Background(), NewCatalogSyncTickTask()); err != nil || len(enq.tasks) != 1 {
		t.Fatalf("idle: err=%v queued=%d", err, len(enq.tasks))
	}
	var p CatalogSyncPayload
	_ = json.Unmarshal(enq.tasks[0].Payload(), &p)
	if enq.tasks[0].Type() != TaskTypeCatalogSync || p.Trigger != TriggerScheduled {
		t.Fatalf("task = %s %+v", enq.tasks[0].Type(), p)
	}

	tick = NewCatalogSyncTick(&fakeEnqueuer{err: errors.New("down")}, &fakeInspector{}, "harvest", nil)
	if err := tick.HandleTask(context.Background(), NewCatalogSyncTickTask()); err == nil {
		t.Fatal("enqueue failure must be returned")
	}
}
