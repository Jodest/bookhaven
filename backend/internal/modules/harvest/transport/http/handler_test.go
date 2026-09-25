package http

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	harvestservice "github.com/bookhaven/backend/internal/modules/harvest/service"
	"github.com/hibiken/asynq"
	"github.com/labstack/echo/v4"
)

// fakeEnqueuer is a hand-rolled TaskEnqueuer fake (no mocking library, per
// repo convention).
type fakeEnqueuer struct {
	err        error
	lastTask   *asynq.Task
	returnInfo *asynq.TaskInfo
}

func (f *fakeEnqueuer) Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error) {
	f.lastTask = task
	if f.err != nil {
		return nil, f.err
	}
	if f.returnInfo != nil {
		return f.returnInfo, nil
	}
	return &asynq.TaskInfo{
		ID:    "task-1",
		Queue: harvestservice.QueueName,
		State: asynq.TaskStatePending,
	}, nil
}

// fakeInspector is a hand-rolled TaskInspector fake.
type fakeInspector struct {
	pending   []*asynq.TaskInfo
	active    []*asynq.TaskInfo
	retry     []*asynq.TaskInfo
	completed []*asynq.TaskInfo
	archived  []*asynq.TaskInfo

	getByID map[string]*asynq.TaskInfo
	getErr  error
	listErr error
}

func (f *fakeInspector) GetTaskInfo(qname, id string) (*asynq.TaskInfo, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	if info, ok := f.getByID[id]; ok {
		return info, nil
	}
	return nil, asynq.ErrTaskNotFound
}

func (f *fakeInspector) ListPendingTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.pending, nil
}

func (f *fakeInspector) ListActiveTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.active, nil
}

func (f *fakeInspector) ListRetryTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.retry, nil
}

func (f *fakeInspector) ListArchivedTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.archived, nil
}

func (f *fakeInspector) ListCompletedTasks(qname string, opts ...asynq.ListOption) ([]*asynq.TaskInfo, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	return f.completed, nil
}

func newTestContext(method, path string, body []byte) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, bytes.NewReader(body))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	rec := httptest.NewRecorder()
	return e.NewContext(req, rec), rec
}

func TestTriggerHarvest_ValidAuthorQIDs_ReturnsCreated(t *testing.T) {
	enqueuer := &fakeEnqueuer{}
	h := NewHandler(enqueuer, &fakeInspector{})

	body, _ := json.Marshal(triggerHarvestRequest{AuthorQIDs: []string{"Q7200", "Q1183296"}, Limit: 100})
	c, rec := newTestContext(http.MethodPost, "/admin/harvest", body)

	if err := h.TriggerHarvest(c); err != nil {
		t.Fatalf("TriggerHarvest returned error: %v", err)
	}
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var resp triggerHarvestResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.TaskID != "task-1" || resp.Queue != harvestservice.QueueName || resp.State != "pending" {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if enqueuer.lastTask == nil {
		t.Fatal("expected enqueuer to receive a task")
	}
	var payload harvestservice.HarvestPayload
	if err := json.Unmarshal(enqueuer.lastTask.Payload(), &payload); err != nil {
		t.Fatalf("unmarshal task payload: %v", err)
	}
	if payload.SPARQLQuery == "" {
		t.Fatal("expected non-empty sparql query built from author QIDs")
	}
}

func TestTriggerHarvest_EmptyAuthorQIDs_ReturnsBadRequest(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{})

	body, _ := json.Marshal(triggerHarvestRequest{AuthorQIDs: []string{}})
	c, rec := newTestContext(http.MethodPost, "/admin/harvest", body)

	if err := h.TriggerHarvest(c); err != nil {
		t.Fatalf("TriggerHarvest returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("at least one author QID is required")) {
		t.Fatalf("expected ErrInvalidQuery message in body, got %s", rec.Body.String())
	}
}

func TestTriggerHarvest_MalformedQID_ReturnsBadRequest(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{})

	body, _ := json.Marshal(triggerHarvestRequest{AuthorQIDs: []string{"not-a-qid"}})
	c, rec := newTestContext(http.MethodPost, "/admin/harvest", body)

	if err := h.TriggerHarvest(c); err != nil {
		t.Fatalf("TriggerHarvest returned error: %v", err)
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("invalid author QID")) {
		t.Fatalf("expected invalid QID message in body, got %s", rec.Body.String())
	}
}

func TestTriggerHarvest_EnqueueError_ReturnsBadGateway(t *testing.T) {
	enqueuer := &fakeEnqueuer{err: errors.New("redis unavailable")}
	h := NewHandler(enqueuer, &fakeInspector{})

	body, _ := json.Marshal(triggerHarvestRequest{AuthorQIDs: []string{"Q7200"}})
	c, rec := newTestContext(http.MethodPost, "/admin/harvest", body)

	if err := h.TriggerHarvest(c); err != nil {
		t.Fatalf("TriggerHarvest returned error: %v", err)
	}
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestListHarvestJobs_MergesAllStatesInDocumentedOrder(t *testing.T) {
	inspector := &fakeInspector{
		active:    []*asynq.TaskInfo{{ID: "a1", State: asynq.TaskStateActive}},
		retry:     []*asynq.TaskInfo{{ID: "r1", State: asynq.TaskStateRetry}},
		pending:   []*asynq.TaskInfo{{ID: "p1", State: asynq.TaskStatePending}},
		completed: []*asynq.TaskInfo{{ID: "c1", State: asynq.TaskStateCompleted}},
		archived:  []*asynq.TaskInfo{{ID: "ar1", State: asynq.TaskStateArchived}},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector)

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs", nil)
	if err := h.ListHarvestJobs(c); err != nil {
		t.Fatalf("ListHarvestJobs returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp listHarvestJobsResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	wantOrder := []string{"a1", "r1", "p1", "c1", "ar1"}
	if len(resp.Items) != len(wantOrder) {
		t.Fatalf("items = %d, want %d", len(resp.Items), len(wantOrder))
	}
	for i, id := range wantOrder {
		if resp.Items[i].ID != id {
			t.Fatalf("items[%d].ID = %q, want %q", i, resp.Items[i].ID, id)
		}
	}
}

func TestGetHarvestJob_Found_ReturnsMappedFields(t *testing.T) {
	result := harvestservice.HarvestResult{Processed: 5, Created: 3, Skipped: 2}
	resultBytes, _ := json.Marshal(result)
	inspector := &fakeInspector{
		getByID: map[string]*asynq.TaskInfo{
			"job-1": {
				ID:       "job-1",
				Queue:    harvestservice.QueueName,
				State:    asynq.TaskStateCompleted,
				Type:     harvestservice.TaskTypeHarvestWikidataQuery,
				MaxRetry: 3,
				Retried:  1,
				Result:   resultBytes,
			},
		},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector)

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs/job-1", nil)
	c.SetParamNames("id")
	c.SetParamValues("job-1")

	if err := h.GetHarvestJob(c); err != nil {
		t.Fatalf("GetHarvestJob returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d, body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var resp harvestJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.State != "completed" || resp.MaxRetry != 3 || resp.Retried != 1 {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if resp.Result == nil || resp.Result.Created != 3 {
		t.Fatalf("expected result to be populated, got %+v", resp.Result)
	}
}

func TestGetHarvestJob_CompletedWithMalformedResult_OmitsResultNotError(t *testing.T) {
	inspector := &fakeInspector{
		getByID: map[string]*asynq.TaskInfo{
			"job-2": {
				ID:     "job-2",
				State:  asynq.TaskStateCompleted,
				Result: []byte("not json"),
			},
		},
	}
	h := NewHandler(&fakeEnqueuer{}, inspector)

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs/job-2", nil)
	c.SetParamNames("id")
	c.SetParamValues("job-2")

	if err := h.GetHarvestJob(c); err != nil {
		t.Fatalf("GetHarvestJob returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp harvestJobResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Result != nil {
		t.Fatalf("expected result to be nil for malformed bytes, got %+v", resp.Result)
	}
}

func TestGetHarvestJob_NotFound_ReturnsNotFound(t *testing.T) {
	h := NewHandler(&fakeEnqueuer{}, &fakeInspector{})

	c, rec := newTestContext(http.MethodGet, "/admin/harvest/jobs/missing", nil)
	c.SetParamNames("id")
	c.SetParamValues("missing")

	if err := h.GetHarvestJob(c); err != nil {
		t.Fatalf("GetHarvestJob returned error: %v", err)
	}
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
